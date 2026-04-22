package tui

import "testing"

func TestParseAttrs(t *testing.T) {
	raw := `{"http.method":"GET","http.status_code":200,"nested":{"a":1}}`
	got := parseAttrs(&raw)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	wantVal := map[string]string{
		"http.method":      "GET",
		"http.status_code": "200",
	}
	for _, r := range got {
		if v, ok := wantVal[r.Key]; ok && r.Value != v {
			t.Errorf("key %q value = %q, want %q", r.Key, r.Value, v)
		}
	}
}

func TestParseAttrsEmpty(t *testing.T) {
	if got := parseAttrs(nil); got != nil {
		t.Errorf("nil raw should return nil, got %v", got)
	}
	s := ""
	if got := parseAttrs(&s); got != nil {
		t.Errorf("empty raw should return nil, got %v", got)
	}
	bad := "not-json"
	got := parseAttrs(&bad)
	if len(got) != 1 || got[0].Key != "_raw" || got[0].Value != "not-json" {
		t.Errorf("bad JSON fallback = %v", got)
	}
}
