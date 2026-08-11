# mix

[![Go Reference](https://pkg.go.dev/badge/github.com/hopeio/mix.svg)](https://pkg.go.dev/github.com/hopeio/mix)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**HTTP and gRPC in one process — often on the same port.**

**同一进程里的 HTTP 与 gRPC——常常共用一个端口。**

![server](_assets/server.webp)

```bash
go get github.com/hopeio/mix@latest
```

---

## English

### What is mix?

**mix** is a small Go microservice runtime. You provide an `http.Handler` and optionally register gRPC services; mix listens (default `:8080`), demultiplexes by `Content-Type`, and runs both stacks until graceful shutdown.

Extras that usually take a weekend to wire: access logs, unified error codes, request binding, CORS, middleware, OpenTelemetry, optional HTTP/3, and an internal port for OpenAPI (Redoc) / pprof.

HTTP is framework-agnostic (`net/http`). Gin and Fiber adapters live under `contrib/`. `gateway/` maps Unary and streaming RPCs onto HTTP handlers.

### Features

- **Same-port multiplexing** — HTTP/1.1 + cleartext HTTP/2 (gRPC); optional HTTP/3 (QUIC)
- **Any `http.Handler`** — stdlib mux, Gin, Fiber, …
- **Gateway** — `UnaryCall` / streaming helpers under `gateway/` (plus `contrib/gin`, `contrib/fiber`)
- **Binding** — `uri` / `query` / `header` / `form` / `json` with validation
- **Error model** — codes aligned with gRPC; shared HTTP/gRPC response helpers
- **Access log** — HTTP and gRPC, optional body capture and path filters
- **OpenTelemetry** — tracing/metrics hooks
- **Internal server** — default `:8081` for docs and pprof
- **Lifecycle** — `SIGINT` / `SIGTERM` stops gRPC then HTTP; inject hooks for DI-style setup

### Architecture

```
  Clients ──► :8080  main listener
              ├─ gRPC Content-Type  → grpc.Server
              └─ otherwise          → http.Handler

  Ops     ──► :8081  internal (OpenAPI / pprof)
```

### Minimal example

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

### Options

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

### Request metadata

```go
md := mix.GetMetadata(ctx)
if md != nil {
	_ = md.TraceId
	_ = md.Token
}
```

### Default ports

| Port | Use |
|------|-----|
| `:8080` | Public HTTP + gRPC |
| `:8081` | OpenAPI / pprof |

### License

[MIT](LICENSE)

---

## 中文

### mix 是什么？

**mix** 是轻量的 Go 微服务运行时。你提供 `http.Handler`，可选注册 gRPC 服务；mix 监听（默认 `:8080`），按 `Content-Type` 分发，直到优雅退出。

常见样板它直接带好：访问日志、统一错误码、请求绑定、CORS、中间件、OpenTelemetry、可选 HTTP/3，以及用于 OpenAPI（Redoc）/ pprof 的内部端口。

HTTP 与框架解耦（标准 `net/http`）。Gin / Fiber 适配在 `contrib/`。`gateway/` 可将 Unary 与流式 RPC 挂成 HTTP Handler。

### 特性

- **同端口多路** — HTTP/1.1 + 明文 HTTP/2（gRPC）；可选 HTTP/3（QUIC）
- **任意 `http.Handler`** — 标准库 mux、Gin、Fiber…
- **Gateway** — `gateway/` 下的 `UnaryCall` / 流式辅助（另有 `contrib/gin`、`contrib/fiber`）
- **绑定** — `uri` / `query` / `header` / `form` / `json`，带校验
- **错误模型** — 与 gRPC codes 对齐；HTTP / gRPC 共用响应辅助
- **访问日志** — HTTP 与 gRPC，可记 body、可按路径过滤
- **OpenTelemetry** — 链路与指标挂钩点
- **内部服务** — 默认 `:8081` 文档与 pprof
- **生命周期** — `SIGINT` / `SIGTERM` 有序停机；提供注入钩子便于 DI 式装配

### 架构

```
  客户端 ──► :8080  主监听
            ├─ gRPC Content-Type  → grpc.Server
            └─ 其他               → http.Handler

  运维   ──► :8081  内部口（OpenAPI / pprof）
```

### 最小示例

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

### 选项

| Option | 作用 |
|--------|------|
| `WithHttpHandler` | 主 HTTP 处理器 |
| `WithGrpcHandler` | 注册 gRPC 服务 |
| `WithHttp` | 调整 `http.Server` |
| `WithHTTP3` | 启用 HTTP/3 |
| `WithInternalServer` | 内部监听地址 |
| `WithCors` | 跨域 |
| `WithOtel` | OpenTelemetry |
| `WithMiddleware` | HTTP 中间件链 |
| `WithGrpc` | gRPC 拦截器 / `ServerOption` |

### 请求元数据

```go
md := mix.GetMetadata(ctx)
if md != nil {
	_ = md.TraceId
	_ = md.Token
}
```

### 默认端口

| 端口 | 用途 |
|------|------|
| `:8080` | 对外 HTTP + gRPC |
| `:8081` | OpenAPI / pprof |

### 许可证

[MIT](LICENSE)
