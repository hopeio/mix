package gin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	mix_http "github.com/hopeio/mix/http"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/mix/http/gateway"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx *gin.Context, message proto.Message) error {
	return gatewayx.HandleResponseMessage(ctx.Writer, ctx.Request, message)
}

var HttpError = func(ctx *gin.Context, err error) {
	s := gatewayx.ErrRespFromError(err)
	delete(ctx.Request.Header, httpx.HeaderTrailer)
	errcode := strconv.Itoa(int(s.Code))
	ctx.Header(httpx.HeaderGrpcStatus, errcode)
	ctx.Header(httpx.HeaderErrorCode, errcode)

	buf, contentType, _ := gatewayx.DefaultMarshal(ctx, s)

	ctx.Header(httpx.HeaderContentType, contentType)
	ow := ctx.Writer.(http.ResponseWriter)
	if uw, ok := ctx.Writer.(httpx.Unwrapper); ok {
		ow = uw.Unwrap()
	}
	if recorder, ok := ow.(httpx.RecordBodyer); ok {
		recorder.RecordBody(buf, s)
	}
	ctx.Writer.Write(buf)
}

func Respond(ctx *gin.Context, v any) {
	if err, ok := v.(error); ok {
		mix_http.ServeError(ctx.Writer, ctx.Request, err)
		return
	}
	mix_http.ServeSuccess(ctx.Writer, ctx.Request, v)
}