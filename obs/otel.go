// Package obs provides OpenTelemetry SDK setup, propagation, and shutdown
// for paper-board services.
//
// SetupOTel initialises a TracerProvider + MeterProvider via OTLP/HTTP
// exporters (endpoint from OTEL_EXPORTER_OTLP_ENDPOINT). W3C TraceContext +
// Baggage propagators are registered globally. Resource attributes attach
// service.name / service.version / service.namespace=paper-board.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset, SetupOTel installs only the W3C
// propagator and returns a no-op Shutdown — services can boot without an
// OTel collector available.
package obs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown is returned by SetupOTel; callers MUST defer it.
type Shutdown func(context.Context) error

// SetupOTel initialises the global tracer + meter providers and propagators.
// Endpoint format: host:port (no scheme). Set OTEL_EXPORTER_OTLP_INSECURE=true
// for plain HTTP (dev clusters); leave unset for TLS.
func SetupOTel(serviceName, version string) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := sdkresource.New(context.Background(),
		sdkresource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
			semconv.ServiceNamespace("paper-board"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	insecure := os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"

	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if insecure {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
	}
	traceExp, err := otlptracehttp.New(context.Background(), traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}

	metricOpts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(endpoint)}
	if insecure {
		metricOpts = append(metricOpts, otlpmetrichttp.WithInsecure())
	}
	metricExp, err := otlpmetrichttp.New(context.Background(), metricOpts...)
	if err != nil {
		_ = traceExp.Shutdown(context.Background())
		return nil, fmt.Errorf("otlp metric exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp, sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(res),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var errs []error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("meter shutdown: %w", err))
		}
		return errors.Join(errs...)
	}, nil
}
