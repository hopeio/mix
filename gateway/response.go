package gateway

import (
	"fmt"
	"net/http"
	"net/textproto"

	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/mix"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
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

// FinalizeStreamTrailers writes the Error-Code trailer and custom metadata at end of an HTTP stream.
// Once the body has started, response headers are locked; status must go in trailers (Error-Code only, like unary HTTP).
func FinalizeStreamTrailers(w http.ResponseWriter, started bool, err error, trailers metadata.MD) {
	if !started {
		return
	}
	if err != nil {
		mix.WriteErrHeaders(w.Header(), ErrRespFromError(err).Code)
	} else {
		w.Header().Set(mix.HeaderErrorCode, "0")
	}
	HandleForwardResponseTrailer(w, trailers)
}

var HandleError = func(w http.ResponseWriter, r *http.Request, err error) {
	// Client already gone: writing an error body only triggers cancel races
	// (and otel/net/http "superfluous WriteHeader" noise).
	if r != nil && r.Context().Err() != nil {
		return
	}
	s := ErrRespFromError(err)
	delete(r.Header, httpx.HeaderTrailer)
	buf, contentType, _ := mix.DefaultMarshal(r.Context(), s)
	header := w.Header()
	header.Set(httpx.HeaderContentType, contentType)
	mix.WriteErrHeaders(header, s.Code)
	ow := w
	if uw, ok := w.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(buf, s)
	}
	// WriteHeader first: business errors often stay HTTP 200 (distinguished by Error-Code);
	// without it, recorder.StatusCode stays 0 and access logs are wrong.
	w.WriteHeader(mix.StatusFromErrCode(s.Code))
	if _, err := w.Write(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}

func ErrRespFromError(err error) *mix.ErrResp {
	return mix.ErrRespFrom(err)
}
