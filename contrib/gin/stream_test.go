package gin

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	jsonx "github.com/hopeio/gox/encoding/json"
	"github.com/hopeio/mix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func encodeFrame(t *testing.T, msg any) []byte {
	t.Helper()
	data, err := jsonx.Marshal(&mix.CommonAnyResp{Data: msg})
	if err != nil {
		t.Fatal(err)
	}
	frame := make([]byte, 5+len(data))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(data)))
	copy(frame[5:], data)
	return frame
}

func TestClientStreamRecvAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := io.MultiReader(
		bytes.NewReader(encodeFrame(t, &wrapperspb.StringValue{Value: "a"})),
		bytes.NewReader(encodeFrame(t, &wrapperspb.StringValue{Value: "b"})),
	)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/upload", body)

	stream := NewServerStream[wrapperspb.StringValue, wrapperspb.Int64Value, *wrapperspb.StringValue, *wrapperspb.Int64Value](ctx)
	stream.forClientRecv()

	msg1, err := stream.Recv()
	if err != nil || msg1.GetValue() != "a" {
		t.Fatalf("first recv: msg=%+v err=%v", msg1, err)
	}
	msg2, err := stream.Recv()
	if err != nil || msg2.GetValue() != "b" {
		t.Fatalf("second recv: msg=%+v err=%v", msg2, err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	want := wrapperspb.Int64(42)
	if err := stream.SendAndClose(want); err != nil {
		t.Fatal(err)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected response body written")
	}
	res := w.Result()
	if got := res.Trailer.Get("Grpc-Status"); got != "0" {
		t.Fatalf("Grpc-Status: got %q want 0", got)
	}
}

func TestServerStreamSatisfiesGRPCServerStream(t *testing.T) {
	var s *ServerStream[emptypb.Empty, wrapperspb.Int64Value, *emptypb.Empty, *wrapperspb.Int64Value]
	var _ grpc.ServerStream = s
}

func TestServerStreamTrailers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/search", nil)

	stream := NewServerStream[emptypb.Empty, wrapperspb.Int64Value, *emptypb.Empty, *wrapperspb.Int64Value](ctx)
	stream.SetTrailer(metadata.Pairs("x-custom", "hello"))

	if err := stream.Send(wrapperspb.Int64(1)); err != nil {
		t.Fatal(err)
	}
	stream.FinalizeTrailers(nil)

	res := w.Result()
	if got := res.Trailer.Get("Grpc-Status"); got != "0" {
		t.Fatalf("Grpc-Status: got %q want 0", got)
	}
	if got := res.Trailer.Get("Grpc-Trailer-X-Custom"); got != "hello" {
		t.Fatalf("custom trailer: got %q want hello", got)
	}
}
