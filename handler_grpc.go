/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/stats/opentelemetry"
	"google.golang.org/grpc/status"
)

type GRPCStatus interface {
	GRPCStatus() *status.Status
}

var grpcLoggerOnce sync.Once

func (s *Server) grpcHandler() *grpc.Server {
	grpcLoggerOnce.Do(func() {
		grpclog.SetLoggerV2(zapgrpc.NewLogger(log.NoCallerLogger().With(zap.String("server", "grpc")).Logger))
	})
	if s.GrpcHandler != nil {
		var stream []grpc.StreamServerInterceptor
		var unary []grpc.UnaryServerInterceptor

		if s.Otel.Enabled {
			s.Grpc.Options = append(s.Grpc.Options, grpc.StatsHandler(
				otelgrpc.NewServerHandler(append([]otelgrpc.Option{
					otelgrpc.WithPropagators(propagation.NewCompositeTextMapPropagator(
						propagation.TraceContext{},
						opentelemetry.GRPCTraceBinPropagator{},
						propagation.Baggage{},
					)),
					// Treat calls WITHOUT the internal-only "Grpc-Internal" metadata
					// header as public (untrusted) endpoints: start a fresh root span
					// instead of inheriting the client trace, and link the client's
					// span context for correlation. Internal service-to-service calls
					// carry Grpc-Internal (see gox httpx.HeaderGrpcInternal) and are
					// trusted — their traceparent is inherited normally.
					otelgrpc.WithPublicEndpointFn(func(ctx context.Context, _ *stats.RPCTagInfo) bool {
						md, ok := metadata.FromIncomingContext(ctx)
						if !ok {
							return true
						}
						_, trusted := md[httpx.HeaderGrpcInternal]
						return !trusted
					}),
				}, s.Otel.OtelgrpcOpts...)...)))
		}
		stream = append(stream, s.StreamAccess)
		stream = append(stream, s.Grpc.StreamServerInterceptors...)
		unary = append(unary, s.UnaryAccess)
		unary = append(unary, s.Grpc.UnaryServerInterceptors...)

		s.Grpc.Options = append(s.Grpc.Options, grpc.ChainStreamInterceptor(stream...),
			grpc.ChainUnaryInterceptor(unary...))

		grpcServer := grpc.NewServer(s.Grpc.Options...)
		s.GrpcHandler(grpcServer)
		reflection.Register(grpcServer)
		// 标准健康检查服务，供 grpc-health-probe / k8s gRPC 探针使用
		healthgrpc.RegisterHealthServer(grpcServer, health.NewServer())
		return grpcServer
	}
	return nil
}

func (s *Server) UnaryAccess(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	//enabledPrometheus := conf.EnabledMetrics

	defer func() {
		if r := recover(); r != nil {
			log.StackLogger().Errorw(fmt.Sprintf("panic: %v", r))
			err = status.Error(codes.Internal, sysErrMsg)
		}
	}()
	// 独立 gRPC 端口不经过 HTTP 分发层，ctx 中可能没有 Metadata，此处兜底注入
	md := GetMetadata(ctx)
	if md == nil {
		md = &Metadata{RequestAt: time.Now()}
		ctx = WithMetadata(ctx, md)
	}
	md.TraceId = trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	md.Logger = log.DefaultLogger().With(zap.String(log.FieldTraceId, md.TraceId))
	md.ServerTransportStream = grpc.ServerTransportStreamFromContext(ctx)
	var ok bool
	md.IncomingMD, ok = metadata.FromIncomingContext(ctx)
	if !ok {
		md.IncomingMD = nil
	}
	if err = ValidateStruct(req); err != nil {
		return nil, err
	}
	resp, err = handler(ctx, req)

	if err != nil {
		if _, ok := err.(GRPCStatus); !ok {
			log.Errorw("untyped grpc handler error",
				zap.String("method", info.FullMethod),
				zap.Error(err))
			err = Internal.ErrResp()
		}
	}

	// 兼容 handler 返回双 nil：grpc 的 proto codec 无法 marshal nil 响应，
	// 直接返回会报 Internal，需按方法签名构造空响应
	if err == nil && resp == nil {
		resp = emptyResp(info)
	}

	if s.Grpc.RecordFunc != nil {
		s.Grpc.RecordFunc(ctx, &GrpcAccessLogParam{
			Method:   info.FullMethod,
			Metadata: md,
			Request:  req,
			Response: resp,
			Err:      err,
		})
	}
	return resp, err
}

