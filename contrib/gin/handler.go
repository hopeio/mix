package gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hopeio/gox/types"
	"github.com/hopeio/mix"
	"google.golang.org/grpc"
)

func UnaryCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp]](
	handler mix.GrpcHandler[Req, Resp, ReqPtr, RespPtr],
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req

		if err := Bind(ctx, &req); err != nil {
			HttpError(ctx, err)
			return
		}
		stream := NewServerTransportStream[Req, Resp, ReqPtr, RespPtr](ctx)
		resp, err := handler(grpc.NewContextWithServerTransportStream(stream.Context(), stream), &req)
		if err != nil {
			HttpError(ctx, err)
			return
		}

		err = HandleResponseMessage(ctx, resp)
		if err != nil {
			HttpError(ctx, err)
			return
		}
	}
}

func ServerSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ServerSideStream[Resp, RespPtr]](
	handler mix.ServerSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		var err error

		if err = Bind(ctx, &req); err != nil {
			HttpError(ctx, err)
			return
		}

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		stream.forServerSendOnly()
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(&req, any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return
		}
	}
}

func ClientSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ClientSideStream[Req, Resp, ReqPtr, RespPtr]](
	handler mix.ClientSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		stream.forClientRecv()

		if err := handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return
		}
	}
}

func BidiStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.BidiStream[Req, Resp, ReqPtr, RespPtr]](
	handler mix.BidiStreamHandler[Req, Resp, ReqPtr, RespPtr, S],
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var err error

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](ctx)
		defer func() { stream.FinalizeTrailers(err) }()
		if err = handler(any(stream).(S)); err != nil {
			HttpError(ctx, err)
			return
		}
	}
}

type Service[REQ, RESP any] func(*gin.Context, REQ) (RESP, *mix.ErrResp)

func HandlerWrap[REQ, RESP any](service Service[*REQ, *RESP]) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		req := new(REQ)
		err := Bind(ctx, req)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			mix.ServeError(ctx.Writer, ctx.Request, mix.InvalidArgument.Wrap(err))
			ctx.Abort()
			return
		}
		res, reserr := service(ctx, req)
		if reserr != nil {
			mix.ServeError(ctx.Writer, ctx.Request, reserr)
			ctx.Abort()
			return
		}
		if httpres, ok := any(res).(http.Handler); ok {
			httpres.ServeHTTP(ctx.Writer, ctx.Request)
			return
		}
		if httpres, ok := any(res).(mix.Responder); ok {
			httpres.Respond(ctx, ctx.Writer)
			return
		}
		mix.ServeSuccess(ctx.Writer, ctx.Request, res)
	}
}

func HandlerWrapCommon[REQ, RESP any](service types.Service[*REQ, *RESP]) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		req := new(REQ)
		err := Bind(ctx, req)
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			mix.ServeError(ctx.Writer, ctx.Request, mix.InvalidArgument.Wrap(err))
			ctx.Abort()
			return
		}
		res, err := service(mix.WrapContext(ctx), req)
		if err != nil {
			mix.ServeError(ctx.Writer, ctx.Request, err)
			ctx.Abort()
			return
		}
		if httpres, ok := any(res).(http.Handler); ok {
			httpres.ServeHTTP(ctx.Writer, ctx.Request)
			return
		}
		if httpres, ok := any(res).(mix.Responder); ok {
			httpres.Respond(ctx, ctx.Writer)
			return
		}
		mix.ServeSuccess(ctx.Writer, ctx.Request, res)
	}
}
