package config

import (
	"reflect"
	"testing"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name    string
		base    map[string]any
		overlay map[string]any
		want    map[string]any
	}{
		{
			name:    "overlay scalar overwrites base",
			base:    map[string]any{"a": 1},
			overlay: map[string]any{"a": 2},
			want:    map[string]any{"a": 2},
		},
		{
			name:    "overlay adds new key",
			base:    map[string]any{"a": 1},
			overlay: map[string]any{"b": 2},
			want:    map[string]any{"a": 1, "b": 2},
		},
		{
			name: "nested map deep-merged",
			base: map[string]any{
				"svc": map[string]any{"name": "x", "env": "dev"},
			},
			overlay: map[string]any{
				"svc": map[string]any{"env": "prod"},
			},
			want: map[string]any{
				"svc": map[string]any{"name": "x", "env": "prod"},
			},
		},
		{
			name:    "list replaced wholesale",
			base:    map[string]any{"xs": []any{1, 2, 3}},
			overlay: map[string]any{"xs": []any{9}},
			want:    map[string]any{"xs": []any{9}},
		},
		{
			name:    "nil unsets key",
			base:    map[string]any{"a": 1, "b": 2},
			overlay: map[string]any{"a": nil},
			want:    map[string]any{"b": 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Merge(tc.base, tc.overlay)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Merge(%v, %v) = %v, want %v", tc.base, tc.overlay, got, tc.want)
			}
		})
	}

}
