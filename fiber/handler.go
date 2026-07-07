package fiber

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/hopeio/gox/errors"
	httpx "github.com/hopeio/gox/net/http"
	grpcx "github.com/hopeio/gox/net/http/grpc"
	gatewayx "github.com/hopeio/gox/net/http/grpc/gateway"
	"github.com/hopeio/gox/types"
)

func withMetadataContext(ctx fiber.Ctx, stream interface {
	bindContext(context.Context)
}) context.Context {
	c := gatewayx.NewMetadataContext(ctx.Context(), fiberReqHeader(ctx))
	stream.bindContext(c)
	return c
}

func UnaryCall[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp]](
	handler grpcx.GrpcHandler[Req, Resp, ReqPtr, RespPtr],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var req Req

		if err := Bind(ctx, &req); err != nil {
			HttpError(ctx, err)
			return nil
		}

		stream := NewServerTransportStream[Req, Resp, ReqPtr, RespPtr](ctx)
		ctx.SetContext(withMetadataContext(ctx, stream))

		resp, err := handler(ctx.Context(), &req)
		if err != nil {
			HttpError(ctx, err)
			return nil
		}

		HandleResponseMessage(ctx, resp)
		return nil
	}
}

func ServerSideStreamCall[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp], S grpcx.ServerSideStream[Resp, RespPtr]](
	handler grpcx.ServerSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var req Req
		var err error

		if err = Bind(ctx, &req); err != nil {
			HttpError(ctx, err)
			return nil
		}

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		stream.forServerSendOnly()
		ctx.SetContext(withMetadataContext(ctx, stream))
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(&req, any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}

func ClientSideStreamCall[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp], S grpcx.ClientSideStream[Req, Resp, ReqPtr, RespPtr]](
	handler grpcx.ClientSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		stream.forClientRecv()
		ctx.SetContext(withMetadataContext(ctx, stream))

		if err := handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}

func BidiStreamCall[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp], S grpcx.BidiStream[Req, Resp, ReqPtr, RespPtr]](
	handler grpcx.BidiStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var err error

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		ctx.SetContext(withMetadataContext(ctx, stream))
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}


type Service[REQ, RESP any] func(fiber.Ctx, REQ) (RESP, *httpx.ErrResp)

func HandlerWrap[REQ, RESP any](service Service[*REQ, *RESP]) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		req := new(REQ)
		err := Bind(ctx, req)
		if err != nil {
			httpReq, _ := http.NewRequestWithContext(ctx.Context(), ctx.Method(), ctx.OriginalURL(), nil)
			httpx.ServeError(NewResponseWriter(ctx), httpReq, errors.InvalidArgument.Msg(err.Error()))
			return nil
		}

		res, reserr := service(ctx, req)
		if reserr != nil {
			httpx.RespondError(ctx, NewResponseWriter(ctx), reserr)
			return nil
		}
		if httpres, ok := any(res).(httpx.Responder); ok {
			httpres.Respond(ctx, NewResponseWriter(ctx))
			return nil
		}
		Respond(ctx, res)
		return nil
	}
}


func HandlerWrapCommon[REQ, RESP any](service types.Service[*REQ, *RESP]) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		req := new(REQ)
		err := Bind(ctx, req)
		if err != nil {
			httpReq, _ := http.NewRequestWithContext(ctx.Context(), ctx.Method(), ctx.OriginalURL(), nil)
			httpx.ServeError(NewResponseWriter(ctx), httpReq, errors.InvalidArgument.Msg(err.Error()))
			return nil
		}

		res, reserr := service(ctx.Context(), req)
		if reserr != nil {
			httpReq, _ := http.NewRequestWithContext(ctx.Context(), ctx.Method(), ctx.OriginalURL(), nil)
			httpx.ServeError(NewResponseWriter(ctx), httpReq, reserr)
			return nil
		}
		if httpres, ok := any(res).(httpx.Responder); ok {
			httpres.Respond(ctx, NewResponseWriter(ctx))
			return nil
		}
		Respond(ctx, res)
		return nil
	}
}

func Respond(ctx fiber.Ctx, v any) {
	if err, ok := v.(error); ok {
		httpx.RespondError(ctx, NewResponseWriter(ctx), err)
		return
	}
	httpx.RespondSuccess(ctx, NewResponseWriter(ctx), v)
}
