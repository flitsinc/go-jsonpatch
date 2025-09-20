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
		{
			name: "replace key containing slash",
			initialDoc: map[string]any{
				"viewStates": map[string]any{
					"Initial Load / No Track Selected": map[string]any{"isLoading": true},
				},
			},
			ops: []map[string]interface{}{
				{"op": "replace", "path": "/viewStates/Initial Load ~1 No Track Selected/isLoading", "value": false},
			},
			expectedDoc: map[string]any{
				"viewStates": map[string]any{
					"Initial Load / No Track Selected": map[string]any{"isLoading": false},
				},
			},
		},
		{
			name: "replace key containing tilde",
			initialDoc: map[string]any{
				"config": map[string]any{
					"Feature~Flag": true,
				},
			},
			ops: []map[string]interface{}{
				{"op": "replace", "path": "/config/Feature~0Flag", "value": false},
			},
			expectedDoc: map[string]any{
				"config": map[string]any{
					"Feature~Flag": false,
				},
			},
		},

		// --- Error Cases ---
		{
			name:          "invalid pointer escape sequence",
			initialDoc:    map[string]any{"foo": map[string]any{"bar": 1}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/foo/~2bar", "value": 2}},
			expectedError: "invalid JSON pointer",
		},
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
			expectedError: "path segment \"c\" not found in map",
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
			expectedError: "target of \"str_ins\" at path \"/field\" is not a string",
		},
		{
			name:          "inc on non-number",
			initialDoc:    map[string]any{"field": "not a number"},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/field", "inc": 1}},
			expectedError: "target key \"field\" of \"inc\" at path \"/field\" is not a number",
		},
		{
			name:          "str_ins missing pos",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/text", "str": "hi"}},
			expectedError: "invalid \"str_ins\" op parameters",
		},
		{
			name:          "inc missing inc field",
			initialDoc:    map[string]any{"counter": 0},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/counter"}},
			expectedError: "op \"inc\" missing \"inc\" field",
		},
		{
			name:          "inc with non-numeric inc value",
			initialDoc:    map[string]any{"counter": 0},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/counter", "inc": "not-a-number"}},
			expectedError: "op \"inc\" \"inc\" field is not a recognized number",
		},
		{
			name:          "str_del pos out of bounds",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 5, "len": 1}},
			expectedError: "invalid \"pos\" 5 or \"len\" 1 for \"str_del\"",
		},
		{
			name:          "unsupported op on root",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "inc", "path": "", "inc": 1}},
			expectedError: "op \"inc\" on root path \"\" is not supported",
		},
		{
			name:          "unknown operation type",
			initialDoc:    map[string]any{"text": "abc"},
			ops:           []map[string]interface{}{{"op": "unknown_op", "path": "/text"}},
			expectedError: "unhandled op type \"unknown_op\"",
		},
		{
			name:          "replace path to map resolves to index", // Internal consistency check
			initialDoc:    map[string]any{"a": map[string]any{"b": "c"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/0", "value": "d"}}, // /a is a map, /0 implies index
			expectedError: "path segment \"0\" not found in map for path \"/a/0\"",                   // error occurs at path traversal because "0" is not a key in map "a"
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
			expectedError: "op \"replace\" on root path \"\" with value of type string; expected map[string]any",
		},
		{
			name:          "replace op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": "a_string", "b": map[string]any{"c": "d"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/x/y", "value": "new_val"}},
			expectedError: "path \"/a/x/y\" traverses a non-container (neither map nor slice) at segment \"x\" (value type: string)",
		},
		{
			name:          "str_ins op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": 123, "b": "hello"},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/a/x/y", "pos": 0, "str": "new"}},
			expectedError: "path \"/a/x/y\" traverses a non-container (neither map nor slice) at segment \"x\" (value type: int)",
		},
		{
			name:          "str_del op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": true, "b": "hello"},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/a/x/y", "pos": 0, "len": 1}},
			expectedError: "path \"/a/x/y\" traverses a non-container (neither map nor slice) at segment \"x\" (value type: bool)",
		},
		{
			name:          "inc op path traverses non-container before final segment",
			initialDoc:    map[string]any{"a": []interface{}{1, 2}, "b": 50},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/a/x/y", "inc": 1}},
			expectedError: "path segment \"x\" is not a valid integer index for slice in path \"/a/x/y\"",
		},
		{
			name:          "replace op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": "a_string"}},
			ops:           []map[string]interface{}{{"op": "replace", "path": "/a/b/c", "value": "new_val"}},
			expectedError: "path \"/a/b/c\" traverses a non-container (neither map nor slice) before final segment; parent is type string",
		},
		{
			name:          "str_ins op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": 123}},
			ops:           []map[string]interface{}{{"op": "str_ins", "path": "/a/b/c", "pos": 0, "str": "new"}},
			expectedError: "path \"/a/b/c\" traverses a non-container (neither map nor slice) before final segment; parent is type int",
		},
		{
			name:          "str_del op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": true}},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/a/b/c", "pos": 0, "len": 1}},
			expectedError: "path \"/a/b/c\" traverses a non-container (neither map nor slice) before final segment; parent is type bool",
		},
		{
			name:          "inc op parent of final segment is non-container",
			initialDoc:    map[string]any{"a": map[string]any{"b": []interface{}{1, 2}}},
			ops:           []map[string]interface{}{{"op": "inc", "path": "/a/b/c", "inc": 1}},
			expectedError: "path segment \"c\" is not a valid integer index for slice in path \"/a/b/c\"",
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
		{
			name:        "add new key to object",
			initialDoc:  map[string]any{"a": 1},
			ops:         []map[string]interface{}{{"op": "add", "path": "/b", "value": 2}},
			expectedDoc: map[string]any{"a": 1, "b": 2},
		},
		{
			name:        "add element to array with dash",
			initialDoc:  map[string]any{"arr": []interface{}{1, 2}},
			ops:         []map[string]interface{}{{"op": "add", "path": "/arr/-", "value": 3}},
			expectedDoc: map[string]any{"arr": []interface{}{1, 2, 3}},
		},
		{
			name:        "remove key from object",
			initialDoc:  map[string]any{"a": 1, "b": 2},
			ops:         []map[string]interface{}{{"op": "remove", "path": "/b"}},
			expectedDoc: map[string]any{"a": 1},
		},
		{
			name:        "remove element from array",
			initialDoc:  map[string]any{"arr": []interface{}{1, 2, 3}},
			ops:         []map[string]interface{}{{"op": "remove", "path": "/arr/1"}},
			expectedDoc: map[string]any{"arr": []interface{}{1, 3}},
		},
		{
			name:        "copy value",
			initialDoc:  map[string]any{"a": 1, "b": 2},
			ops:         []map[string]interface{}{{"op": "copy", "from": "/a", "path": "/c"}},
			expectedDoc: map[string]any{"a": 1, "b": 2, "c": 1},
		},
		{
			name:        "move element within array",
			initialDoc:  map[string]any{"arr": []interface{}{1, 2, 3}},
			ops:         []map[string]interface{}{{"op": "move", "from": "/arr/0", "path": "/arr/2"}},
			expectedDoc: map[string]any{"arr": []interface{}{2, 3, 1}},
		},
		{
			name:        "test success",
			initialDoc:  map[string]any{"a": map[string]any{"b": 1}},
			ops:         []map[string]interface{}{{"op": "test", "path": "/a/b", "value": 1}},
			expectedDoc: map[string]any{"a": map[string]any{"b": 1}},
		},
		{
			name:          "test failure",
			initialDoc:    map[string]any{"a": 1},
			ops:           []map[string]interface{}{{"op": "test", "path": "/a", "value": 2}},
			expectedError: "test operation failed",
		},
		{
			name:        "add element to middle of array",
			initialDoc:  map[string]any{"arr": []interface{}{1, 3}},
			ops:         []map[string]interface{}{{"op": "add", "path": "/arr/1", "value": 2}},
			expectedDoc: map[string]any{"arr": []interface{}{1, 2, 3}},
		},
		{
			name:       "copy nested object",
			initialDoc: map[string]any{"a": map[string]any{"b": 1}, "target": map[string]any{}},
			ops:        []map[string]interface{}{{"op": "copy", "from": "/a", "path": "/target/copied"}},
			expectedDoc: map[string]any{
				"a":      map[string]any{"b": 1},
				"target": map[string]any{"copied": map[string]any{"b": 1}},
			},
		},
		{
			name:        "copy array element",
			initialDoc:  map[string]any{"arr": []interface{}{1, 2, 3}},
			ops:         []map[string]interface{}{{"op": "copy", "from": "/arr/0", "path": "/arr/2"}},
			expectedDoc: map[string]any{"arr": []interface{}{1, 2, 1, 3}},
		},
		{
			name:          "move path prefix error",
			initialDoc:    map[string]any{"a": map[string]any{"b": 1}},
			ops:           []map[string]interface{}{{"op": "move", "from": "/a", "path": "/a/b"}},
			expectedError: "from path \"/a\" is a proper prefix",
		},
		{
			name:       "test object equality",
			initialDoc: map[string]any{"obj": map[string]any{"a": 1, "b": []interface{}{"x", "y"}}},
			ops: []map[string]interface{}{
				{"op": "test", "path": "/obj", "value": map[string]interface{}{"a": 1, "b": []interface{}{"x", "y"}}},
			},
			expectedDoc: map[string]any{"obj": map[string]any{"a": 1, "b": []interface{}{"x", "y"}}},
		},
		{
			name:        "str_del with str parameter",
			initialDoc:  map[string]any{"text": "Hello cruel world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 6, "str": "cruel "}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_del with str parameter on unicode text",
			initialDoc:  map[string]any{"text": "Hello 🌍 world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 6, "str": "🌍 "}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_del with str parameter empty string",
			initialDoc:  map[string]any{"text": "Hello world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 5, "str": ""}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_del with len parameter (existing functionality)",
			initialDoc:  map[string]any{"text": "Hello cruel world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 6, "len": 6}},
			expectedDoc: map[string]any{"text": "Hello world"},
		},
		{
			name:        "str_ins with utf16 offset after emoji",
			initialDoc:  map[string]any{"text": "Hello 🌍 world"},
			ops:         []map[string]interface{}{{"op": "str_ins", "path": "/text", "pos": 9, "str": "big "}},
			expectedDoc: map[string]any{"text": "Hello 🌍 big world"},
		},
		{
			name:        "str_del utf16 offset after emoji",
			initialDoc:  map[string]any{"text": "Hello 🌍 world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 8, "str": " world"}},
			expectedDoc: map[string]any{"text": "Hello 🌍"},
		},
		{
			name:          "str_del with neither str nor len",
			initialDoc:    map[string]any{"text": "Hello world"},
			ops:           []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 5}},
			expectedError: "str or len required",
		},
		{
			name:        "str_del with str taking precedence over len",
			initialDoc:  map[string]any{"text": "Hello cruel world"},
			ops:         []map[string]interface{}{{"op": "str_del", "path": "/text", "pos": 6, "str": "cruel", "len": 10}},
			expectedDoc: map[string]any{"text": "Hello  world"}, // "cruel" is 5 chars, not 10
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

func TestResolvePath(t *testing.T) {
	baseDoc := map[string]any{
		"settings": map[string]any{"theme": "dark"},
		"list":     []any{"zero", "one", "two"},
	}

	parent, finalKey, finalIndex, containerParent, containerParentKey, containerParentIndex, err := resolvePath(baseDoc, "/settings/theme")
	if err != nil {
		t.Fatalf("resolvePath returned error: %v", err)
	}
	parentMap, ok := parent.(map[string]any)
	if !ok {
		t.Fatalf("expected parent container to be map, got %T", parent)
	}
	if finalKey != "theme" {
		t.Fatalf("expected key 'theme', got %q", finalKey)
	}
	containerParentMap, ok := containerParent.(map[string]any)
	if !ok || containerParentKey != "settings" || containerParentIndex != -1 {
		t.Fatalf("unexpected container parent info: %v, %q, %d", containerParent, containerParentKey, containerParentIndex)
	}
	if _, exists := containerParentMap["settings"]; !exists {
		t.Fatalf("expected container parent to expose 'settings'")
	}
	if parentMap[finalKey] != "dark" {
		t.Fatalf("expected value 'dark', got %v", parentMap[finalKey])
	}

	parent, finalKey, finalIndex, containerParent, containerParentKey, containerParentIndex, err = resolvePath(baseDoc, "/list/1")
	if err != nil {
		t.Fatalf("resolvePath returned error: %v", err)
	}
	parentSlice, ok := parent.([]any)
	if !ok {
		t.Fatalf("expected slice parent, got %T", parent)
	}
	if finalIndex != 1 {
		t.Fatalf("expected index 1, got %d", finalIndex)
	}
	containerParentMap, ok = containerParent.(map[string]any)
	if !ok || containerParentKey != "list" || containerParentIndex != -1 {
		t.Fatalf("unexpected container parent info for list: %v, %q, %d", containerParent, containerParentKey, containerParentIndex)
	}
	if _, exists := containerParentMap["list"]; !exists {
		t.Fatalf("expected container parent to expose 'list'")
	}
	if parentSlice[finalIndex] != "one" {
		t.Fatalf("expected value 'one', got %v", parentSlice[finalIndex])
	}

	_, _, finalIndex, _, _, _, err = resolvePath(baseDoc, "/list/-")
	if err != nil {
		t.Fatalf("resolvePath returned error: %v", err)
	}
	if finalIndex != len(baseDoc["list"].([]any)) {
		t.Fatalf("expected index %d for '-', got %d", len(baseDoc["list"].([]any)), finalIndex)
	}
}

func TestResolvePathErrors(t *testing.T) {
	testCases := []struct {
		name    string
		doc     map[string]any
		path    string
		wantErr string
	}{
		{
			name:    "dash not final",
			doc:     map[string]any{"list": []any{1, 2}},
			path:    "/list/-/value",
			wantErr: "is not a valid integer index",
		},
		{
			name:    "non-container before last",
			doc:     map[string]any{"a": map[string]any{"b": 1}},
			path:    "/a/b/c",
			wantErr: "parent is type int",
		},
		{
			name:    "non-container at final",
			doc:     map[string]any{"a": "leaf"},
			path:    "/a/b",
			wantErr: "before final segment; parent is type string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, _, _, err := resolvePath(tc.doc, tc.path)
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestSliceHelpers(t *testing.T) {
	base := []any{0, 1, 2}
	withInsert := insertValueIntoSlice(base, 1, "x")
	if got := withInsert[1]; got != "x" {
		t.Fatalf("insertValueIntoSlice expected 'x' at index 1, got %v", got)
	}
	if len(withInsert) != 4 {
		t.Fatalf("expected length 4 after insert, got %d", len(withInsert))
	}

	trimmed, removed := removeValueFromSlice(withInsert, 2)
	if removed != 1 {
		t.Fatalf("expected removed value 1, got %v", removed)
	}
	if len(trimmed) != 3 {
		t.Fatalf("expected length 3 after remove, got %d", len(trimmed))
	}

	parentMap := map[string]any{"arr": []any{1, 2}}
	updatedSlice := []any{7, 8}
	if err := assignSliceToParent(parentMap, "arr", -1, updatedSlice, "test"); err != nil {
		t.Fatalf("assignSliceToParent on map returned error: %v", err)
	}
	if !reflect.DeepEqual(parentMap["arr"], updatedSlice) {
		t.Fatalf("assignSliceToParent did not update map: %v", parentMap["arr"])
	}

	parentSlice := []any{[]any{0, 1}}
	if err := assignSliceToParent(parentSlice, "", 0, []any{3, 4}, "test"); err != nil {
		t.Fatalf("assignSliceToParent on slice returned error: %v", err)
	}
	if got := parentSlice[0].([]any); !reflect.DeepEqual(got, []any{3, 4}) {
		t.Fatalf("assignSliceToParent did not update slice parent: %v", got)
	}

	if err := assignSliceToParent(nil, "", 0, []any{}, "test"); err == nil {
		t.Fatalf("expected error when parent is invalid")
	}
}

func TestJSONEqual(t *testing.T) {
	testCases := []struct {
		name  string
		a     any
		b     any
		equal bool
	}{
		{name: "numeric equality", a: 1, b: float64(1), equal: true},
		{name: "bool equality", a: true, b: true, equal: true},
		{name: "nil equality", a: nil, b: nil, equal: true},
		{name: "map mismatch", a: map[string]any{"a": 1}, b: map[string]any{"a": 2}, equal: false},
		{name: "slice equality interface", a: []any{1, "two"}, b: []interface{}{1, "two"}, equal: true},
		{name: "slice mismatch", a: []any{1, 2}, b: []any{1, 3}, equal: false},
		{name: "struct equality", a: struct{ X int }{1}, b: struct{ X int }{1}, equal: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonEqual(tc.a, tc.b); got != tc.equal {
				t.Fatalf("jsonEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.equal)
			}
		})
	}
}

func TestUTF16LenToRuneLen(t *testing.T) {
	if got := utf16LenToRuneLen("hello", 0, 0); got != 0 {
		t.Fatalf("expected zero length when len is zero, got %d", got)
	}
	if got := utf16LenToRuneLen("a🌍b", 1, 2); got != 1 {
		t.Fatalf("expected rune length 1, got %d", got)
	}
}
