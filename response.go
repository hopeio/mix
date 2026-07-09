/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"mime"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"

	iox "github.com/hopeio/gox/io"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/gox/strings"
	stringsx "github.com/hopeio/gox/strings"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

type ResponseWriter interface {
	WriteHeader(code int)
	HeaderX() httpx.Header
	Write([]byte) (int, error)
}

type Responder interface {
	Respond(ctx context.Context, w http.ResponseWriter) (int, error)
}

var errCodeHttpStatusMap = map[ErrCode]int{
	Success: http.StatusOK,
}

func RegisterErrCodeHttpStatus(code ErrCode, status int) {
	errCodeHttpStatusMap[code] = status
}

type errHeaderKey struct{}

var ErrHeaderKey errHeaderKey

func RespodWithErrHeader(ctx context.Context) context.Context {
	return context.WithValue(ctx, ErrHeaderKey, true)
}

func StatusFromErrCode(code ErrCode) int {
	if status, ok := errCodeHttpStatusMap[code]; ok {
		return status
	}
	return http.StatusOK
}

// CommonResp 主要用来接收返回，发送请使用 CommonAnyResp
type CommonResp[T any] struct {
	Code ErrCode `json:"code"`
	Msg  string          `json:"msg,omitempty"`
	//验证码
	Data T `json:"data,omitempty"`
}

func (res *CommonResp[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res.Respond(r.Context(), w)
}

func (res *CommonResp[T]) Respond(ctx context.Context, w http.ResponseWriter) (int, error) {
	data, contentType, err := DefaultMarshal(ctx, res)
	if err != nil {
		return RespondError(ctx, w, err)
	}
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		if res.Code != Success && ctx.Value(ErrHeaderKey) != nil {
			header.Set(httpx.HeaderErrorCode, strconv.Itoa(int(res.Code)))
			header.Set(httpx.HeaderErrorMsg, res.Msg)
		}
		header.Set(httpx.HeaderContentType, contentType)
	} else {
		header := w.Header()
		if res.Code != Success && ctx.Value(ErrHeaderKey) != nil {
			header.Set(httpx.HeaderErrorCode, strconv.Itoa(int(res.Code)))
			header.Set(httpx.HeaderErrorMsg, res.Msg)
		}
		header.Set(httpx.HeaderContentType, contentType)
	}

	if res.Code != Success {
		w.WriteHeader(StatusFromErrCode(res.Code))
	} else {
		w.WriteHeader(http.StatusOK)
	}

	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(data, res)
	}
	return w.Write(data)
}

type CommonProtoResp[T proto.Message]  CommonResp[T]

func NewCommonProtoResp[T proto.Message](code ErrCode, msg string, data T) *CommonProtoResp[T] {
	return &CommonProtoResp[T]{Code: code, Msg: msg, Data: data}
}

