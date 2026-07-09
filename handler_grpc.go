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

	"github.com/hopeio/gox/log"
	"github.com/hopeio/gox/validator"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats/opentelemetry"
	"google.golang.org/grpc/status"
)

type GRPCStatus interface {
	GRPCStatus() *status.Status
}

func (s *Server) grpcHandler() *grpc.Server {
	//conf := s.Config
	grpclog.SetLoggerV2(zapgrpc.NewLogger(log.NoCallerLogger().With(zap.String("server", "grpc")).Logger))
	if s.GrpcHandler != nil {
		var stream []grpc.StreamServerInterceptor
		var unary []grpc.UnaryServerInterceptor

		if s.Otel.Enabled {
			s.Grpc.Options = append(s.Grpc.Options, grpc.StatsHandler(
				otelgrpc.NewServerHandler(append([]otelgrpc.Option{
					otelgrpc.WithPropagators(propagation.NewCompositeTextMapPropagator(
						opentelemetry.GRPCTraceBinPropagator{}, propagation.Baggage{},
					))}, s.Otel.OtelgrpcOpts...)...)))
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
	md := GetMetadata(ctx)
	md.TraceId = trace.SpanFromContext(ctx).SpanContext().TraceID().String()
	md.Logger = log.DefaultLogger().With(zap.String(log.FieldTraceId, md.TraceId))
	md.ServerTransportStream = grpc.ServerTransportStreamFromContext(ctx)
	var ok bool
	md.IncomingMD, ok = metadata.FromIncomingContext(ctx)
	if !ok {
		md.IncomingMD = nil
	}
	if err = validator.ValidateStruct(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	resp, err = handler(ctx, req)

	if err != nil {
		if _, ok := err.(GRPCStatus); !ok {
			err = status.Error(codes.Unknown, err.Error())
		}
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
	if err == nil && resp == nil {
		resp = reflect.New(reflect.TypeOf(resp).Elem()).Interface()
	}
	return resp, err

}

func (s *Server) StreamAccess(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.StackLogger().Errorw(fmt.Sprintf("panic: %v", r))
			err = status.Error(codes.Internal, sysErrMsg)
		}
	}()
	ctx := stream.Context()
	md := GetMetadata(ctx)
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
	}
	err = handler(srv, wrapper)
	if err != nil {
		if _, ok := err.(GRPCStatus); !ok {
			err = status.Error(codes.Unknown, err.Error())
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
}

func (s *recvWrapper) SendMsg(m interface{}) error {
	s.Response = m
	return s.ServerStream.SendMsg(m)
}

func (s *recvWrapper) RecvMsg(m interface{}) error {
	s.Request = m
	if err := validator.ValidateStruct(m); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	return nil
}
