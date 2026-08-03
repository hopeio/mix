/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/gox/net/http/openapi"
	stringsx "github.com/hopeio/gox/strings"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// InternalHandler 往内部端口的私有 mux 上注册 OpenAPI 文档与调试端点
func (s *Server) InternalHandler(mux *http.ServeMux) {
	if s.Openapi.Enabled {
		openapi.Openapi(mux, s.Openapi.UriPrefix, s.Openapi.Dir)
	}
	if s.DebugHandler.Enabled {
		httpx.HandleDebug(mux, s.DebugHandler.UriPrefix)
	}
}

func (s *Server) httpHandler() http.Handler {
	var handler http.Handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.StackLogger().Errorw(fmt.Sprintf("panic: %v", err))
				code := strconv.Itoa(int(Internal))
				w.Header().Set(httpx.HeaderErrorCode, code)
				se := &ErrResp{Code: Internal, Msg: sysErrMsg}
				buf, contentType, _ := DefaultMarshal(r.Context(), se)
				w.Header().Set(httpx.HeaderContentType, contentType)
				w.Write(buf)
			}
		}()
		// 不记录日志
		if len(s.AccessLog.ExcludePrefixes) > 0 {
			if stringsx.HasPrefixes(r.RequestURI, s.AccessLog.ExcludePrefixes) &&
				!stringsx.HasPrefixes(r.RequestURI, s.AccessLog.IncludePrefixes) {
				s.HttpHandler.ServeHTTP(w, r)
				return
			}
		}
		md := GetMetadata(r.Context())
		if md == nil {
			md = &Metadata{Request: r, ResponseWriter: w, RequestAt: time.Now()}
		}
		md.TraceId = trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		md.Logger = log.DefaultLogger().With(zap.String(log.FieldTraceId, md.TraceId))
		md.Baggage = baggage.FromContext(r.Context())
		// gRPC metadata 约定小写 key，HTTP header 是 CanonicalMIME 大小写，需转换
		md.IncomingMD = make(metadata.MD, len(r.Header))
		for k, v := range r.Header {
			md.IncomingMD[strings.ToLower(k)] = v
		}
		recorder := httpx.NewRecorder(w, r)
		r.Body = &recorder.RequestRecorder
		s.HttpHandler.ServeHTTP(&recorder.ResponseRecorder, r)

		if s.AccessLog.RecordFunc != nil {
			recorder.RequestRecorder.ContentType = r.Header.Get(httpx.HeaderContentType)
			recorder.ResponseRecorder.ContentType = recorder.Header().Get(httpx.HeaderContentType)
			s.AccessLog.RecordFunc(r.Context(), &AccessLogParam{
				r.Method, r.RequestURI,
				recorder,
				md,
			})
		}
		recorder.Reset()
	})
	if s.Otel.Enabled {
		return otelhttp.NewHandler(handler, "http", append([]otelhttp.Option{otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return r.RequestURI
		})}, s.Otel.OtelhttpOpts...)...)
	}
	return handler
}
