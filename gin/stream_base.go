package gin

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	gatewayx "github.com/hopeio/mix/http/gateway"
	httpx "github.com/hopeio/gox/net/http"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type ginStreamBase struct {
	ctx         *gin.Context
	method      string
	trailers    metadata.MD
	started     bool
	contentType string
}

func newGinStreamBase(ctx *gin.Context) ginStreamBase {
	return ginStreamBase{
		ctx:         ctx,
		contentType: ctx.ContentType(),
	}
}

func (b *ginStreamBase) Context() context.Context { return b.ctx.Request.Context() }

func (b *ginStreamBase) Method() string { return b.method }

func (b *ginStreamBase) Trailer() metadata.MD { return b.trailers }

func (b *ginStreamBase) Status() bool { return b.started }

func (b *ginStreamBase) SetHeader(md metadata.MD) error {
	for k, vs := range md {
		for _, v := range vs {
			if strings.HasSuffix(k, "-bin") {
				b.ctx.Header(k, base64.StdEncoding.EncodeToString([]byte(v)))
			} else {
				b.ctx.Header(k, v)
			}
		}
	}
	return nil
}

func (b *ginStreamBase) SendHeader(md metadata.MD) error {
	_ = b.SetHeader(md)
	return nil
}

func (b *ginStreamBase) setTrailer(md metadata.MD) {
	b.trailers = metadata.Join(b.trailers, md)
	if b.started {
		gatewayx.HandleForwardResponseTrailerHeader(b.ctx.Writer, md)
	}
}

func (b *ginStreamBase) bindContext(ctx context.Context) {
	b.ctx.Request = b.ctx.Request.WithContext(ctx)
}

func (b *ginStreamBase) FinalizeTrailers(err error) {
	b.finalize(err)
}

func (b *ginStreamBase) finalize(err error) {
	gatewayx.FinalizeStreamTrailers(b.ctx.Writer, b.started, err, b.trailers)
}

func (b *ginStreamBase) sendFrame(msg proto.Message) error {
	data, contentType, err := gatewayx.DefaultMarshal(b.ctx.Request.Context(), msg)
	if err != nil {
		return err
	}
	if !b.started {
		b.started = true
		gatewayx.BeginGRPCStream(b.ctx.Writer, b.trailers)
		b.ctx.Header(httpx.HeaderContentType, contentType)
		b.ctx.Writer.WriteHeader(200)
	}
	frame := make([]byte, 5+len(data))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data)))
	copy(frame[5:], data)
	if _, err := b.ctx.Writer.Write(frame); err != nil {
		return err
	}
	b.ctx.Writer.Flush()
	return nil
}

func (b *ginStreamBase) recvFrame() ([]byte, error) {
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(b.ctx.Request.Body, hdr); err != nil {
		return nil, err
	}
	if hdr[0] != 0 {
		return nil, status.Error(codes.Unimplemented, "compressed frames not supported")
	}
	length := binary.BigEndian.Uint32(hdr[1:5])
	payload := make([]byte, length)
	if _, err := io.ReadFull(b.ctx.Request.Body, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
