# mix

[![Go Reference](https://pkg.go.dev/badge/github.com/hopeio/mix.svg)](https://pkg.go.dev/github.com/hopeio/mix)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[中文文档](README.zh-CN.md)

```bash
go get github.com/hopeio/mix@latest
```

**mix** runs HTTP and gRPC together. Hand them an `http.Handler` and a gRPC registrar; it listens (default `:8080`), routes by `Content-Type`, and shuts down cleanly on signal.

## When to use it

You want one binary and usually one public port for browsers/REST *and* gRPC clients, without maintaining two servers and two Service ports. mix also ships the glue people usually copy-paste: binding, error responses, access logs, CORS, middleware, OpenTelemetry, optional HTTP/3, and an internal listen address for OpenAPI/pprof.

## Behavior

```
inbound :8080
  ├─ Content-Type looks like gRPC  →  grpc.Server.ServeHTTP
  └─ everything else               →  your http.Handler

inbound :8081 (optional internal)
  └─ OpenAPI (Redoc) / pprof / metrics hooks
```

HTTP stays on `net/http`. Prefer Gin or Fiber? Use `contrib/gin` or `contrib/fiber`. Map RPC methods to HTTP with `gateway` (stdlib) or the contrib wrappers.

## Feature list

- Same-listener HTTP/1.1 + h2c gRPC; optional QUIC HTTP/3
- `WithHttpHandler` / `WithGrpcHandler` and functional options for timeouts, CORS, OTel, middleware
- Request binding: path, query, header, form, JSON + validation
- Error codes aligned with gRPC status; shared response helpers
- Access logging for HTTP and gRPC (path include/exclude)
- Metadata on context (`TraceId`, token, custom keys)
- Graceful stop on `SIGINT` / `SIGTERM`
- Inject hooks (`BeforeInject` / `AfterInject`) if you compose mix inside a DI bootstrap

## Smallest server

```go
package main

import (
	"net/http"

	"github.com/hopeio/mix"
	"google.golang.org/grpc"
)

func main() {
	mix.NewServer(
		mix.WithHttpHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})),
		mix.WithGrpcHandler(func(gs *grpc.Server) {
			// pb.RegisterXxxServer(gs, impl)
		}),
	).Run()
}
```

```bash
go run ./_example
```

## Options cheat sheet

| Option | Effect |
|--------|--------|
| `WithHttpHandler` | Public HTTP stack |
| `WithGrpcHandler` | Register gRPC services |
| `WithHttp` | Mutate `http.Server` (addr, timeouts) |
| `WithHTTP3` | Enable HTTP/3 + cert paths |
| `WithInternalServer` | Docs / pprof listener |
| `WithCors` | CORS policy |
| `WithOtel` | Tracing / metrics |
| `WithMiddleware` | HTTP middleware chain |
| `WithGrpc` | Interceptors and `grpc.ServerOption` |

## Metadata

```go
md := mix.GetMetadata(ctx)
if md != nil {
	trace := md.TraceId
	_ = trace
}
```

## Binding & errors

```go
type Q struct {
	ID int `uri:"id" validate:"required"`
}
var q Q
if err := mix.Bind(r, &q); err != nil {
	mix.ServeError(w, err)
	return
}
mix.ServeSuccess(w, data)
```

## Gateway helpers

`github.com/hopeio/mix/gateway` exposes `UnaryCall` and streaming adapters so an existing gRPC method can be mounted on an `http.ServeMux`. Gin/Fiber equivalents live under `contrib/`.

## Defaults

| Address | Role |
|---------|------|
| `:8080` | Public HTTP + gRPC |
| `:8081` | Internal diagnostics / OpenAPI |

## License

[MIT](LICENSE)
