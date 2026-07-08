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

	"github.com/hopeio/gox/errors"
	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/gox/net/http/openapi"
	stringsx "github.com/hopeio/gox/strings"
	mix_http "github.com/hopeio/mix/http"
	gatewayx "github.com/hopeio/mix/http/gateway"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (s *Server) InternalHandler() {
	if s.Openapi.Enabled {
		openapi.Openapi(http.DefaultServeMux, s.Openapi.UriPrefix, s.Openapi.Dir)
	}
	if s.DebugHandler.Enabled {
		httpx.HandleDebug(s.DebugHandler.UriPrefix)
	}
}

func (s *Server) httpHandler() http.Handler {
	var handler http.Handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.StackLogger().Errorw(fmt.Sprintf("panic: %v", err))
				code := strconv.Itoa(int(errors.Internal))
				w.Header().Set(httpx.HeaderErrorCode, code)
				se := &mix_http.ErrResp{Code: errors.Internal, Msg: sysErrMsg}
				buf, contentType, _ := gatewayx.DefaultMarshal(r.Context(), se)
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
		metadata := GetMetadata(r.Context())
		metadata.TraceId = trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
		metadata.Logger = log.DefaultLogger().With(zap.String(log.FieldTraceId, metadata.TraceId))
		metadata.Bagage = baggage.FromContext(r.Context())
		recorder := httpx.NewRecorder(w, r)
		r.Body = &recorder.RequestRecorder
		s.HttpHandler.ServeHTTP(&recorder.ResponseRecorder, r)

		if s.AccessLog.RecordFunc != nil {
			recorder.RequestRecorder.ContentType = r.Header.Get(httpx.HeaderContentType)
			recorder.ResponseRecorder.ContentType = recorder.Header().Get(httpx.HeaderContentType)
			s.AccessLog.RecordFunc(r.Context(), &AccessLogParam{
				r.Method, r.RequestURI,
				recorder,
				metadata,
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
