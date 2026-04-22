package cli

import (
	"bytes"
	"testing"
)

func TestVersionCmdPrintsVersion(t *testing.T) {
	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := out.String(); got != "otelkit dev\n" {
		t.Fatalf("output = %q, want %q", got, "otelkit dev\n")
	}
}
