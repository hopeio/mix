package fiber

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/gox/net/http/grpc/gateway"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx fiber.Ctx, message proto.Message) error {
	var contentType string
	var buf []byte
	var err error
	switch rb := message.(type) {
	case httpx.Responder:
		rb.Respond(ctx, NewResponseWriter(ctx))
		return nil
	case httpx.ResponseBody:
		buf, contentType = rb.ResponseBody()
	case httpx.XXXResponseBody:
		buf, contentType, err = gatewayx.DefaultMarshal(ctx, rb.XXX_ResponseBody())
		if err != nil {
			return err
		}
	default:
		buf, contentType, err = gatewayx.DefaultMarshal(ctx, message)
		if err != nil {
			return err
		}
	}
	ctx.Response().Header.Set(httpx.HeaderContentType, contentType)
	_, err = ctx.Write(buf)
	return err
}

var HttpError = func(ctx fiber.Ctx, err error) {
	s := gatewayx.ErrRespFromError(err)
	errcodeHeader := strconv.Itoa(int(s.Code))
	buf, contentType, _ := gatewayx.DefaultMarshal(ctx.Context(), s)
	ctx.Set(httpx.HeaderContentType, contentType)
	ctx.Set(httpx.HeaderGrpcStatus, errcodeHeader)
	ctx.Set(httpx.HeaderErrorCode, errcodeHeader)
	if err := ctx.Send(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}
