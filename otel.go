/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"errors"

	_ "github.com/hopeio/gox/net/http"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

const ScopeName = "github.com/hopeio/mix"

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func setupOTelSDK(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	newPropagator()

	// 注意：otel.GetTracerProvider()/GetMeterProvider() 永远不返回 nil（无配置时返回 noop 实现），
	// 不能用 nil 判断是否已初始化；此处以 Otel.Enabled 为准，启用时由 mix 接管 pipeline
	tracerProvider, err := newTraceProvider()
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)

	meterProvider, err := newMeterProvider()
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)

	return
}

func newPropagator() {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

func newTraceProvider() (*trace.TracerProvider, error) {
	tracerProvider := trace.NewTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	return tracerProvider, nil
}

func newMeterProvider() (*metric.MeterProvider, error) {
	meterProvider := metric.NewMeterProvider()
	otel.SetMeterProvider(meterProvider)
	return meterProvider, nil
}
