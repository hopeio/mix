# mix

[![Go Reference](https://pkg.go.dev/badge/github.com/hopeio/mix.svg)](https://pkg.go.dev/github.com/hopeio/mix)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**One process. HTTP and gRPC. Same port.**  
开箱即用的 Go 微服务运行时：HTTP/1.1 · 明文 HTTP/2（gRPC）· 可选 HTTP/3，内置访问日志、统一错误码、请求绑定、OpenTelemetry 与优雅关停。

适合与 [hopeio/protobuf](https://github.com/hopeio/protobuf) 工具链一起，用一份 Protobuf 契约快速长出云原生服务。

![server](_assets/server.webp)

```bash
go get github.com/hopeio/mix@latest
```

## 为什么是 mix

多数项目要么「Gin 一套 + 另起 gRPC 端口」，要么上沉重的服务网格才谈多协议。  
**mix** 把双栈收进同一监听地址：按 `Content-Type` 把流量分给 `http.Handler` 或 `grpc.Server`——本地开发与 K8s Service 都更简单。

## 特性

- **同端口多协议** — HTTP/1.1 + h2c gRPC；可选 [quic-go](https://github.com/quic-go/quic-go) HTTP/3
- **框架无关 HTTP** — 注入任意 `http.Handler`；Gin / Fiber 走 `contrib/`
- **Gateway** — `mix/gateway` 把 Unary / 流式 RPC 挂成 HTTP（stdlib；另有 gin / fiber）
- **Binding** — `uri` / `query` / `header` / `form` / `json` 一次绑定并校验
- **统一错误码** — 对齐 gRPC codes，HTTP / gRPC 同一套 `ErrResp`
- **访问日志** — HTTP / gRPC 可记 body，支持路径前缀过滤
- **OpenTelemetry** — 链路与指标，字段风格与 [gox](https://github.com/hopeio/gox) 日志对齐
- **运维口** — 默认 `:8081`：OpenAPI（Redoc）+ pprof
- **CORS · 中间件 · TLS · 优雅关停** — `SIGINT` / `SIGTERM` 有序停机
- **可注入** — 实现 `BeforeInject` / `AfterInject`，与 [initialize](https://github.com/hopeio/initialize) 无缝配合

## 架构

```
                    ┌─────────────────────────────────┐
  Client ──────────►│  :8080  主服务（HTTP + gRPC）    │
                    │  ├─ HTTP  → 你的 http.Handler   │
                    │  └─ gRPC  → grpc.Server         │
                    └─────────────────────────────────┘
                    ┌─────────────────────────────────┐
  运维 / 文档 ──────►│  :8081  内部端口                 │
                    │  ├─ /openapi  Redoc             │
                    │  └─ /debug    pprof             │
                    └─────────────────────────────────┘
```

## 最小示例

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

完整示例：

```bash
go run ./_example
```

`_example` 演示 gRPC 注册，以及通过 `mix/gateway` 将 RPC 暴露为 HTTP。

## 配置选项

| Option | 说明 |
|--------|------|
| `WithHttpHandler` | 主 HTTP 处理器（必填） |
| `WithGrpcHandler` | 注册 gRPC 服务 |
| `WithHttp` | 自定义 `http.Server`（地址、超时等） |
| `WithHTTP3` | 启用 HTTP/3 |
| `WithInternalServer` | 内部端口（OpenAPI / pprof） |
| `WithCors` | 跨域 |
| `WithOtel` | OpenTelemetry |
| `WithMiddleware` | HTTP 中间件链 |
| `WithGrpc` | gRPC 拦截器与 `ServerOption` |

### 请求元数据

```go
md := mix.GetMetadata(ctx)
if md != nil {
	_ = md.TraceId
	_ = md.Token
	md.Set("key", "value")
}
```

### 与 initialize 一起用

```go
global.Conf.Server.WithOptions(
	mix.WithHttpHandler(app),
	mix.WithGrpcHandler(api.GrpcRegister),
).Run()
```

## Protobuf 工具链

mix 不负责代码生成。推荐：

```bash
# 安装 hopeio 插件集
go run $(go list -m -f {{.Dir}} github.com/hopeio/protobuf)/tools/install_tools.go

# 生成 Go + OpenAPI + Gateway + Validator
protogen go -d -e -w -v -i ./proto -o ./protobuf
```

| 标志 | 含义 |
|------|------|
| `-d` | OpenAPI |
| `-w` | HTTP Gateway（`framework=gin\|fiber\|nethttp`） |
| `-v` | 请求校验 |
| `-I` / `-p` | include / hopeio `_proto` |

Docker：

```bash
docker run --rm -v "$PWD:/work" jybl/protogen \
  protogen go -d -w -v -i /work/proto -o /work/gen
```

## 默认端口

| 端口 | 用途 |
|------|------|
| `:8080` | 主服务（HTTP + gRPC） |
| `:8081` | OpenAPI / pprof |

## hopeio 生态

| 仓库 | 说明 |
|------|------|
| [gox](https://github.com/hopeio/gox) | 日志、HTTP 工具、常量与调度 |
| [initialize](https://github.com/hopeio/initialize) | 配置与 DAO 注入 |
| [protobuf](https://github.com/hopeio/protobuf) | `protogen` 与公共 proto |
| [scaffold](https://github.com/hopeio/scaffold) | OTel / JWT 等业务脚手架（可选） |

## License

[MIT](LICENSE) · Copyright © hopeio
