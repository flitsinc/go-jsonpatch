package jsonpatch

import (
	"reflect"
	"strings"
	"testing"
)

// Helper to make deep copies of docs for testing, as Apply mutates the doc.
func deepCopyDoc(doc map[string]any) map[string]any {
	if doc == nil {
		return nil
	}
	copy := make(map[string]any)
	for k, v := range doc {
		switch val := v.(type) {
		case map[string]any:
			copy[k] = deepCopyDoc(val)
		case []interface{}:
			copy[k] = deepCopySlice(val)
		default:
			copy[k] = v
		}
	}
	return copy
}

func deepCopySlice(slice []interface{}) []interface{} {
	if slice == nil {
		return nil
	}
	copy := make([]interface{}, len(slice))
	for i, v := range slice {
		switch val := v.(type) {
		case map[string]any:
			copy[i] = deepCopyDoc(val)
		case []interface{}:
			copy[i] = deepCopySlice(val)
		default:
			copy[i] = v
		}
	}
	return copy
}

func TestApply(t *testing.T) {
	testCases := []struct {
		name          string
		initialDoc    map[string]any
		ops           []map[string]interface{}
		expectedDoc   map[string]any
		expectedError string // Substring of the expected error message
	}{
		// --- Success Cases ---
		{
			name:        "replace top-level string",
			initialDoc:  map[string]any{"foo": "bar"},
			ops:         []map[string]interface{}{{"op": "replace", "path": "/foo", "value": "baz"}},
			expectedDoc: map[string]any{"foo": "baz"},
		},
		{
			name:        "replace nested number",
			initialDoc:  map[string]any{"a": map[string]any{"b": 10}},
			ops:         []map[string]interface{}{{"op": "replace", "path": "/a/b", "value": 20}},
			expectedDoc: map[string]any{"a": map[string]any{"b": 20}},
		},
		{
			name:        "replace element in array",
			initialDoc:  map[string]any{"arr": []interface{}{"one", "two", "three"}},
			ops:         []map[string]interface{}{{"op": "replace", "path": "/arr/1", "value": "deux"}},
			expectedDoc: map[string]any{"arr": []interface{}{"one", "deux", "three"}},
		},
		{
			name:        "str_ins at start",
			initialDoc:  map[string]any{"text": "world"},
			ops:         []map[string]interface{}{{"op": "str_ins", "path": "/text", "pos": 0, "str": "Hello "}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_ins in middle",
			initialDoc:  map[string]any{"text": "Helloworld"},
			ops:         []map[string]interface{}{{"op": "str_ins", "path": "/text", "pos": 5, "str": " "}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_ins at end",
			initialDoc:  map[string]any{"text": "Hello"},
			ops:         []map[string]interface{}{{"op": "str_ins", "path": "/text", "pos": 5, "str": " world"}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_del from start",
			initialDoc:  map[string]any{"text": "Goodbye Hello world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 0, "len": 8}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_del from middle",
			initialDoc:  map[string]any{"text": "Hello cruel world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 6, "len": 6}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "inc integer",
			initialDoc:  map[string]any{"counter": 5},
			ops:         []map[string]interface{}{{"op": "inc", "path": "/counter", "inc": 1}},
			expectedDoc: map[string]any{"counter": 6},
		},
		{
			name:        "inc float (stored as int)",
			initialDoc:  map[string]any{"value": 10.5},
			ops:         []map[string]interface{}{{"op": "inc", "path": "/value", "inc": 1.0}},
			expectedDoc: map[string]any{"value": 11}, // Result of int(10.5 + 1.0)
		},
		{
			name:        "inc on array element",
			initialDoc:  map[string]any{"numbers": []interface{}{10, 20, 30}},
			ops:         []map[string]interface{}{{"op": "inc", "path": "/numbers/1", "inc": 5}},
			expectedDoc: map[string]any{"numbers": []interface{}{10, 25, 30}},
		},
		{
			name:        "replace root",
			initialDoc:  map[string]any{"foo": "bar", "baz": "qux"},
			ops:         []map[string]interface{}{{"op": "replace", "path": "", "value": map[string]interface{}{"new": "doc"}}},
			expectedDoc: map[string]any{"new": "doc"},
		},
		{
			name:        "add root (same as replace)",
			initialDoc:  map[string]any{"foo": "bar"},
			ops:         []map[string]interface{}{{"op": "add", "path": "", "value": map[string]interface{}{"new": "doc"}}},
			expectedDoc: map[string]any{"new": "doc"},
		},
		{
			name:        "remove root",
			initialDoc:  map[string]any{"foo": "bar"},
			ops:         []map[string]interface{}{{"op": "remove", "path": ""}},
			expectedDoc: map[string]any{},
		},
		{
			name: "multiple operations",
			initialDoc: map[string]any{
				"text":    "abc",
				"counter": 0,
				"nested":  map[string]any{"value": "original"},
			},
			ops: []map[string]interface{}{
				{"op": "str_ins", "path": "/text", "pos": 3, "str": "def"},     // text: "abcdef"
				{"op": "inc", "path": "/counter", "inc": 5},                    // counter: 5
				{"op": "replace", "path": "/nested/value", "value": "updated"}, // nested.value: "updated"
			},
			expectedDoc: map[string]any{
				"text":    "abcdef",
				"counter": 5,
				"nested":  map[string]any{"value": "updated"},
			},
		},

		// --- Error Cases ---
		{
			name:          "invalid op: missing op field",
			initialDoc:    map[string]any{"foo": "bar"},
			ops:           []map[string]interface{}{{"path": "/foo", "value": "baz"}},
			expectedError: "invalid op format",
		},
		{
			name:          "invalid op: missing path field",
			initialDoc:    map[string]any{"foo": "bar"},
			ops:           []map[string]interface{}{{"op": "replace", "value": "baz"}},
			expectedError: "invalid op format",
		},
		{
			name:          "path segment not found in map",
			initialDoc:    map[string]any{"a": map[string]any{"b": 1}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/c", "value": 2}},
			expectedError: "path segment 'c' not found in map",
		},
		{
			name:          "array index out of bounds (replace)",
			initialDoc:    map[string]any{"arr": []interface{}{1, 2}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/arr/2", "value": 3}},
			expectedError: "index 2 out of bounds",
		},
		{
			name:          "traversing non-container",
			initialDoc:    map[string]any{"a": "not a map"},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/b", "value": 1}},
			expectedError: "traverses a non-container",
		},
		{
			name:          "str_ins on non-string",
			initialDoc:    map[string]any{"field": 123},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/field", "pos": 0, "str": "hi"}},
			expectedError: "target of 'str_ins' at path '/field' is not a string",
		},
		{
			name:          "inc on non-number",
			initialDoc:    map[string]any{"field": "not a number"},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/field", "inc": 1}},
			expectedError: "target key 'field' of 'inc' at path '/field' is not a number",
		},
		{
			name:          "str_ins missing pos",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/text", "str": "hi"}},
			expectedError: "invalid 'str_ins' op parameters",
		},
		{
			name:          "inc missing inc field",
			initialDoc:    map[string]any{"counter": 0},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/counter"}},
			expectedError: "op 'inc' missing 'inc' field",
		},
		{
			name:          "inc with non-numeric inc value",
			initialDoc:    map[string]any{"counter": 0},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/counter", "inc": "not-a-number"}},
			expectedError: "op 'inc' 'inc' field is not a recognized number",
		},
		{
			name:          "str_del pos out of bounds",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 5, "len": 1}},
			expectedError: "invalid 'pos' 5 or 'len' 1 for 'str_del'",
		},
		{
			name:          "unsupported op on root",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "inc", "path": "", "inc": 1}},
			expectedError: "op 'inc' on root path \"\" is not supported",
		},
		{
			name:          "unknown operation type",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "unknown_op", "path": "/text"}},
			expectedError: "unhandled op type 'unknown_op'",
		},
		{
			name:          "replace path to map resolves to index", // Internal consistency check
			initialDoc:    map[string]any{"a": map[string]any{"b": "c"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/0", "value": "d"}}, // /a is a map, /0 implies index
			expectedError: "path segment '0' not found in map for path '/a/0'",                       // error occurs at path traversal because "0" is not a key in map "a"
		},
		{
			name:        "str_ins on array element",
			initialDoc:  map[string]any{"arr": []interface{}{"hello", "world"}},
			ops:         []map[string]interface{}{{"op": "str_ins", "path": "/arr/0", "pos": 5, "str": "_suffix"}},
			expectedDoc: map[string]any{"arr": []interface{}{"hello_suffix", "world"}},
		},
		{
			name:        "str_del on array element",
			initialDoc:  map[string]any{"arr": []interface{}{"prefix_text", "world"}},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/arr/0", "pos": 0, "len": 7}},
			expectedDoc: map[string]any{"arr": []interface{}{"text", "world"}},
		},
		// --- New tests for increased coverage and complex scenarios ---
		{
			name:        "inc with int32 value in doc",
			initialDoc:  map[string]any{"counter": int32(10)},
			ops:         []map[string]interface{}{{"op": "inc", "path": "/counter", "inc": int32(5)}},
			expectedDoc: map[string]any{"counter": 15}, // Stored as int after operation
		},
		{
			name:        "inc with int64 value in doc",
			initialDoc:  map[string]any{"counter": int64(100)},
			ops:         []map[string]interface{}{{"op": "inc", "path": "/counter", "inc": int64(50)}},
			expectedDoc: map[string]any{"counter": 150}, // Stored as int after operation
		},
		{
			name:          "replace root with non-map value",
			initialDoc:    map[string]any{"foo": "bar"},
			ops:           []map[string]interface{}{{"op": "replace", "path": "", "value": "not a map"}},
			expectedError: "op 'replace' on root path \"\" with value of type string; expected map[string]any",
		},
		{
			name:          "replace op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": "a_string", "b": map[string]any{"c": "d"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/x/y", "value": "new_val"}},
			expectedError: "path '/a/x/y' traverses a non-container (neither map nor slice) at segment 'x' (value type: string)",
		},
		{
			name:          "str_ins op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": 123, "b": "hello"},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/a/x/y", "pos": 0, "str": "new"}},
			expectedError: "path '/a/x/y' traverses a non-container (neither map nor slice) at segment 'x' (value type: int)",
		},
		{
			name:          "str_del op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": true, "b": "hello"},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/a/x/y", "pos": 0, "len": 1}},
			expectedError: "path '/a/x/y' traverses a non-container (neither map nor slice) at segment 'x' (value type: bool)",
		},
		{
			name:          "inc op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": []interface{}{1, 2}, "b": 50},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/a/x/y", "inc": 1}},
			expectedError: "path segment 'x' is not a valid integer index for slice in path '/a/x/y'",
		},
		{
			name:          "replace op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": "a_string"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/b/c", "value": "new_val"}},
			expectedError: "path '/a/b/c' traverses a non-container (neither map nor slice) before final segment; parent is type string",
		},
		{
			name:          "str_ins op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": 123}},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/a/b/c", "pos": 0, "str": "new"}},
			expectedError: "path '/a/b/c' traverses a non-container (neither map nor slice) before final segment; parent is type int",
		},
		{
			name:          "str_del op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": true}},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/a/b/c", "pos": 0, "len": 1}},
			expectedError: "path '/a/b/c' traverses a non-container (neither map nor slice) before final segment; parent is type bool",
		},
		{
			name:          "inc op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": []interface{}{1, 2}}},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/a/b/c", "inc": 1}},
			expectedError: "path segment 'c' is not a valid integer index for slice in path '/a/b/c'",
		},
		{
			name: "complex session: multiple ops, nested, arrays, different types",
			initialDoc: map[string]any{
				"user": map[string]any{
					"name": "Alice",
					"details": map[string]any{
						"age":    30,
						"city":   "Wonderland",
						"scores": []interface{}{10, 20, 30.5},
					},
				},
				"status": "active",
				"log":    "Initial entry.",
			},
			ops: []map[string]interface{}{
				{"op": "replace", "path": "/user/details/city", "value": "New York"},    // Replace string in nested map
				{"op": "inc", "path": "/user/details/age", "inc": 1},                    // Increment integer in nested map
				{"op": "inc", "path": "/user/details/scores/2", "inc": 9.5},             // Increment float (stored as int) in nested array (30.5 + 9.5 = 40)
				{"op": "str_ins", "path": "/log", "pos": 0, "str": "Update: "},          // Prepend to string
				{"op": "str_del", "path": "/log", "pos": 15, "len": 7},                  // Delete "entry." from "Update: Initial entry."
				{"op": "replace", "path": "/status", "value": "inactive"},               // Replace top-level string
				{"op": "str_ins", "path": "/user/name", "pos": 5, "str": " Wonderland"}, // Insert into string: Alice Wonderland
				{"op": "replace", "path": "/user/details/scores/0", "value": 100},       // Replace element in array
			},
			expectedDoc: map[string]any{
				"user": map[string]any{
					"name": "Alice Wonderland",
					"details": map[string]any{
						"age":    31,
						"city":   "New York",
						"scores": []interface{}{100, 20, 40}, // 30.5 + 9.5 becomes 40 (int)
					},
				},
				"status": "inactive",
				"log":    "Update: Initial",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy because Apply mutates the doc
			docToTest := deepCopyDoc(tc.initialDoc)
			err := Apply(docToTest, tc.ops)

			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil error", tc.expectedError)
				} else if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if !reflect.DeepEqual(docToTest, tc.expectedDoc) {
					t.Errorf("Documents not equal.\nInitial: %v\nOps: %v\nGot:     %v\nExpected: %v", tc.initialDoc, tc.ops, docToTest, tc.expectedDoc)
				}
			}
		})
	}
}
