package fiber

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"strings"

	"github.com/gofiber/fiber/v3"
	httpx "github.com/hopeio/gox/net/http"
	gatewayx "github.com/hopeio/mix/gateway"
	"github.com/hopeio/mix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type fiberStreamBase struct {
	ctx         fiber.Ctx
	metaCtx     context.Context
	w           *ResponseWriter
	method      string
	trailers    metadata.MD
	started     bool
	contentType string
}

func newFiberStreamBase(ctx fiber.Ctx) fiberStreamBase {
	w := NewResponseWriter(ctx)
	return fiberStreamBase{
		ctx:         ctx,
		metaCtx:     gatewayx.NewMetadataContext(ctx, w.reqHeader),
		w:           w,
		contentType: string(ctx.Request().Header.ContentType()),
	}
}

func (b *fiberStreamBase) Context() context.Context { return b.metaCtx }

func (b *fiberStreamBase) Method() string { return b.method }

func (b *fiberStreamBase) Trailer() metadata.MD { return b.trailers }

func (b *fiberStreamBase) Status() bool { return b.started }

func (b *fiberStreamBase) SetHeader(md metadata.MD) error {
	for k, vs := range md {
		for _, v := range vs {
			if strings.HasSuffix(k, "-bin") {
				b.ctx.Set(k, base64.StdEncoding.EncodeToString([]byte(v)))
			} else {
				b.ctx.Set(k, v)
			}
		}
	}
	return nil
}

func (b *fiberStreamBase) SendHeader(md metadata.MD) error {
	_ = b.SetHeader(md)
	return nil
}

func (b *fiberStreamBase) setTrailer(md metadata.MD) {
	b.trailers = metadata.Join(b.trailers, md)
	if b.started {
		gatewayx.HandleForwardResponseTrailerHeader(b.w, md)
	}
}

func (b *fiberStreamBase) bindContext(c context.Context) {
	b.ctx.SetContext(c)
}

func (b *fiberStreamBase) FinalizeTrailers(err error) {
	b.finalize(err)
}

func (b *fiberStreamBase) finalize(err error) {
	gatewayx.FinalizeStreamTrailers(b.w, b.started, err, b.trailers)
}

func (b *fiberStreamBase) sendFrame(msg proto.Message) error {
	data, contentType, err := mix.DefaultMarshal(b.metaCtx, msg)
	if err != nil {
		return err
	}
	if !b.started {
		b.started = true
		gatewayx.BeginGRPCStream(b.w, b.trailers)
		b.ctx.Set(httpx.HeaderContentType, contentType)
		b.ctx.Status(fiber.StatusOK)
	}
	if err := gatewayx.WriteGRPCFrameData(b.w, data); err != nil {
		return err
	}
	b.w.Flush()
	return nil
}

func (b *fiberStreamBase) recvFrame() ([]byte, error) {
	body := b.ctx.Request().BodyStream()
	hdr := make([]byte, 5)
	if _, err := io.ReadFull(body, hdr); err != nil {
		return nil, err
	}
	if hdr[0] != 0 {
		return nil, status.Error(codes.Unimplemented, "compressed frames not supported")
	}
	length := binary.BigEndian.Uint32(hdr[1:5])
	payload := make([]byte, length)
	if _, err := io.ReadFull(body, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
