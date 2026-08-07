/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/hopeio/gox/log"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const ScopeName = "github.com/hopeio/mix"

const (
	otelProtocolHTTP = "http"
	otelProtocolGRPC = "grpc"
)

// setupOTelSDK bootstraps OTLP traces/metrics/logs when Enabled and no prior SDK.
// Returns a no-op shutdown when skipped (SkipSDK or non-noop tracer already set).
func setupOTelSDK(ctx context.Context, cfg *OtelConfig) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }
	if cfg == nil {
		return noop, nil
	}
	if cfg.SkipSDK && !cfg.ForceSDK {
		return noop, nil
	}
	if !cfg.ForceSDK && tracerProviderAlreadySet() {
		log.Info("otel: skip mix SDK (global tracer provider already set)")
		return noop, nil
	}

	wantTraces := !cfg.DisableTraces
	wantMetrics := !cfg.DisableMetrics
	wantLogs := !cfg.DisableLogs
	// Resolve pyroscope early (needs service name hint; full name after resource).
	pyroHint := cfg.Pyroscope.resolve(strings.TrimSpace(cfg.ServiceName))
	wantPyroscope := pyroHint.Enabled
	if !wantTraces && !wantMetrics && !wantLogs && !wantPyroscope {
		return noop, nil
	}

	var shutdownFuncs []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	installPropagator()

	res, err := newOTelResource(ctx, cfg)
	if err != nil {
		handleErr(err)
		return
	}

	proto := resolveOTelProtocol(cfg.Protocol)
	headers := resolveOTelHeaders(cfg.Headers)
	pyroCfg := cfg.Pyroscope.resolve(serviceNameFromResource(res))

	if wantTraces {
		tp, stopProf, e := newTraceProvider(ctx, res, cfg, proto, headers, pyroCfg)
		if e != nil {
			handleErr(e)
			return
		}
		shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
		if stopProf != nil {
			shutdownFuncs = append(shutdownFuncs, stopProf)
		}
	} else if pyroCfg.Enabled {
		profiler, e := startPyroscope(pyroCfg)
		if e != nil {
			handleErr(e)
			return
		}
		shutdownFuncs = append(shutdownFuncs, func(context.Context) error { return profiler.Stop() })
		log.Infof("otel: pyroscope profiling → %s app=%s (traces disabled)", pyroCfg.ServerAddress, pyroCfg.ApplicationName)
	}
	if wantMetrics {
		mp, e := newMeterProvider(ctx, res, cfg, proto, headers)
		if e != nil {
			handleErr(e)
			return
		}
		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	}
	if wantLogs {
		lp, e := newLoggerProvider(ctx, res, cfg, proto, headers)
		if e != nil {
			handleErr(e)
			return
		}
		shutdownFuncs = append(shutdownFuncs, lp.Shutdown)
	}

	log.Infof("otel: mix SDK ready protocol=%s endpoint=%q traces=%v metrics=%v logs=%v pyroscope=%v",
		proto, cfg.Endpoint, wantTraces, wantMetrics, wantLogs, pyroCfg.Enabled)
	return
}

func tracerProviderAlreadySet() bool {
	_, isNoop := otel.GetTracerProvider().(tracenoop.TracerProvider)
	return !isNoop
}

func installPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

func resolveOTelProtocol(protocol string) string {
	p := strings.ToLower(strings.TrimSpace(protocol))
	if p == "" {
		p = strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	}
	switch p {
	case otelProtocolGRPC, "grpc/protobuf":
		return otelProtocolGRPC
	case "", otelProtocolHTTP, "http/protobuf", "http/json":
		return otelProtocolHTTP
	default:
		return otelProtocolHTTP
	}
}

func resolveOTelHeaders(cfg map[string]string) map[string]string {
	if len(cfg) > 0 {
		out := make(map[string]string, len(cfg))
		for k, v := range cfg {
			out[k] = v
		}
		return out
	}
	raw := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	if raw == "" {
		return nil
	}
	out := make(map[string]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func newOTelResource(ctx context.Context, cfg *OtelConfig) (*resource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, 4+len(cfg.ResourceAttributes))
	name := strings.TrimSpace(cfg.ServiceName)
	if name == "" {
		name = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if name == "" {
		name = "mix"
	}
	attrs = append(attrs, semconv.ServiceName(name))
	ver := strings.TrimSpace(cfg.ServiceVersion)
	if ver == "" {
		ver = strings.TrimSpace(os.Getenv("OTEL_SERVICE_VERSION"))
	}
	if ver != "" {
		attrs = append(attrs, semconv.ServiceVersion(ver))
	}
	for k, v := range cfg.ResourceAttributes {
		if k == "" {
			continue
		}
		attrs = append(attrs, attribute.String(k, v))
	}
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithContainer(),
		resource.WithAttributes(attrs...),
	)
}

func sampleRatio(cfg *OtelConfig) float64 {
	if cfg.SampleRatio > 0 {
		if cfg.SampleRatio >= 1 {
			return 1
		}
		return cfg.SampleRatio
	}
	if v := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil && f > 0 {
			if f >= 1 {
				return 1
			}
			return f
		}
	}
	return 1
}

func traceSampler(cfg *OtelConfig) sdktrace.Sampler {
	r := sampleRatio(cfg)
	var root sdktrace.Sampler
	switch {
	case r >= 1:
		root = sdktrace.AlwaysSample()
	case r <= 0:
		root = sdktrace.NeverSample()
	default:
		root = sdktrace.TraceIDRatioBased(r)
	}
	return sdktrace.ParentBased(root, sdktrace.WithRemoteParentNotSampled(root))
}

