package fiber

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	httpx "github.com/hopeio/gox/net/http"
	mix_http "github.com/hopeio/mix/http"
	gatewayx "github.com/hopeio/mix/http/gateway"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

var HandleResponseMessage = func(ctx fiber.Ctx, message proto.Message) error {
	var contentType string
	var buf []byte
	var err error
	switch rb := message.(type) {
	case mix_http.Responder:
		rb.Respond(ctx, NewResponseWriter(ctx))
		return nil
	case mix_http.ResponseBody:
		buf, contentType = rb.ResponseBody()
	case mix_http.XXXResponseBody:
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
	buf, contentType, _ := gatewayx.DefaultMarshal(ctx.RequestCtx(), s)
	ctx.Set(httpx.HeaderContentType, contentType)
	ctx.Set(httpx.HeaderGrpcStatus, errcodeHeader)
	ctx.Set(httpx.HeaderErrorCode, errcodeHeader)
	if err := ctx.Send(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}
}
