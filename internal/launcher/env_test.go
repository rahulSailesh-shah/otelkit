package launcher_test

import (
	"slices"
	"testing"

	"github.com/rahulSailesh-shah/otelkit/internal/launcher"
)

func TestBuildEnv_alwaysInjectsEndpoint(t *testing.T) {
	env := launcher.BuildEnv(launcher.Config{Endpoint: "localhost:4317"})
	if !slices.Contains(env, "OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317") {
		t.Errorf("expected OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 in %v", env)
	}
}

func TestBuildEnv_injectsServiceNameWhenSet(t *testing.T) {
	env := launcher.BuildEnv(launcher.Config{Endpoint: "localhost:4317", ServiceName: "my-api"})
	if !slices.Contains(env, "OTEL_SERVICE_NAME=my-api") {
		t.Errorf("expected OTEL_SERVICE_NAME=my-api in %v", env)
	}
}

func TestBuildEnv_omitsServiceNameWhenEmpty(t *testing.T) {
	env := launcher.BuildEnv(launcher.Config{Endpoint: "localhost:4317", ServiceName: ""})
	for _, e := range env {
		if len(e) >= 17 && e[:17] == "OTEL_SERVICE_NAME" {
			t.Errorf("expected no OTEL_SERVICE_NAME entry, got %q", e)
		}
	}
}
