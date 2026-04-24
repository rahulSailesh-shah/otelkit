package clicfg

import "testing"

func TestExpandEnv(t *testing.T) {
	t.Setenv("FOO", "bar")
	t.Setenv("EMPTY", "")

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no var", "plain string", "plain string"},
		{"simple var", "${FOO}", "bar"},
		{"inline var", "prefix-${FOO}-suffix", "prefix-bar-suffix"},
		{"missing no default", "${MISSING}", ""},
		{"missing with default", "${MISSING:-fallback}", "fallback"},
		{"present with default", "${FOO:-fallback}", "bar"},
		{"empty with default", "${EMPTY:-fallback}", "fallback"},
		{"multiple vars", "${FOO}-${MISSING:-x}", "bar-x"},
		{"escaped dollar", `\${FOO}`, "${FOO}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Expand(tc.in)
			if got != tc.want {
				t.Errorf("Expand(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandMap(t *testing.T) {
	t.Setenv("KEY", "value")
	in := map[string]string{
		"a": "${KEY}",
		"b": "literal",
	}
	got := ExpandMap(in)
	if got["a"] != "value" || got["b"] != "literal" {
		t.Errorf("ExpandMap(%v) = %v, want %v", in, got, in)
	}
}
