package mix

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestResolveOTelProtocol(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	assert.Equal(t, otelProtocolHTTP, resolveOTelProtocol(""))
	assert.Equal(t, otelProtocolHTTP, resolveOTelProtocol("http"))
	assert.Equal(t, otelProtocolHTTP, resolveOTelProtocol("http/protobuf"))
	assert.Equal(t, otelProtocolGRPC, resolveOTelProtocol("grpc"))
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	assert.Equal(t, otelProtocolGRPC, resolveOTelProtocol(""))
}

func TestResolveOTelHeaders(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "a=1,b=2")
	h := resolveOTelHeaders(nil)
	assert.Equal(t, "1", h["a"])
	assert.Equal(t, "2", h["b"])
	h2 := resolveOTelHeaders(map[string]string{"x": "y"})
	assert.Equal(t, "y", h2["x"])
	assert.NotContains(t, h2, "a")
}

func TestStripURLScheme(t *testing.T) {
	assert.Equal(t, "localhost:4317", stripURLScheme("http://localhost:4317"))
	assert.Equal(t, "localhost:4317", stripURLScheme("localhost:4317"))
}

func TestTracerProviderAlreadySet(t *testing.T) {
	prev := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	otel.SetTracerProvider(tracenoop.NewTracerProvider())
	assert.False(t, tracerProviderAlreadySet())

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })
	otel.SetTracerProvider(tp)
	assert.True(t, tracerProviderAlreadySet())
}

func TestSetupOTelSDKSkipWhenProviderSet(t *testing.T) {
	prev := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })
	otel.SetTracerProvider(tp)

	shutdown, err := setupOTelSDK(t.Context(), &OtelConfig{
		Enabled:        true,
		DisableMetrics: true,
		DisableLogs:    true,
		Insecure:       true,
		Endpoint:       "127.0.0.1:4318",
		Protocol:       "http",
	})
	require.NoError(t, err)
	require.NoError(t, shutdown(t.Context()))
	assert.Same(t, tp, otel.GetTracerProvider())
}

func TestSampleRatioDefault(t *testing.T) {
	_ = os.Unsetenv("OTEL_TRACES_SAMPLER_ARG")
	assert.Equal(t, 1.0, sampleRatio(&OtelConfig{}))
	assert.Equal(t, 0.5, sampleRatio(&OtelConfig{SampleRatio: 0.5}))
}
