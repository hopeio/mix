package fiber

import (
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/hopeio/mix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerTransportStream[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp]] struct {
	fiberStreamBase
	closed bool
}

func NewServerTransportStream[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp]](ctx fiber.Ctx) *ServerTransportStream[Req, Resp, ReqPtr, RespPtr] {
	return &ServerTransportStream[Req, Resp, ReqPtr, RespPtr]{fiberStreamBase: newFiberStreamBase(ctx)}
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) SetTrailer(md metadata.MD) error {
	s.setTrailer(md)
	return nil
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) Send(msg RespPtr) error {
	return s.sendFrame(msg)
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) Recv() (ReqPtr, error) {
	var msg Req
	if err := s.RecvMsg(&msg); err != nil {
		var zero ReqPtr
		return zero, err
	}
	return &msg, nil
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) RecvMsg(m any) error {
	if s.closed {
		return io.EOF
	}
	pm, ok := m.(ReqPtr)
	if !ok {
		return status.Errorf(codes.Internal, "RecvMsg: %T is not proto.Message", m)
	}
	data, err := s.recvFrame()
	if err != nil {
		return err
	}
	return mix.DefaultUnmarshal(s.metaCtx, s.contentType, data, pm)
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) SendMsg(m any) error {
	msg, ok := m.(RespPtr)
	if !ok {
		return status.Errorf(codes.Internal, "SendMsg: unexpected message type %T, expected %T", m, new(Resp))
	}
	return s.Send(msg)
}