func (r *CommonProtoResp[T]) MarshalProto() ([]byte, error) {
	buf := make([]byte, 0, 64)

	if r.Code != 0 {
		buf = protowire.AppendVarint(buf, 0x08)
		buf = protowire.AppendVarint(buf, uint64(r.Code))
	}

	// 编码 Msg 字段 (field number 2, string)
	if r.Msg != "" {
		buf = protowire.AppendVarint(buf, 0x12)
		buf = protowire.AppendString(buf, r.Msg)
	}

	// 编码 Data 字段 (field number 3, bytes)
	if r.Code == 0 {
		buf = append(buf, 0)
		var err error
		buf, err = proto.MarshalOptions{}.MarshalAppend(buf, r.Data)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

// UnmarshalProto 手动解码 protobuf 数据到 CommonProtoResp
func (r *CommonProtoResp[T]) UnmarshalProto(data []byte) error {
	var pos int
	if data[0] == 0 {
		if reflect.ValueOf(r.Data).IsNil() {
			r.Data = r.Data.ProtoReflect().New().Interface().(T)
		}
		if len(data[pos:]) > 0 {
			return proto.Unmarshal(data[1:], r.Data)
		}
		return nil
	}
	for pos < len(data) {
		// 解析标签 (tag 和 wire type)
		tag, n := protowire.ConsumeVarint(data[pos:])
		if n < 0 {
			return errors.New("invalid protobuf data: unable to consume varint")
		}
		pos += n

		fieldNum, wireType := protowire.DecodeTag(tag)
		switch fieldNum {
		case 1: // Code 字段
			if wireType != protowire.VarintType {
				return errors.New("invalid wire type for Code field")
			}
			code, n := protowire.ConsumeVarint(data[pos:])
			if n < 0 {
				return errors.New("invalid protobuf data: unable to consume Code varint")
			}
			r.Code = ErrCode(code)
			pos += n

		case 2: // Msg 字段
			if wireType != protowire.BytesType {
				return errors.New("invalid wire type for Msg field")
			}
			msg, n := protowire.ConsumeString(data[pos:])
			if n < 0 {
				return errors.New("invalid protobuf data: unable to consume Msg string")
			}
			r.Msg = msg
			pos += n
		}
	}

	return nil
}


type CommonAnyResp = CommonResp[any]

func NewCommonAnyResp(code ErrCode, msg string, data any) *CommonAnyResp {
	return &CommonAnyResp{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

func ServeErrCodeMsg(w http.ResponseWriter, r *http.Request, code ErrCode, msg string) {
	NewErrResp(code, msg).ServeHTTP(w, r)
}

func RespondErrCodeMsg(ctx context.Context, w http.ResponseWriter, code ErrCode, msg string) {
	NewErrResp(code, msg).Respond(ctx, w)
}

func ServeError(w http.ResponseWriter, r *http.Request, err error) {
	ErrRespFrom(err).ServeHTTP(w, r)
}

func RespondError(ctx context.Context, w http.ResponseWriter, err error) (int, error) {
	return ErrRespFrom(err).Respond(ctx, w)
}

func ServeSuccess(w http.ResponseWriter, r *http.Request, res any) {
	RespondSuccess(r.Context(), w, res)
}

func RespondSuccess(ctx context.Context, w http.ResponseWriter, res any) (int, error) {
	data, contentType, err := DefaultMarshal(ctx, res)
	if err != nil {
		return RespondError(ctx, w, err)
	}
	if wx, ok := w.(ResponseWriter); ok {
		wx.HeaderX().Set(httpx.HeaderContentType, contentType)
	} else {
		w.Header().Set(httpx.HeaderContentType, contentType)
	}
	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(data, res)
	}
	return w.Write(data)
}

func Serve(w http.ResponseWriter, r *http.Request, data any) {
	if err, ok := data.(error); ok {
		ServeError(w, r, err)
	}
	ServeSuccess(w, r, data)
}

func Respond(ctx context.Context, w http.ResponseWriter, data any) (int, error) {
	if err, ok := data.(error); ok {
		return RespondError(ctx, w, err)
	}
	return RespondSuccess(ctx, w, data)
}

type Response struct {
	Status  int                `json:"status,omitempty"`
	Headers http.Header        `json:"header,omitempty"`
	Body    iox.WriterToCloser `json:"body,omitempty"`
}

func (res *Response) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res.Respond(r.Context(), w)
}

func (res *Response) Respond(ctx context.Context, w http.ResponseWriter) (int, error) {
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		for k, v := range res.Headers {
			for _, vv := range v {
				header.Add(k, vv)
			}
		}
	} else {
		httpx.CopyHttpHeader(w.Header(), res.Headers)
	}
	w.WriteHeader(res.Status)
	n, err := res.Body.WriteTo(w)
	res.Body.Close()
	return int(n), err
}

type ErrResp struct {
	Code ErrCode `json:"code"`
	Msg  string  `json:"msg,omitempty"`
}

func NewErrResp(code ErrCode, msg string) *ErrResp {
	return &ErrResp{
		Code: code,
		Msg:  msg,
	}
}

func ErrRespFrom(err error) *ErrResp {
	if err == nil {
		return nil
	}
	if errresp, ok := err.(*ErrResp); ok {
		return errresp
	}
	type errresp interface{ ErrResp() *ErrResp }
	if se, ok := err.(errresp); ok {
		return se.ErrResp()
	}
	rv := reflect.ValueOf(err)
	kind := rv.Kind()
	if kind >= reflect.Int && kind <= reflect.Int64 {
		return NewErrResp(ErrCode(rv.Int()), err.Error())
	}
	if kind >= reflect.Uint && kind <= reflect.Uint64 {
		return NewErrResp(ErrCode(rv.Uint()), err.Error())
	}
	return NewErrResp(Unknown, err.Error())
}

func (res *ErrResp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res.Respond(r.Context(), w)
}

func (res *ErrResp) Respond(ctx context.Context, w http.ResponseWriter) (int, error) {
	data, contentType, _ := DefaultMarshal(ctx, res)
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		if res.Code != Success && ctx.Value(ErrHeaderKey) != nil {
			header.Set(httpx.HeaderErrorCode, strconv.Itoa(int(res.Code)))
			header.Set(httpx.HeaderErrorMsg, res.Msg)
		}
		header.Set(httpx.HeaderContentType, contentType)
	} else {
		header := w.Header()
		if res.Code != Success && ctx.Value(ErrHeaderKey) != nil {
			header.Set(httpx.HeaderErrorCode, strconv.Itoa(int(res.Code)))
			header.Set(httpx.HeaderErrorMsg, res.Msg)
		}
		header.Set(httpx.HeaderContentType, contentType)
	}
	if res.Code != Success {
		w.WriteHeader(StatusFromErrCode(res.Code))
	} else {
		w.WriteHeader(http.StatusOK)
	}
	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(data, res)
	}
	return w.Write(data)
}

