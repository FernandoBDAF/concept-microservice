package tracing

import (
	"context"
	"testing"
)

// With an OTLP endpoint configured, Init must build the SDK tracer provider.
// Regression: resource.Merge(resource.Default(), <resource pinned to an older
// semconv schema>) returns ErrSchemaURLConflict — it crashed every Go service
// at startup while `make verify` stayed green, because only the no-endpoint
// path was ever exercised in-process.
func TestInitWithEndpointBuildsProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")

	shutdown, err := Init(context.Background(), "tracing-test")
	if err != nil {
		t.Fatalf("Init with endpoint set: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestInitWithoutEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Init(context.Background(), "tracing-test")
	if err != nil {
		t.Fatalf("Init without endpoint: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown: %v", err)
	}
}
