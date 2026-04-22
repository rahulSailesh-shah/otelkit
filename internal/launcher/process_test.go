package launcher_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rahulSailesh-shah/otelkit/internal/launcher"
)

func TestRun_exitZeroOnSuccess(t *testing.T) {
	code, err := launcher.Run(context.Background(), []string{"echo", "hello"}, nil, launcher.IOConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRun_returnsNonZeroExitCode(t *testing.T) {
	// `false` is a POSIX command that always exits 1
	code, err := launcher.Run(context.Background(), []string{"false"}, nil, launcher.IOConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestRun_injectsEnvIntoChild(t *testing.T) {
	// sh -c 'test "$OTELKIT_TEST_VAR" = "hello"' exits 0 if var is set, 1 if not
	code, err := launcher.Run(
		context.Background(),
		[]string{"sh", "-c", `test "$OTELKIT_TEST_VAR" = "hello"`},
		[]string{"OTELKIT_TEST_VAR=hello"},
		launcher.IOConfig{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("env var not injected into child: exit code %d", code)
	}
}

func TestRun_errorsOnMissingCommand(t *testing.T) {
	_, err := launcher.Run(context.Background(), []string{"__nonexistent_cmd_xyz__"}, nil, launcher.IOConfig{})
	if err == nil {
		t.Error("expected error for missing command, got nil")
	}
}

func TestRun_redirectsStdout(t *testing.T) {
	var buf bytes.Buffer
	code, err := launcher.Run(
		context.Background(),
		[]string{"sh", "-c", `echo redirected`},
		nil,
		launcher.IOConfig{Stdout: &buf},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(buf.String(), "redirected") {
		t.Errorf("stdout buffer = %q, want to contain 'redirected'", buf.String())
	}
}
