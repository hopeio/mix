/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"fmt"
	"strings"

	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	stringsx "github.com/hopeio/gox/strings"
	"go.uber.org/zap"
)

const (
	ContentTypeJson     = "json"
	ContentTypeProtobuf = "protobuf"
)

type Body struct {
	ContentType string
	Raw         []byte
	Data        any
}

type AccessLogParam struct {
	Method, Url string
	*httpx.Recorder
	Metadata *Metadata
}

type AccessLog = func(ctx context.Context, param *AccessLogParam)

// safeStringer 安全地把任意值转成字符串，避免访问日志里的类型断言把服务打崩
func safeStringer(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}

func DefaultAccessLog(ctx context.Context, param *AccessLogParam) {
	reqBodyField := zap.Skip()
	if len(param.RequestRecorder.Raw) > 0 || param.RequestRecorder.Value != nil || param.RequestRecorder.Body != nil {
		if param.RequestRecorder.Raw == nil && param.RequestRecorder.Body != nil {
			param.RequestRecorder.Raw = param.RequestRecorder.Body.Bytes()
		}
		if strings.HasSuffix(param.RequestRecorder.ContentType, ContentTypeJson) {
			reqBodyField = log.RawJson("body", param.RequestRecorder.Raw)
		} else if strings.HasSuffix(param.RequestRecorder.ContentType, ContentTypeProtobuf) {
			reqBodyField = zap.String("body", safeStringer(param.RequestRecorder.Value))
		} else {
			reqBodyField = zap.String("body", stringsx.FromBytes(param.RequestRecorder.Raw))
		}
	}
	respBodyField := zap.Skip()
	if len(param.ResponseRecorder.Raw) > 0 || param.ResponseRecorder.Value != nil || param.ResponseRecorder.Body != nil {
		if param.ResponseRecorder.Raw == nil && param.ResponseRecorder.Body != nil {
			param.ResponseRecorder.Raw = param.ResponseRecorder.Body.Bytes()
		}
		if strings.HasSuffix(param.ResponseRecorder.ContentType, ContentTypeJson) {
			respBodyField = log.RawJson("response", param.ResponseRecorder.Raw)
		} else if strings.HasSuffix(param.ResponseRecorder.ContentType, ContentTypeProtobuf) {
			respBodyField = zap.String("response", safeStringer(param.ResponseRecorder.Value))
		} else {
			respBodyField = zap.String("response", stringsx.FromBytes(param.ResponseRecorder.Raw))
		}
	}

	if ce := log.NoCallerLogger().Logger.Check(zap.InfoLevel, "access"); ce != nil {
		ce.Write(zap.Inline(zap.DictObject(param.Metadata.AccessLogFields...)),
			zap.String("url", param.Url),
			zap.String("method", param.Method),
			reqBodyField,
			log.Context(ctx),
			zap.Duration("duration", ce.Time.Sub(param.Metadata.RequestAt)),
			respBodyField,
			zap.Int("status", param.StatusCode))
	}
}

type GrpcAccessLogParam struct {
	Method            string
	Request, Response any
	Err               error
	Metadata          *Metadata
}

type GrpcAccessLog = func(ctx context.Context, param *GrpcAccessLogParam)

func DefaultGrpcAccessLog(ctx context.Context, param *GrpcAccessLogParam) {
	respBodyField := zap.Skip()
	errField := zap.Skip()
	if param.Err != nil {
		errField = zap.Error(param.Err)
	} else {
		respBodyField = zap.String("response", safeStringer(param.Response))
	}

	if ce := log.NoCallerLogger().Logger.Check(zap.InfoLevel, "access"); ce != nil {
		ce.Write(zap.Inline(zap.DictObject(param.Metadata.AccessLogFields...)),
			zap.String("url", param.Method),
			zap.String("method", "grpc"),
			zap.String("body", safeStringer(param.Request)),
			log.Context(ctx),
			zap.Duration("duration", ce.Time.Sub(param.Metadata.RequestAt)),
			errField,
			respBodyField)
	}
}
