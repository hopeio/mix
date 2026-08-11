# mix

[![Go Reference](https://pkg.go.dev/badge/github.com/hopeio/mix.svg)](https://pkg.go.dev/github.com/hopeio/mix)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[中文文档](README.zh-CN.md)

**HTTP and gRPC in one process — often on the same port.**

![server](_assets/server.webp)

```bash
go get github.com/hopeio/mix@latest
```

## What is mix?

**mix** is a small Go microservice runtime. You provide an `http.Handler` and optionally register gRPC services; mix listens (default `:8080`), demultiplexes by `Content-Type`, and runs both stacks until graceful shutdown.

Extras that usually take a weekend to wire: access logs, unified error codes, request binding, CORS, middleware, OpenTelemetry, optional HTTP/3, and an internal port for OpenAPI (Redoc) / pprof.

HTTP is framework-agnostic (`net/http`). Gin and Fiber adapters live under `contrib/`. `gateway/` maps Unary and streaming RPCs onto HTTP handlers.

## Features

- **Same-port multiplexing** — HTTP/1.1 + cleartext HTTP/2 (gRPC); optional HTTP/3 (QUIC)
- **Any `http.Handler`** — stdlib mux, Gin, Fiber, …
- **Gateway** — `UnaryCall` / streaming helpers under `gateway/` (plus `contrib/gin`, `contrib/fiber`)
- **Binding** — `uri` / `query` / `header` / `form` / `json` with validation
- **Error model** — codes aligned with gRPC; shared HTTP/gRPC response helpers
- **Access log** — HTTP and gRPC, optional body capture and path filters
- **OpenTelemetry** — tracing/metrics hooks
- **Internal server** — default `:8081` for docs and pprof
- **Lifecycle** — `SIGINT` / `SIGTERM` stops gRPC then HTTP; inject hooks for DI-style setup

## Architecture

```
  Clients ──► :8080  main listener
              ├─ gRPC Content-Type  → grpc.Server
              └─ otherwise          → http.Handler

  Ops     ──► :8081  internal (OpenAPI / pprof)
```

## Minimal example

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
			w.Write([]byte("hello"))
		})),
		mix.WithGrpcHandler(func(s *grpc.Server) {
			// pb.RegisterYourServiceServer(s, &impl{})
		}),
	).Run()
}
```

```bash
go run ./_example
```

## Options

| Option | Role |
|--------|------|
| `WithHttpHandler` | Main HTTP handler (required for HTTP traffic) |
| `WithGrpcHandler` | Register gRPC services |
| `WithHttp` | Tune `http.Server` (addr, timeouts, …) |
| `WithHTTP3` | Enable HTTP/3 |
| `WithInternalServer` | Internal listen address |
| `WithCors` | CORS |
| `WithOtel` | OpenTelemetry |
| `WithMiddleware` | HTTP middleware chain |
| `WithGrpc` | gRPC interceptors / `ServerOption`s |

## Request metadata

```go
md := mix.GetMetadata(ctx)
if md != nil {
	_ = md.TraceId
	_ = md.Token
}
```

## Default ports

| Port | Use |
|------|-----|
| `:8080` | Public HTTP + gRPC |
| `:8081` | OpenAPI / pprof |

## License

[MIT](LICENSE)
