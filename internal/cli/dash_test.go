package cli

import "testing"

func TestDashCmdFlags(t *testing.T) {
	cmd := newDashCmd()
	if cmd.Use != "dash" {
		t.Fatalf("Use = %q, want dash", cmd.Use)
	}
	if f := cmd.Flags().Lookup("refresh"); f == nil {
		t.Error("--refresh missing")
	} else if f.DefValue != "2s" {
		t.Errorf("--refresh default = %q, want 2s", f.DefValue)
	}
}