func (res *ErrResp) ErrResp() *ErrResp {
	return (*ErrResp)(res)
}

func (x *ErrResp) GRPCStatus() *status.Status {
	return status.New(codes.Code(x.Code), x.Msg)
}

func (x *ErrResp) Error() string {
	return fmt.Sprintf("code: %d, msg: %s", x.Code, x.Msg)
}

func (x *ErrResp) MarshalJSON() ([]byte, error) {
	return stringsx.ToBytes(`{"code":` + strconv.Itoa(int(x.Code)) + `,"msg":` + strconv.Quote(x.Msg) + `}`), nil
}


type ResponseStream struct {
	Status  int                          `json:"status,omitempty"`
	Headers http.Header                  `json:"header,omitempty"`
	Body    iter.Seq[iox.WriterToCloser] `json:"body,omitempty"`
}

func (res *ResponseStream) Respond(ctx context.Context, w http.ResponseWriter) (int, error) {
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		for k, v := range res.Headers {
			for _, vv := range v {
				header.Add(k, vv)
			}
		}
	} else {
		httpx.CopyHttpHeader(w.Header(), res.Headers)
	}
	w.WriteHeader(res.Status)
	return RespondStream(ctx, w, res.Body)
}

func (res *ResponseStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res.Respond(r.Context(), w)
}

func RespondStream(ctx context.Context, w http.ResponseWriter, dataSource iter.Seq[iox.WriterToCloser]) (int, error) {
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		header.Set(httpx.HeaderCacheControl, httpx.CacheControlNoCache)
		header.Set(httpx.HeaderTransferEncoding, httpx.TransferEncodingChunked)
	} else {
		header := w.Header()
		header.Set(httpx.HeaderCacheControl, httpx.CacheControlNoCache)
		header.Set(httpx.HeaderTransferEncoding, httpx.TransferEncodingChunked)
	}
	var n int
	flusher := w.(http.Flusher)
	for data := range dataSource {
		select {
		case <-ctx.Done():
			return n, ctx.Err()
		default:
			writen, err := data.WriteTo(w)
			n += int(writen)
			if err != nil {
				return n, err
			}
			flusher.Flush()
		}
	}
	return n, nil
}

type SSEData string

func (data SSEData) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(strings.ToBytes(fmt.Sprintf("data: %s\n\n", data)))
	return int64(n), err
}
func (data SSEData) Close() error {
	return nil
}

func RespondSSE[T ~string](ctx context.Context, w http.ResponseWriter, dataSource iter.Seq[T]) (int, error) {
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		header.Set(httpx.HeaderContentType, httpx.ContentTypeTextEventStream)
	} else {
		header := w.Header()
		header.Set(httpx.HeaderContentType, httpx.ContentTypeTextEventStream)
	}
	return RespondStream(ctx, w, func(yield func(iox.WriterToCloser) bool) {
		for data := range dataSource {
			yield(SSEData(data))
		}
	})
}

type XXXResponseBody interface {
	XXX_ResponseBody() any
}

type ResponseBody interface {
	ResponseBody() ([]byte, string)
}

type StatusCode interface {
	StatusCode(v any) int
}

type ResponseFile struct {
	Name        string             `json:"name"`
	Body        iox.WriterToCloser `json:"body"`
	ContentType string
}

func (res *ResponseFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	res.Respond(r.Context(), w)
}

func (res *ResponseFile) Respond(ctx context.Context, w http.ResponseWriter) (int, error) {
	contentType := res.ContentType
	var limitedWriter iox.LimitedWriter
	if contentType == "" {
		if ext := filepath.Ext(res.Name); ext != "" {
			contentType = mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = httpx.ContentTypeOctetStream
			}
		} else {
			limitedWriter = iox.NewLimitedWriter(512)
			_, err := res.Body.WriteTo(&limitedWriter)
			if err != nil {
				return 0, err
			}
			contentType = http.DetectContentType(limitedWriter)
		}
	}
	contentDisposition := "inline"
	if res.Name != "" {
		contentDisposition = fmt.Sprintf(httpx.AttachmentTmpl, res.Name)
	}
	if wx, ok := w.(ResponseWriter); ok {
		header := wx.HeaderX()
		header.Set(httpx.HeaderContentType, contentType)
		header.Set(httpx.HeaderContentDisposition, contentDisposition)
	} else {
		header := w.Header()
		header.Set(httpx.HeaderContentType, contentType)
		header.Set(httpx.HeaderContentDisposition, contentDisposition)
	}
	if len(limitedWriter) > 0 {
		n, err := w.Write(limitedWriter)
		if err != nil {
			return n, err
		}
	}
	n, err := res.Body.WriteTo(w)
	res.Body.Close()
	if len(limitedWriter) > 0 {
		n += int64(len(limitedWriter))
	}
	return int(n), err
}
