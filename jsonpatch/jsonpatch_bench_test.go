package jsonpatch

import (
	"strings"
	"testing"
)

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []interface{}:
		return cloneSlice(v)
	default:
		return v
	}
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = cloneValue(v)
	}
	return dst
}

func cloneSlice(src []interface{}) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	for i, v := range src {
		dst[i] = cloneValue(v)
	}
	return dst
}

func benchmarkApply(b *testing.B, base map[string]any, ops []map[string]any) {
	b.Helper()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc := cloneMap(base)
		if err := Apply(doc, ops); err != nil {
			b.Fatalf("Apply returned error: %v", err)
		}
	}
}

func BenchmarkApplyReplaceNested(b *testing.B) {
	base := map[string]any{
		"viewStates": map[string]any{
			"Initial Load / No Track Selected": map[string]any{
				"isLoading": true,
				"count":     1,
			},
		},
		"config": map[string]any{
			"Feature~Flag": true,
		},
	}

	ops := []map[string]any{
		{"op": "replace", "path": "/viewStates/Initial Load ~1 No Track Selected/isLoading", "value": false},
		{"op": "inc", "path": "/viewStates/Initial Load ~1 No Track Selected/count", "inc": 3},
		{"op": "replace", "path": "/config/Feature~0Flag", "value": false},
	}

	benchmarkApply(b, base, ops)
}

func BenchmarkApplyArrayOps(b *testing.B) {
	const arraySize = 512
	values := make([]any, arraySize)
	for i := range values {
		values[i] = i
	}

	base := map[string]any{
		"arr": values,
	}

	ops := []map[string]any{
		{"op": "add", "path": "/arr/0", "value": -1},
		{"op": "add", "path": "/arr/256", "value": "mid"},
		{"op": "add", "path": "/arr/-", "value": arraySize},
		{"op": "remove", "path": "/arr/10"},
		{"op": "replace", "path": "/arr/5", "value": "five"},
	}

	benchmarkApply(b, base, ops)
}

func BenchmarkApplyStringOps(b *testing.B) {
	text := strings.Repeat("Hello 🌍 ", 16)
	base := map[string]any{
		"text": text,
	}

	insertPos := utf16Length("Hello 🌍")

	ops := []map[string]any{
		{"op": "str_ins", "path": "/text", "pos": insertPos, "str": "beautiful "},
		{"op": "str_del", "path": "/text", "pos": insertPos + 10, "len": 6},
		{"op": "str_ins", "path": "/text", "pos": 0, "str": "Start: "},
	}

	benchmarkApply(b, base, ops)
}

func BenchmarkApplyMixedOperations(b *testing.B) {
	base := map[string]any{
		"metadata": map[string]any{
			"version": 1,
			"tag":     "beta",
		},
		"matrix": []any{
			[]any{0, 1, 2},
			[]any{3, 4, 5},
		},
		"list": []any{"a", "b", "c", "d"},
	}

	ops := []map[string]any{
		{"op": "test", "path": "/metadata/tag", "value": "beta"},
		{"op": "copy", "from": "/metadata/tag", "path": "/metadata/previous"},
		{"op": "move", "from": "/list/0", "path": "/list/3"},
		{"op": "replace", "path": "/matrix/0/1", "value": 42},
		{"op": "add", "path": "/matrix/1/-", "value": 6},
	}

	benchmarkApply(b, base, ops)
}
