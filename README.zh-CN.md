# mix

[![Go Reference](https://pkg.go.dev/badge/github.com/hopeio/mix.svg)](https://pkg.go.dev/github.com/hopeio/mix)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md)

```bash
go get github.com/hopeio/mix@latest
```

**mix** 把 HTTP 和 gRPC 跑在一起。传入 `http.Handler` 与 gRPC 注册函数；默认监听 `:8080`，按 `Content-Type` 分流，信号到达时优雅退出。

## 适用场景

希望一个二进制、通常一个对外端口同时服务浏览器/REST 与 gRPC，而不是维护两套 Server、两个 Service 端口。绑定、错误响应、访问日志、CORS、中间件、OpenTelemetry、可选 HTTP/3、内部 OpenAPI/pprof 端口等样板，也一并带上。

## 行为

```
入站 :8080
  ├─ Content-Type 像 gRPC  →  grpc.Server.ServeHTTP
  └─ 其余                  →  你的 http.Handler

入站 :8081（可选内部口）
  └─ OpenAPI（Redoc）/ pprof / metrics
```

HTTP 基于 `net/http`。要用 Gin / Fiber，看 `contrib/gin`、`contrib/fiber`。把 RPC 挂到 HTTP 用 `gateway`（标准库）或 contrib 封装。

## 功能

- 同监听 HTTP/1.1 + h2c gRPC；可选 QUIC HTTP/3
- `WithHttpHandler` / `WithGrpcHandler`，以及超时、CORS、OTel、中间件等 Option
- 请求绑定：path、query、header、form、JSON + 校验
- 错误码对齐 gRPC status；统一响应辅助函数
- HTTP / gRPC 访问日志（路径包含/排除）
- Context 元数据（`TraceId`、token、自定义键）
- `SIGINT` / `SIGTERM` 优雅停机
- `BeforeInject` / `AfterInject`，便于放进 DI 启动流程

## 最小服务

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

## Option 速查

| Option | 作用 |
|--------|------|
| `WithHttpHandler` | 对外 HTTP |
| `WithGrpcHandler` | 注册 gRPC 服务 |
| `WithHttp` | 修改 `http.Server` |
| `WithHTTP3` | 开启 HTTP/3 |
| `WithInternalServer` | 文档 / pprof |
| `WithCors` | 跨域 |
| `WithOtel` | 链路 / 指标 |
| `WithMiddleware` | HTTP 中间件 |
| `WithGrpc` | 拦截器与 `ServerOption` |

## 元数据

```go
md := mix.GetMetadata(ctx)
if md != nil {
	trace := md.TraceId
	_ = trace
}
```

## 绑定与错误

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

## Gateway

`github.com/hopeio/mix/gateway` 提供 `UnaryCall` 与流式适配，把已有 gRPC 方法挂到 `http.ServeMux`。Gin/Fiber 对等实现在 `contrib/`。

## 默认地址

| 地址 | 用途 |
|------|------|
| `:8080` | 对外 HTTP + gRPC |
| `:8081` | 内部诊断 / OpenAPI |

## 许可证

[MIT](LICENSE)
