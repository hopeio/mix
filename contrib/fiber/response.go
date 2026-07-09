package fiber

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/mix"
	gatewayx "github.com/hopeio/mix/gateway"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx fiber.Ctx, message proto.Message) error {
	var contentType string
	var buf []byte
	var err error
	switch rb := message.(type) {
	case mix.Responder:
		rb.Respond(ctx, NewResponseWriter(ctx))
		return nil
	case mix.ResponseBody:
		buf, contentType = rb.ResponseBody()
	case mix.XXXResponseBody:
		buf, contentType, err = mix.DefaultMarshal(ctx, rb.XXX_ResponseBody())
		if err != nil {
			return err
		}
	default:
		buf, contentType, err = mix.DefaultMarshal(ctx, message)
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
	buf, contentType, _ := mix.DefaultMarshal(ctx.RequestCtx(), s)
	ctx.Set(httpx.HeaderContentType, contentType)
	ctx.Set(httpx.HeaderGrpcStatus, errcodeHeader)
	ctx.Set(httpx.HeaderErrorCode, errcodeHeader)
	if err := ctx.Send(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}
