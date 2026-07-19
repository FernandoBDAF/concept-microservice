// Package tracing bootstraps OpenTelemetry for this service (ADR-003.2).
//
// This package is intentionally duplicated in
// api-service/internal/pkg/tracing — the two Go modules share no library
// module, so each carries its own copy. Keep the two files in sync when
// changing either.
package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// Init configures the global OpenTelemetry tracer provider and propagators.
//
// Behavior is driven by the standard OTel env vars:
//   - OTEL_EXPORTER_OTLP_ENDPOINT: if empty, a no-op tracer provider is
//     installed (spans cost nothing, context still propagates) and the
//     returned shutdown is a no-op. If set, an OTLP/HTTP exporter is used;
//     an http:// scheme means plaintext, https:// means TLS.
//   - OTEL_SERVICE_NAME: overrides serviceName if set.
//
// The W3C TraceContext + Baggage propagators are registered globally in
// both modes so extraction/injection always works. Sampling is
// ParentBased(AlwaysSample).
//
// The returned shutdown func flushes and stops the exporter; call it on
// process exit.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Propagators are always registered, even when export is disabled:
	// inject/extract of traceparent must work regardless.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		serviceName = name
	}

	// Endpoint (and optional headers, timeouts, ...) come from the standard
	// OTEL_EXPORTER_OTLP_* env vars, which the exporter reads itself.
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// resource.Default() carries the SDK's own semconv schema URL; merging a
	// resource pinned to a different schema version returns
	// ErrSchemaURLConflict (this crashed every Go service at startup when the
	// SDK moved to schema 1.41.0 while this file pinned semconv/v1.26.0). A
	// schemaless resource adopts Default's schema on merge, and service.name
	// is stable across schema versions.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build OTel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
