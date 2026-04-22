package cli

import "testing"

func TestRootCmdWiring(t *testing.T) {
	cmd := newRootCmd()
	if cmd.Use != "otelkit" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "otelkit")
	}
	for _, name := range []string{"db"} {
		if f := cmd.PersistentFlags().Lookup(name); f == nil {
			t.Errorf("persistent flag --%s missing", name)
		}
	}
	wantSub := map[string]bool{"run": false, "dash": false, "version": false}
	for _, sub := range cmd.Commands() {
		if _, ok := wantSub[sub.Name()]; ok {
			wantSub[sub.Name()] = true
		}
	}
	for name, found := range wantSub {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}
