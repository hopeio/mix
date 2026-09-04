package gateway

import (
	"context"
	"net/http"
	"strings"

	"github.com/hopeio/mix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func UnaryCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp]](gprcHanlder mix.GrpcHandler[Req, Resp, ReqPtr, RespPtr]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Req

		if err := mix.Bind(r, &req); err != nil {
			HandleError(w, r, err)
			return
		}

		stream := NewServerTransportStream[Req, Resp, ReqPtr, RespPtr](w, r)
		resp, err := gprcHanlder(grpc.NewContextWithServerTransportStream(stream.Context(), stream), &req)
		if err != nil {
			HandleError(w, r, err)
			return
		}

		err = HandleResponseMessage(w, r, resp)
		if err != nil {
			HandleError(w, r, err)
			return
		}
	})
}

func ServerSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ServerSideStream[Resp, RespPtr]](gprcHanlder mix.ServerSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Req
		var err error

		if err = mix.Bind(r, &req); err != nil {
			HandleError(w, r, err)
			return
		}

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](w, r)
		stream.forServerSendOnly()

		defer func() {
			FinalizeStreamTrailers(w, stream.Status(), err, stream.Trailer())
		}()
		if err = gprcHanlder(&req, any(stream).(S)); err != nil {
			// 流已开写：只能靠 trailer 的 Error-Code，不能再写一元错误体。
			if !stream.Status() {
				HandleError(w, r, err)
			}
			return
		}
	})
}

func ClientSideStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.ClientSideStream[Req, Resp, ReqPtr, RespPtr] ](gprcHanlder mix.ClientSideStreamHandler[Req, Resp, ReqPtr, RespPtr, S]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](w, r)
		stream.forClientRecv()

		if err := gprcHanlder(any(stream).(S)); err != nil {
			HandleError(w, r, err)
			return
		}
	})
}

func BidiStreamCall[Req, Resp any, ReqPtr mix.ProtoMessage[Req], RespPtr mix.ProtoMessage[Resp], S mix.BidiStream[Req, Resp, ReqPtr, RespPtr], GprcHandler mix.BidiStreamHandler[Req, Resp, ReqPtr, RespPtr, S]](gprcHanlder GprcHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error

		stream := NewServerStream[Req, Resp, ReqPtr, RespPtr](w, r)
		defer func() {
			FinalizeStreamTrailers(w, stream.Status(), err, stream.Trailer())
		}()
		if err = gprcHanlder(any(stream).(S)); err != nil {
			if !stream.Status() {
				HandleError(w, r, err)
			}
			return
		}
	})
}

// NewMetadataContext 设置 incoming
//
// 必须显式小写化键，而不是 metadata.MD(header) 直接转换：后者保留 net/http 的
// 规范化写法（如 "X-Internal-Auth"），而真实 gRPC 链路上的键是小写的
// （"x-internal-auth"）。不小写化的话，同一份 md.Get 代码在 HTTP 网关路径
// 与真实 gRPC 路径上会一边命中一边漏判。
func NewMetadataContext(ctx context.Context, header http.Header) context.Context {
	md := make(metadata.MD, len(header))
	for k, vals := range header {
		md[strings.ToLower(k)] = append(md[strings.ToLower(k)], vals...)
	}
	return metadata.NewIncomingContext(ctx, md)
}
