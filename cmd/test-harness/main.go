package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/flitsinc/go-jsonpatch/jsonpatch"
)

// TestCase represents a single test case from JS
type TestCase struct {
	OriginalDoc map[string]any   `json:"originalDoc"`
	ExpectedDoc map[string]any   `json:"expectedDoc"`
	Operations  []map[string]any `json:"operations"`
	TestID      string           `json:"testId"`
}

// TestResult represents the result of applying operations
type TestResult struct {
	TestID    string         `json:"testId"`
	Success   bool           `json:"success"`
	ResultDoc map[string]any `json:"resultDoc,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// deepCopy creates a deep copy of a map[string]any
func deepCopy(original map[string]any) map[string]any {
	if original == nil {
		return nil
	}

	copy := make(map[string]any)
	for k, v := range original {
		switch val := v.(type) {
		case map[string]any:
			copy[k] = deepCopy(val)
		case []interface{}:
			copy[k] = deepCopySlice(val)
		default:
			copy[k] = v
		}
	}
	return copy
}

func deepCopySlice(original []interface{}) []interface{} {
	if original == nil {
		return nil
	}

	copy := make([]interface{}, len(original))
	for i, v := range original {
		switch val := v.(type) {
		case map[string]any:
			copy[i] = deepCopy(val)
		case []interface{}:
			copy[i] = deepCopySlice(val)
		default:
			copy[i] = v
		}
	}
	return copy
}

func main() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var testCase TestCase
		if err := decoder.Decode(&testCase); err != nil {
			if err.Error() == "EOF" {
				break
			}
			result := TestResult{
				TestID:  "unknown",
				Success: false,
				Error:   fmt.Sprintf("Failed to decode test case: %v", err),
			}
			encoder.Encode(result)
			continue
		}

		// Create a deep copy of the original document
		docCopy := deepCopy(testCase.OriginalDoc)

		// Apply the operations
		if err := jsonpatch.Apply(docCopy, testCase.Operations); err != nil {
			result := TestResult{
				TestID:  testCase.TestID,
				Success: false,
				Error:   fmt.Sprintf("Failed to apply operations: %v", err),
			}
			encoder.Encode(result)
			continue
		}

		// Return the result
		result := TestResult{
			TestID:    testCase.TestID,
			Success:   true,
			ResultDoc: docCopy,
		}
		encoder.Encode(result)
	}
}