func metricInterval(cfg *OtelConfig) time.Duration {
	if cfg.MetricInterval > 0 {
		return cfg.MetricInterval
	}
	return 10 * time.Second
}

func newTraceProvider(ctx context.Context, res *resource.Resource, cfg *OtelConfig, proto string, headers map[string]string, pyroCfg PyroscopeConfig) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	var exporter sdktrace.SpanExporter
	var err error
	switch proto {
	case otelProtocolGRPC:
		opts := make([]otlptracegrpc.Option, 0, 4)
		if ep := strings.TrimSpace(cfg.Endpoint); ep != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(stripURLScheme(ep)))
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(headers))
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	default:
		opts := make([]otlptracehttp.Option, 0, 6)
		opts = append(opts, otlpHTTPEndpointOpts(cfg.Endpoint)...)
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(headers))
		}
		if p := strings.TrimSpace(cfg.TracesURLPath); p != "" {
			opts = append(opts, otlptracehttp.WithURLPath(p))
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	}
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(traceSampler(cfg)),
	)
	if !pyroCfg.Enabled {
		otel.SetTracerProvider(tp)
		return tp, nil, nil
	}
	otel.SetTracerProvider(otelpyroscope.NewTracerProvider(tp))
	profiler, err := startPyroscope(pyroCfg)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, nil, err
	}
	log.Infof("otel: pyroscope profiling → %s app=%s", pyroCfg.ServerAddress, pyroCfg.ApplicationName)
	return tp, func(context.Context) error { return profiler.Stop() }, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, cfg *OtelConfig, proto string, headers map[string]string) (*sdkmetric.MeterProvider, error) {
	var reader sdkmetric.Reader
	var err error
	switch proto {
	case otelProtocolGRPC:
		opts := make([]otlpmetricgrpc.Option, 0, 4)
		if ep := strings.TrimSpace(cfg.Endpoint); ep != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(stripURLScheme(ep)))
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(headers))
		}
		var exp *otlpmetricgrpc.Exporter
		exp, err = otlpmetricgrpc.New(ctx, opts...)
		if err == nil {
			reader = sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(metricInterval(cfg)))
		}
	default:
		opts := make([]otlpmetrichttp.Option, 0, 6)
		opts = append(opts, otlpMetricHTTPEndpointOpts(cfg.Endpoint)...)
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(headers))
		}
		if p := strings.TrimSpace(cfg.MetricsURLPath); p != "" {
			opts = append(opts, otlpmetrichttp.WithURLPath(p))
		}
		var exp *otlpmetrichttp.Exporter
		exp, err = otlpmetrichttp.New(ctx, opts...)
		if err == nil {
			reader = sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(metricInterval(cfg)))
		}
	}
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(mp)
	if !cfg.DisableRuntimeMetrics {
		if err := runtime.Start(
			runtime.WithMeterProvider(mp),
			runtime.WithMinimumReadMemStatsInterval(15*time.Second),
		); err != nil {
			return nil, err
		}
	}
	return mp, nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource, cfg *OtelConfig, proto string, headers map[string]string) (*sdklog.LoggerProvider, error) {
	var exporter sdklog.Exporter
	var err error
	switch proto {
	case otelProtocolGRPC:
		opts := make([]otlploggrpc.Option, 0, 4)
		if ep := strings.TrimSpace(cfg.Endpoint); ep != "" {
			opts = append(opts, otlploggrpc.WithEndpoint(stripURLScheme(ep)))
		}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(headers))
		}
		exporter, err = otlploggrpc.New(ctx, opts...)
	default:
		opts := make([]otlploghttp.Option, 0, 6)
		opts = append(opts, otlpLogHTTPEndpointOpts(cfg.Endpoint)...)
		if cfg.Insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		if len(headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(headers))
		}
		if p := strings.TrimSpace(cfg.LogsURLPath); p != "" {
			opts = append(opts, otlploghttp.WithURLPath(p))
		}
		exporter, err = otlploghttp.New(ctx, opts...)
	}
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)
	return lp, nil
}

func stripURLScheme(endpoint string) string {
	ep := strings.TrimSpace(endpoint)
	if i := strings.Index(ep, "://"); i >= 0 {
		ep = ep[i+3:]
	}
	return strings.TrimSuffix(ep, "/")
}

func otlpHTTPEndpointOpts(endpoint string) []otlptracehttp.Option {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		return nil
	}
	if strings.Contains(ep, "://") {
		return []otlptracehttp.Option{otlptracehttp.WithEndpointURL(ep)}
	}
	return []otlptracehttp.Option{otlptracehttp.WithEndpoint(ep)}
}

func otlpMetricHTTPEndpointOpts(endpoint string) []otlpmetrichttp.Option {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		return nil
	}
	if strings.Contains(ep, "://") {
		return []otlpmetrichttp.Option{otlpmetrichttp.WithEndpointURL(ep)}
	}
	return []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(ep)}
}

func otlpLogHTTPEndpointOpts(endpoint string) []otlploghttp.Option {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		return nil
	}
	if strings.Contains(ep, "://") {
		return []otlploghttp.Option{otlploghttp.WithEndpointURL(ep)}
	}
	return []otlploghttp.Option{otlploghttp.WithEndpoint(ep)}
}