// emptyRespTypeCache 缓存 FullMethod -> 响应元素类型，避免每次请求都反射方法签名
var emptyRespTypeCache sync.Map

// emptyResp 从服务实现的方法签名反射出响应类型并构造空实例。
// 不能对 nil 的 resp 自身反射（reflect.TypeOf(nil) == nil，调 Elem 必 panic），
// 只能从 info.Server 的方法签名拿类型。类型按 FullMethod 缓存，
// 稳定后每次请求仅一次 sync.Map 查询 + 一次小对象分配。
func emptyResp(info *grpc.UnaryServerInfo) any {
	if v, ok := emptyRespTypeCache.Load(info.FullMethod); ok {
		if t, _ := v.(reflect.Type); t != nil {
			return reflect.New(t).Interface()
		}
		return nil
	}
	var t reflect.Type
	if i := strings.LastIndexByte(info.FullMethod, '/'); i >= 0 && i < len(info.FullMethod)-1 {
		if m, ok := reflect.TypeOf(info.Server).MethodByName(info.FullMethod[i+1:]); ok {
			ft := m.Type
			// 生成的服务方法签名为 (ctx, req) (*Resp, error)，取第一个非 error 返回值
			if ft.NumOut() > 0 && ft.Out(0) != reflect.TypeFor[error]() {
				rt := ft.Out(0)
				if rt.Kind() == reflect.Pointer {
					t = rt.Elem()
				} else {
					t = rt
				}
			}
		}
	}
	emptyRespTypeCache.Store(info.FullMethod, t)
	if t == nil {
		return nil
	}
	return reflect.New(t).Interface()
}

func (s *Server) StreamAccess(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.StackLogger().Errorw(fmt.Sprintf("panic: %v", r))
			err = status.Error(codes.Internal, sysErrMsg)
		}
	}()
	ctx := stream.Context()
	// 独立 gRPC 端口不经过 HTTP 分发层，ctx 中可能没有 Metadata，此处兜底注入
	md := GetMetadata(ctx)
	if md == nil {
		md = &Metadata{RequestAt: time.Now()}
		ctx = WithMetadata(ctx, md)
	}
	md.TraceId = trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	md.Logger = log.DefaultLogger().With(zap.String(log.FieldTraceId, md.TraceId))
	md.ServerTransportStream = grpc.ServerTransportStreamFromContext(ctx)
	var ok bool
	md.IncomingMD, ok = metadata.FromIncomingContext(ctx)
	if !ok {
		md.IncomingMD = nil
	}
	wrapper := &recvWrapper{
		ServerStream: stream,
		ctx:          ctx,
	}
	err = handler(srv, wrapper)
	if err != nil {
		if _, ok := err.(GRPCStatus); !ok {
			log.Errorw("untyped grpc stream handler error",
				zap.String("method", info.FullMethod),
				zap.Error(err))
			err = Internal.ErrResp()
		}
	}

	if s.Grpc.RecordFunc != nil {
		wrapper.Err = err
		wrapper.Method = info.FullMethod
		s.Grpc.RecordFunc(wrapper.Context(), &wrapper.GrpcAccessLogParam)
	}
	return err
}

type recvWrapper struct {
	grpc.ServerStream
	GrpcAccessLogParam
	ctx context.Context
}

// Context 返回注入了 Metadata 的 ctx，保证业务 handler 能取到
func (s *recvWrapper) Context() context.Context {
	return s.ctx
}

func (s *recvWrapper) SendMsg(m interface{}) error {
	s.Response = m
	return s.ServerStream.SendMsg(m)
}

func (s *recvWrapper) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	// 必须先接收数据再校验，否则校验的是零值结构体
	s.Request = m
	if err := ValidateStruct(m); err != nil {
		return err
	}
	return nil
}
