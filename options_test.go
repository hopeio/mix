package mix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
	"github.com/rs/cors"
	"google.golang.org/grpc"
)

func TestWithContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{ k string }{"k"}, "v")
	s := NewServer(WithContext(ctx))
	if s.BaseContext != ctx {
		t.Fatal("BaseContext not set by WithContext")
	}
}

func TestWithHttp(t *testing.T) {
	s := NewServer(WithHttp(func(hs *http.Server) {
		hs.Addr = ":9090"
		hs.ReadTimeout = 0
	}))
	if s.Addr != ":9090" {
		t.Fatalf("Addr=%q", s.Addr)
	}
}

func TestWithHTTP3(t *testing.T) {
	s := NewServer(WithHTTP3(func(h3 *Http3Config) {
		h3.Enabled = true
		h3.Addr = ":8443"
	}))
	if !s.HTTP3.Enabled || s.HTTP3.Addr != ":8443" {
		t.Fatalf("HTTP3=%+v", s.HTTP3)
	}
}

func TestWithHttpHandler(t *testing.T) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	s := NewServer(WithHttpHandler(h))
	if s.HttpHandler == nil {
		t.Fatal("HttpHandler nil")
	}
	rec := httpx.NewRecorder(nil, httptest.NewRequest(http.MethodGet, "/", nil))
	s.HttpHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("handler not invoked")
	}
}

func TestWithGrpcHandler(t *testing.T) {
	var got *grpc.Server
	s := NewServer(WithGrpcHandler(func(gs *grpc.Server) {
		got = gs
	}))
	if s.GrpcHandler == nil {
		t.Fatal("GrpcHandler nil")
	}
	s.GrpcHandler(grpc.NewServer())
	if got == nil {
		t.Fatal("GrpcHandler callback not run")
	}
}

func TestWithInternalServer(t *testing.T) {
	s := NewServer(WithInternalServer(func(hs *http.Server) {
		hs.Addr = ":7070"
	}))
	if s.InternalServer.Addr != ":7070" {
		t.Fatalf("InternalServer.Addr=%q", s.InternalServer.Addr)
	}
}

func TestWithGrpc(t *testing.T) {
	s := NewServer(WithGrpc(func(g *GrpcConfig) {
		g.Addr = ":50051"
	}))
	if s.Grpc.Addr != ":50051" {
		t.Fatalf("Grpc.Addr=%q", s.Grpc.Addr)
	}
}

func TestWithCors(t *testing.T) {
	s := NewServer(WithCors(func(o *cors.Options) {
		o.AllowedOrigins = []string{"https://example.com"}
	}))
	if len(s.Cors.AllowedOrigins) != 1 || s.Cors.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("Cors=%+v", s.Cors.Options)
	}
}

func TestWithOtel(t *testing.T) {
	s := NewServer(WithOtel(func(o *OtelConfig) {
		o.Enabled = true
	}))
	if !s.Otel.Enabled {
		t.Fatal("Otel.Enabled not set")
	}
}

func TestWithMiddleware(t *testing.T) {
	var order []string
	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	}
	s := NewServer(WithMiddleware(mw1, mw2))
	if len(s.Middlewares) != 2 {
		t.Fatalf("Middlewares len=%d", len(s.Middlewares))
	}

	chain := httpx.UseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}), s.Middlewares...)
	rec := httpx.NewRecorder(nil, httptest.NewRequest(http.MethodGet, "/", nil))
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	// UseMiddleware 按注册顺序从里到外包裹，执行时后注册的先运行。
	if len(order) != 3 || order[0] != "mw2" || order[1] != "mw1" || order[2] != "handler" {
		t.Fatalf("middleware order=%v", order)
	}
}

func TestZeroValueServerWithOptions(t *testing.T) {
	var s Server
	s.WithOptions(
		WithHttp(func(hs *http.Server) { hs.Addr = ":3000" }),
		WithGrpc(func(g *GrpcConfig) { g.Addr = ":3001" }),
	)
	if s.Addr != ":3000" || s.Grpc.Addr != ":3001" {
		t.Fatalf("zero Server options: addr=%q grpc=%q", s.Addr, s.Grpc.Addr)
	}
}
