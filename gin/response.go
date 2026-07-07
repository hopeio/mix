package gin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/gox/net/http/grpc/gateway"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx *gin.Context, message proto.Message) {
	_ = gatewayx.HandleResponseMessage(ctx.Writer, ctx.Request, message)
}

var HttpError = func(ctx *gin.Context, err error) {
	s, ok := status.FromError(err)
	if !ok {
		grpclog.Warningf("Failed to convert error to status: %v", err)
	}
	delete(ctx.Request.Header, httpx.HeaderTrailer)
	errcodeHeader := strconv.Itoa(int(s.Code()))
	buf, contentType := gatewayx.DefaultMarshal(ctx, s)
	ctx.Header(httpx.HeaderContentType, contentType)
	ctx.Header(httpx.HeaderGrpcStatus, errcodeHeader)
	ctx.Header(httpx.HeaderErrorCode, errcodeHeader)
	if _, err := ctx.Writer.Write(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}
