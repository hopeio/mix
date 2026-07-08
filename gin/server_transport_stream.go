package gin

import (
	"io"

	"github.com/gin-gonic/gin"
	grpcx "github.com/hopeio/gox/net/http/grpc"
	gatewayx "github.com/hopeio/mix/http/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ServerTransportStream 双向 streaming；Unary 路径也用它承载 metadata（grpc.SetHeader 等）。
type ServerTransportStream[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp]] struct {
	ginStreamBase
	closed bool
}

func NewServerTransportStream[Req, Resp any, ReqPtr grpcx.ProtoMessage[Req], RespPtr grpcx.ProtoMessage[Resp]](ctx *gin.Context) *ServerTransportStream[Req, Resp, ReqPtr, RespPtr] {
	return  &ServerTransportStream[Req, Resp, ReqPtr, RespPtr]{ginStreamBase: newGinStreamBase(ctx)}
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
	return gatewayx.DefaultUnmarshal(s.ctx, s.contentType, data, pm)
}

func (s *ServerTransportStream[Req, Resp, ReqPtr, RespPtr]) SendMsg(m any) error {
	msg, ok := m.(RespPtr)
	if !ok {
		return status.Errorf(codes.Internal, "SendMsg: unexpected message type %T, expected %T", m, new(Resp))
	}
	return s.Send(msg)
}
