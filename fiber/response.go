package fiber

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/gox/net/http/grpc/gateway"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx fiber.Ctx, message proto.Message) error {
	var contentType string
	var buf []byte
	switch rb := message.(type) {
	case httpx.Responder:
		rb.Respond(ctx, NewResponseWriter(ctx))
		return nil
	case httpx.ResponseBody:
		buf, contentType = rb.ResponseBody()
	case httpx.XXXResponseBody:
		buf, contentType = gatewayx.DefaultMarshal(ctx, rb.XXX_ResponseBody())
	default:
		buf, contentType = gatewayx.DefaultMarshal(ctx, message)
	}
	ctx.Response().Header.Set(httpx.HeaderContentType, contentType)
	_, err := ctx.Write(buf)
	return err
}



var HttpError = func(ctx fiber.Ctx, err error) {
	s, ok := status.FromError(err)
	if !ok {
		grpclog.Warningf("Failed to convert error to status: %v", err)
	}
	errcodeHeader := strconv.Itoa(int(s.Code()))
	buf, contentType := gatewayx.DefaultMarshal(ctx.Context(), s)
	ctx.Set(httpx.HeaderContentType, contentType)
	ctx.Set(httpx.HeaderGrpcStatus, errcodeHeader)
	ctx.Set(httpx.HeaderErrorCode, errcodeHeader)
	if err := ctx.Send(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}
