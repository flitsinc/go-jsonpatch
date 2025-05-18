package jsonpatch

import "testing"

func BenchmarkApplyRealistic(b *testing.B) {
	patch := []map[string]any{
		{"op": "str_ins", "path": "/text", "pos": 3, "str": "def"},
		{"op": "inc", "path": "/counter", "inc": 5},
		{"op": "replace", "path": "/nested/value", "value": "updated"},
		{"op": "add", "path": "/arr/0", "value": 1},
		{"op": "remove", "path": "/arr/0"},
	}

	for i := 0; i < b.N; i++ {
		doc := map[string]any{
			"text":    "abc",
			"counter": 0,
			"nested":  map[string]any{"value": "original"},
			"arr":     []any{},
		}
		if err := Apply(doc, patch); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
