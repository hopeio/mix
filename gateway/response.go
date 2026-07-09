package gateway

import (
	"fmt"
	"net/http"
	"net/textproto"
	"strconv"

	"github.com/hopeio/mix"
	httpx "github.com/hopeio/gox/net/http"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(w http.ResponseWriter, r *http.Request, message proto.Message) error {
	var contentType string
	var buf []byte
	var err error
	switch rb := message.(type) {
	case http.Handler:
		rb.ServeHTTP(w, r)
		return nil
	case mix.Responder:
		rb.Respond(r.Context(), w)
		return nil
	case mix.ResponseBody:
		buf, contentType = rb.ResponseBody()
	case mix.XXXResponseBody:
		buf, contentType, err = mix.DefaultMarshal(r.Context(), rb.XXX_ResponseBody())
		if err != nil {
			return err
		}
	default:
		buf, contentType, err = mix.DefaultMarshal(r.Context(), message)
		if err != nil {
			return err
		}
	}
	w.Header().Set(httpx.HeaderContentType, contentType)
	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(buf, message)
	}
	_, err = w.Write(buf)
	return err
}

func HandleForwardResponseTrailerHeader(w http.ResponseWriter, md metadata.MD) {
	for k := range md {
		tKey := textproto.CanonicalMIMEHeaderKey(fmt.Sprintf("%s%s", MetadataTrailerPrefix, k))
		w.Header().Add(httpx.HeaderTrailer, tKey)
	}
}

func HandleForwardResponseTrailer(w http.ResponseWriter, md metadata.MD) {
	for k, vs := range md {
		tKey := fmt.Sprintf("%s%s", MetadataTrailerPrefix, k)
		for _, v := range vs {
			w.Header().Add(tKey, v)
		}
	}
}

// FinalizeStreamTrailers 在流式响应结束时写出 grpc-status / grpc-message 及自定义 trailer metadata。
func FinalizeStreamTrailers(w http.ResponseWriter, started bool, err error, trailers metadata.MD) {
	if !started {
		return
	}
	if err != nil {
		w.Header().Set(httpx.HeaderGrpcStatus, strconv.Itoa(int(status.Code(err))))
		w.Header().Set(httpx.HeaderGrpcMessage, err.Error())
	} else {
		w.Header().Set(httpx.HeaderGrpcStatus, "0")
	}
	HandleForwardResponseTrailer(w, trailers)
}

var HandleError = func(w http.ResponseWriter, r *http.Request, err error) {
	s := ErrRespFromError(err)
	delete(r.Header, httpx.HeaderTrailer)
	errcodeHeader := strconv.Itoa(int(s.Code))
	buf, contentType, _ := mix.DefaultMarshal(r.Context(), s)
	header := w.Header()
	header.Set(httpx.HeaderContentType, contentType)
	header.Set(httpx.HeaderGrpcStatus, errcodeHeader)
	header.Set(httpx.HeaderErrorCode, errcodeHeader)
	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(buf, s)
	}
	if _, err := w.Write(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}

func ErrRespFromError(err error) *mix.ErrResp {
	if err == nil {
		return nil
	}
	s, ok := status.FromError(err)
	if ok {
		return &mix.ErrResp{
			Code: mix.ErrCode(s.Code()),
			Msg:  s.Message(),
		}
	}
	if errresp, ok := err.(*mix.ErrResp); ok {
		return errresp
	}
	return mix.ErrRespFrom(err)
}