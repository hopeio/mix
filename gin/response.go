package gin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/gox/net/http/grpc/gateway"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx *gin.Context, message proto.Message) error {
	return gatewayx.HandleResponseMessage(ctx.Writer, ctx.Request, message)
}

var HttpError = func(ctx *gin.Context, err error) {
		s, _ := status.FromError(err)
		delete(ctx.Request.Header, httpx.HeaderTrailer)
		errcode := strconv.Itoa(int(s.Code()))
		ctx.Header(httpx.HeaderGrpcStatus, errcode)
		ctx.Header(httpx.HeaderErrorCode, errcode)
		message := s.Proto()

		buf, contentType,_ := gatewayx.DefaultMarshal(ctx, message)

		ctx.Header(httpx.HeaderContentType, contentType)
		ow := ctx.Writer.(http.ResponseWriter)
		if uw, ok := ctx.Writer.(httpx.Unwrapper); ok {
			ow = uw.Unwrap()
		}
		if recorder, ok := ow.(httpx.RecordBodyer); ok {
			recorder.RecordBody(buf, message)
		}
		ctx.Writer.Write(buf)
}
