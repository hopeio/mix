/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/hopeio/gox/crypto/tls"
	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/quic-go/quic-go/http3"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Http3Config struct {
	Enabled bool
	http3.Server
	CertFile string
	KeyFile  string
}

type AccessLogConfig struct {
	RecordFunc      AccessLog
	ExcludePrefixes []string
	IncludePrefixes []string
}

type Server struct {
	http.Server
	CertFile       string
	KeyFile        string
	AccessLog      AccessLogConfig
	HTTP3          Http3Config
	Cors           CorsConfig
	Grpc           GrpcConfig
	InternalServer http.Server
	Openapi        OpenapiConfig
	Otel           OtelConfig
	tracer         trace.Tracer
	meter          metric.Meter
	Debug          DebugConfig
	BaseContext    context.Context
	Middlewares    []httpx.Middleware
	HttpHandler    http.Handler
	GrpcHandler    func(*grpc.Server)
	// DisableInternalServer 关闭内部端口（健康检查/OpenAPI/调试端点）
	DisableInternalServer bool
}

type DebugConfig struct {
	Enabled   bool
	UriPrefix string
}

type OpenapiConfig struct {
	Enabled        bool
	UriPrefix, Dir string
}

type GrpcConfig struct {
	Addr                     string
	RecordFunc               GrpcAccessLog
	Options                  []grpc.ServerOption
	UnaryServerInterceptors  []grpc.UnaryServerInterceptor
	StreamServerInterceptors []grpc.StreamServerInterceptor
}

type CorsConfig struct {
	Enabled bool
	cors.Options
}

type OtelConfig struct {
	Enabled bool

	// ServiceName sets resource service.name; empty → OTEL_SERVICE_NAME or "mix".
	ServiceName string
	// ServiceVersion sets resource service.version; empty → OTEL_SERVICE_VERSION.
	ServiceVersion string
	// ResourceAttributes merges into the resource (in addition to env detectors).
	ResourceAttributes map[string]string

	// Protocol selects OTLP transport: "http" (default) or "grpc".
	// Empty → OTEL_EXPORTER_OTLP_PROTOCOL (http/protobuf|http → http, grpc → grpc).
	Protocol string
	// Endpoint is host:port or URL. Empty → OTEL_EXPORTER_OTLP_ENDPOINT (exporter default).
	Endpoint string
	// Insecure uses plaintext to the collector (typical for local/dev).
	Insecure bool
	// Headers are sent on OTLP export (merged with OTEL_EXPORTER_OTLP_HEADERS when empty).
	Headers map[string]string
	// URLPath overrides OTLP HTTP path (traces/metrics/logs use signal defaults if empty).
	TracesURLPath  string
	MetricsURLPath string
	LogsURLPath    string

	// SampleRatio is root/local sampling in [0,1]; 0 with no env → 1 (always).
	SampleRatio float64
	// MetricInterval for periodic metric export; 0 → 10s.
	MetricInterval time.Duration

	DisableTraces  bool
	DisableMetrics bool
	DisableLogs    bool
	// DisableRuntimeMetrics skips go.opentelemetry.io/contrib/instrumentation/runtime.
	DisableRuntimeMetrics bool

	// Pyroscope：仅 Enabled=true 时启动；地址可从 ServerAddress 或 PYROSCOPE_SERVER_ADDRESS 补。
	Pyroscope PyroscopeConfig

	// InternalAuth marks a trusted internal call: the inbound request must
	// carry InternalAuthHeader whose value equals the live secret (see
	// InternalAuthSecret / InternalAuthSecretFn). Presence-only checks are
	// forgeable. Empty secret = trust nothing (safe default).
	InternalAuthHeader string
	InternalAuthSecret string
	// InternalAuthSecretFn, when set, is preferred over InternalAuthSecret so
	// hot-reloaded config stays in sync with business IsInternalCall checks.
	InternalAuthSecretFn func() string

	OtelhttpOpts []otelhttp.Option
	OtelgrpcOpts []otelgrpc.Option
}

