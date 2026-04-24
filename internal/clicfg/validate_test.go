package clicfg

import (
	"strings"
	"testing"
)

func TestValidate_OKEmpty(t *testing.T) {
	cfg := Defaults()
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("defaults should validate: %v", err)
	}
	_ = warnings
}

func TestValidate_UnknownFanoutType(t *testing.T) {
	cfg := Defaults()
	cfg.Fanout = []FanoutEntry{{Type: "nope", Endpoint: "localhost:1234"}}
	_, err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error mentioning unknown type, got %v", err)
	}
}

func TestValidate_StorageEnabledEmptyPath(t *testing.T) {
	cfg := Defaults()
	cfg.Storage.Enabled = true
	cfg.Storage.Path = ""
	_, err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for storage enabled with empty path")
	}
}

func TestValidate_KnownTypes(t *testing.T) {
	cfg := Defaults()
	cfg.Fanout = []FanoutEntry{
		{Type: "jaeger", Endpoint: "localhost:14317"},
		{Type: "prometheus", Listen: ":9091"},
		{Type: "loki", Endpoint: "http://localhost:3100"},
		{Type: "signoz", Endpoint: "localhost:24317"},
		{Type: "otlp", Endpoint: "localhost:4317"},
	}
	_, err := cfg.Validate()
	if err != nil {
		t.Fatalf("all known types should validate: %v", err)
	}
}
