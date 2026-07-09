package fiber

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/hopeio/gox/types"
	"github.com/hopeio/mix"
	"google.golang.org/grpc"
)


func UnaryCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp]](
	handler mix.GrpcHandler[Req, Resp, ReqPtr, RespPtr],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var req Req

		if err := Bind(ctx, &req); err != nil {
			HttpError(ctx, err)
			return nil
		}

		stream := NewServerTransportStream[Req, Resp, ReqPtr, RespPtr](ctx)

		resp, err := handler(grpc.NewContextWithServerTransportStream(stream.Context(), stream), &req)
		if err != nil {
			HttpError(ctx, err)
			return nil
		}

		err = HandleResponseMessage(ctx, resp)
		if err != nil {
			HttpError(ctx, err)
			return err
		}
		return nil
	}
}

func ServerSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ServerSideStream[Resp, RespPtr]](
	handler mix.ServerSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
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
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(&req, any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}

func ClientSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ClientSideStream[Req, Resp, ReqPtr, RespPtr]](
	handler mix.ClientSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		stream.forClientRecv()

		if err := handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}

func BidiStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.BidiStream[Req, Resp, ReqPtr, RespPtr]](
	handler mix.BidiStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		var err error

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return nil
		}
		return nil
	}
}


type Service[REQ, RESP any] func(fiber.Ctx, REQ) (RESP, *mix.ErrResp)

func HandlerWrap[REQ, RESP any](service Service[*REQ, *RESP]) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		req := new(REQ)
		err := Bind(ctx, req)
		if err != nil {
			httpReq, _ := http.NewRequestWithContext(ctx.RequestCtx(), ctx.Method(), ctx.OriginalURL(), nil)
			mix.ServeError(NewResponseWriter(ctx), httpReq, mix.InvalidArgument.Msg(err.Error()))
			return nil
		}

		res, reserr := service(ctx, req)
		if reserr != nil {
			mix.RespondError(ctx, NewResponseWriter(ctx), reserr)
			return nil
		}
		if httpres, ok := any(res).(mix.Responder); ok {
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
			httpReq, _ := http.NewRequestWithContext(ctx.RequestCtx(), ctx.Method(), ctx.OriginalURL(), nil)
			mix.ServeError(NewResponseWriter(ctx), httpReq, mix.InvalidArgument.Msg(err.Error()))
			return nil
		}

		res, reserr := service(ctx.RequestCtx(), req)
		if reserr != nil {
			httpReq, _ := http.NewRequestWithContext(ctx.RequestCtx(), ctx.Method(), ctx.OriginalURL(), nil)
			mix.ServeError(NewResponseWriter(ctx), httpReq, reserr)
			return nil
		}
		if httpres, ok := any(res).(mix.Responder); ok {
			httpres.Respond(ctx, NewResponseWriter(ctx))
			return nil
		}
		Respond(ctx, res)
		return nil
	}
}

func Respond(ctx fiber.Ctx, v any) {
	if err, ok := v.(error); ok {
		mix.RespondError(ctx, NewResponseWriter(ctx), err)
		return
	}
	mix.RespondSuccess(ctx, NewResponseWriter(ctx), v)
}
