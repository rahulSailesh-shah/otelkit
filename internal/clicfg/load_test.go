package clicfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NoFile_Defaults(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Receiver.GRPCAddr != ":4317" {
		t.Errorf("grpc_addr = %q, want :4317", cfg.Receiver.GRPCAddr)
	}
	if !cfg.Storage.Enabled {
		t.Error("storage should default to enabled")
	}
	if cfg.Storage.Path != "./otelkit.db" {
		t.Errorf("path = %q", cfg.Storage.Path)
	}
	if cfg.TUI.Enabled {
		t.Error("tui should default to disabled")
	}
	if cfg.SourcePath() != "" {
		t.Errorf("sourcePath should be empty for defaults, got %q", cfg.SourcePath())
	}
}

func TestLoad_MinimalYAML(t *testing.T) {
	cfg, err := Load("testdata/minimal.yaml", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Enabled {
		t.Error("storage should be disabled per minimal.yaml")
	}
	if cfg.Receiver.GRPCAddr != ":4317" {
		t.Errorf("absent field should use default, got %q", cfg.Receiver.GRPCAddr)
	}
}

func TestLoad_FullYAML(t *testing.T) {
	cfg, err := Load("testdata/full.yaml", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Receiver.GRPCAddr != ":14317" {
		t.Errorf("grpc_addr = %q", cfg.Receiver.GRPCAddr)
	}
	if len(cfg.Fanout) != 1 || cfg.Fanout[0].Type != "jaeger" {
		t.Errorf("fanout unexpected: %+v", cfg.Fanout)
	}
	if cfg.TUI.Enabled {
		t.Error("tui should be disabled in full.yaml")
	}
	if cfg.SourcePath() == "" {
		t.Error("sourcePath should be set after loading a file")
	}
}

func TestLoad_Discovery(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	content := []byte("tui:\n  enabled: true\n")
	if err := os.WriteFile(filepath.Join(dir, "otelkit.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TUI.Enabled {
		t.Error("discovery should have loaded otelkit.yaml and enabled tui")
	}
}

func TestLoad_ExplicitMissing(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml", "")
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestLoad_ProfileApplied(t *testing.T) {
	t.Setenv("OTELKIT_PROFILE", "prod")
	cfg, err := Load("testdata/profiles.yaml", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Enabled {
		t.Error("prod profile should disable storage")
	}
	if cfg.TUI.Enabled {
		t.Error("prod profile should disable tui")
	}
	if cfg.ActiveProfile() != "prod" {
		t.Errorf("active profile = %q, want prod", cfg.ActiveProfile())
	}
}

func TestLoad_UnknownProfile(t *testing.T) {
	t.Setenv("OTELKIT_PROFILE", "does-not-exist")
	_, err := Load("testdata/profiles.yaml", "")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("OTELKIT_GRPC_ADDR", ":9999")
	cfg, err := Load("testdata/full.yaml", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Receiver.GRPCAddr != ":9999" {
		t.Errorf("env override failed: %q", cfg.Receiver.GRPCAddr)
	}
}

func TestLoad_UnknownKey(t *testing.T) {
	cfg, err := Load("testdata/unknown-key.yaml", "")
	if err != nil {
		t.Fatalf("unknown key should warn not error: %v", err)
	}
	if !cfg.Storage.Enabled {
		t.Errorf("known fields should still decode: %+v", cfg.Storage)
	}
}

func TestLoad_BadYAML(t *testing.T) {
	_, err := Load("testdata/bad-yaml.yaml", "")
	if err == nil {
		t.Fatal("expected parse error for bad YAML")
	}
}
