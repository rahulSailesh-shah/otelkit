package cli

import "testing"

func TestRunCmdFlags(t *testing.T) {
	cmd := newRunCmd()

	wantFlags := map[string]string{
		"service":   "",
		"child-log": "",
		"config":    "",
	}
	for name, def := range wantFlags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s missing", name)
			continue
		}
		if f.DefValue != def {
			t.Errorf("flag --%s default = %q, want %q", name, f.DefValue, def)
		}
	}

	for _, name := range []string{
		"grpc-addr",
		"jaeger-addr",
		"prometheus-addr",
		"loki-addr",
		"signoz-addr",
		"tui",
		"refresh",
		"profile",
	} {
		if f := cmd.Flags().Lookup(name); f != nil {
			t.Errorf("flag --%s should not exist", name)
		}
	}
}

func TestRunCmdRequiresChildArgs(t *testing.T) {
	cmd := newRunCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no child command provided")
	}
}
