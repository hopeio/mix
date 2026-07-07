/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package fiber

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	iox "github.com/hopeio/gox/io"
	mix_http "github.com/hopeio/mix/http"
	stringsx "github.com/hopeio/gox/strings"
	"github.com/hopeio/gox/kvstruct"
	"github.com/valyala/fasthttp"
)

func Bind(c fiber.Ctx, obj interface{}) error {
	return mix_http.CommonBind(RequestSource{c}, obj)
}

type RequestSource struct {
	fiber.Ctx
}

func (s RequestSource) Uri() kvstruct.Getter {
	return uriSource{s.Ctx}
}

func (s RequestSource) Query() kvstruct.ValuesGetter {
	return (*ArgsSource)(s.Request().URI().QueryArgs())
}

func (s RequestSource) Header() kvstruct.ValuesGetter {
	return (*HeaderSource)(&s.Request().Header)
}

func (s RequestSource) Body() (context.Context, string, io.ReadCloser) {
	if s.Method() == http.MethodGet {
		return s.Context(), "", nil
	}
	contentType := stringsx.FromBytes(s.Request().Header.ContentType())
	req := s.Ctx.Request()
	if req.IsBodyStream() {
		return s.Context(), contentType, iox.WrapReader(req.BodyStream(), req.CloseBodyStream)
	}
	return s.Context(), contentType, iox.RawBytes(req.Body())
}

type ArgsSource fasthttp.Args

func (form *ArgsSource) Get(key string) ([]string, bool) {
	var values []string
	(*fasthttp.Args)(form).VisitAll(func(k, v []byte) {
		if string(k) == key {
			values = append(values, stringsx.FromBytes(v))
		}
	})
	return values, len(values) > 0
}

type CtxSource fasthttp.RequestCtx


func (form *CtxSource) Get(key string) (string, bool) {
	v := (*fasthttp.RequestCtx)(form).UserValue(key).(string)
	return v, v != ""
}

type HeaderSource fasthttp.RequestHeader

func (form *HeaderSource) Get(key string) ([]string, bool) {
	var values []string
	for k,v:=range (*fasthttp.RequestHeader)(form).All() {
		if string(k) == key {
			v, _ := url.QueryUnescape(stringsx.FromBytes(v))
			values = append(values, v)
		}
	}
	return values, len(values) > 0
}


type uriSource struct {
	fiber.Ctx
}

func (s uriSource) Get(key string) (string, bool) {
	v := s.Params(key)
	return v, v != ""
}
