package obs_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/paper-board/sdk/obs"
)

func TestSetupOTelNoEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := obs.SetupOTel("test-svc", "v0.0.1")
	if err != nil {
		t.Fatalf("SetupOTel: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Shutdown nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if otel.GetTextMapPropagator() == nil {
		t.Fatal("propagator not set")
	}
}
