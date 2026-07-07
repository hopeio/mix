package fiber

import (
	"bufio"
	"context"
	"iter"
	"net/http"

	"github.com/gofiber/fiber/v3"
	iox "github.com/hopeio/gox/io"
	httpx "github.com/hopeio/gox/net/http"
)

// ResponseWriter 将 fiber.Ctx 适配为 http.ResponseWriter（含 Flush），供 gateway 流式写出复用。
type ResponseWriter struct {
	fiber.Ctx
	reqHeader   http.Header
	respHeader  http.Header
	wroteHeader bool
}

func (w *ResponseWriter) Header() http.Header {
	if w.respHeader == nil {
		w.respHeader = http.Header{}
	}
	return w.respHeader
}

func (w *ResponseWriter) HeaderX() httpx.Header {
	return ResponseHeader{ResponseHeader: &w.Response().Header}
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	w.writeHeader()
	return w.Ctx.Write(p)
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.writeHeader()
	w.Ctx.Status(statusCode)
}

func (w *ResponseWriter) writeHeader() {
	if !w.wroteHeader {
		header := &w.Ctx.Response().Header
		for k, v := range w.respHeader {
			for _, vv := range v {
				header.Add(k, vv)
			}
		}
		w.wroteHeader = true
	}
}

func (w *ResponseWriter) Flush() {
	w.Response().ImmediateHeaderFlush = true
}

func (w *ResponseWriter) RespondStream(ctx context.Context, dataSource iter.Seq[iox.WriterToCloser]) {
	w.Ctx.Set(httpx.HeaderTransferEncoding, "chunked")
	w.Ctx.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		for data := range dataSource {
			_, err := data.WriteTo(w)
			if err != nil {
				return
			}
			w.Flush()
		}
	})
}

var _ http.Flusher = (*ResponseWriter)(nil)

func NewResponseWriter(ctx fiber.Ctx) *ResponseWriter {
	return &ResponseWriter{Ctx: ctx}
}

func fiberReqHeader(ctx fiber.Ctx) http.Header {
	h := make(http.Header)
	for key, value := range ctx.Request().Header.All() {
		h.Add(string(key), string(value))
	}
	return h
}