// internalAuthMatch is a constant-time compare to avoid timing side channels.
func internalAuthMatch(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (c *OtelConfig) internalAuthSecret() string {
	if c == nil {
		return ""
	}
	if c.InternalAuthSecretFn != nil {
		return strings.TrimSpace(c.InternalAuthSecretFn())
	}
	return strings.TrimSpace(c.InternalAuthSecret)
}

// IsInternalCall reports whether ctx carries the configured internal auth
// header with the matching secret. Always false when the secret is unset.
func (c *OtelConfig) IsInternalCall(ctx context.Context) bool {
	if c == nil || c.InternalAuthHeader == "" {
		return false
	}
	secret := c.internalAuthSecret()
	if secret == "" {
		return false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	vals := md.Get(c.InternalAuthHeader)
	if len(vals) == 0 {
		return false
	}
	return internalAuthMatch(strings.TrimSpace(vals[0]), secret)
}

// PyroscopeConfig configures grafana/pyroscope-go push client.
type PyroscopeConfig struct {
	Enabled             bool
	ServerAddress       string
	ApplicationName     string
	BasicAuthUser       string
	BasicAuthPassword   string
	AuthToken           string
	DisableMutexProfile bool
	DisableBlockProfile bool
	// Log enables pyroscope.StandardLogger (or set PYROSCOPE_LOG=1).
	Log bool
}

func (c *OtelConfig) SetOtelhttpOptions(otelhttpOpts []otelhttp.Option) {
	c.OtelhttpOpts = otelhttpOpts
}

func (c *OtelConfig) SetOtelgrpcOptions(otelgrpcOpts []otelgrpc.Option) {
	c.OtelgrpcOpts = otelgrpcOpts
}

type PrometheusConfig struct {
	Enabled bool
	HttpURI string
	promhttp.HandlerOpts
}

func (s *Server) Init() {
	if s.BaseContext == nil {
		s.BaseContext = context.Background()
	}
	if s.Addr == "" {
		s.Addr = ":8080"
	}

	if s.AccessLog.RecordFunc == nil {
		s.AccessLog.RecordFunc = DefaultAccessLog
	}
	if s.Grpc.RecordFunc == nil {
		s.Grpc.RecordFunc = DefaultGrpcAccessLog
	}

	if s.InternalServer.Addr == "" {
		s.InternalServer.Addr = ":8081"
	}

	log.ValueLevelNotify("ReadTimeout", s.ReadTimeout, time.Second)
	log.ValueLevelNotify("WriteTimeout", s.WriteTimeout, time.Second)
	// 头部读取超时兜底，防 slowloris 占连接；只限请求头，不影响大文件上传
	if s.ReadHeaderTimeout == 0 && s.ReadTimeout == 0 {
		s.ReadHeaderTimeout = 10 * time.Second
	}
	if s.CertFile != "" && s.KeyFile != "" {
		tlsConfig, err := tls.NewServerTLSConfig(s.CertFile, s.KeyFile)
		if err != nil {
			log.Fatal(err)
		}
		s.TLSConfig = tlsConfig
	}
	if s.HTTP3.Enabled {
		if s.HTTP3.Addr == "" {
			s.HTTP3.Addr = ":8080"
		}
		if s.HTTP3.CertFile != "" && s.HTTP3.KeyFile != "" {
			tlsConfig, err := tls.NewServerTLSConfig(s.HTTP3.CertFile, s.HTTP3.KeyFile)
			if err != nil {
				log.Fatal(err)
			}
			s.HTTP3.TLSConfig = tlsConfig
		}
	}
	if s.Cors.Enabled {
		if len(s.Cors.AllowedOrigins) == 0 {
			s.Cors.AllowedOrigins = []string{"*"}
		}
		if len(s.Cors.AllowedMethods) == 0 {
			s.Cors.AllowedMethods = []string{http.MethodHead,
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete}
		}
		if len(s.Cors.AllowedHeaders) == 0 {
			s.Cors.AllowedHeaders = []string{"*"}
		}
		if len(s.Cors.ExposedHeaders) == 0 {
			s.Cors.ExposedHeaders = []string{
				HeaderErrorCode,
			}
		}
	}

}

// implement initialize
func (s *Server) BeforeInject() {
	s.Init()
}

func (s *Server) AfterInject() {
	s.Init()
}

func (s *Server) WithOptions(options ...Option) *Server {
	for _, option := range options {
		option(s)
	}
	return s
}
