package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/flitsinc/go-jsonpatch/jsonpatch"
)

// TestCase represents a single test case from JS
type TestCase struct {
	OriginalDoc map[string]any `json:"originalDoc"`
	ExpectedDoc map[string]any `json:"expectedDoc"`
	Operations  []map[string]any `json:"operations"`
	TestID      string `json:"testId"`
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

// convertJSOffsetsToRunes converts JavaScript UTF-16 offsets to Go rune offsets
func convertJSOffsetsToRunes(text string, jsOffset int) int {
	if jsOffset <= 0 {
		return 0
	}
	
	// Convert to UTF-16 to match JavaScript's indexing
	utf16 := []uint16{}
	for _, r := range text {
		if r <= 0xFFFF {
			utf16 = append(utf16, uint16(r))
		} else {
			// Surrogate pair for code points > 0xFFFF
			r -= 0x10000
			utf16 = append(utf16, uint16((r>>10)+0xD800))  // High surrogate
			utf16 = append(utf16, uint16((r&0x3FF)+0xDC00)) // Low surrogate
		}
	}
	
	if jsOffset >= len(utf16) {
		return len([]rune(text))
	}
	
	// Convert back to find rune position
	runeCount := 0
	utf16Pos := 0
	
	for _, r := range text {
		if utf16Pos >= jsOffset {
			break
		}
		
		if r <= 0xFFFF {
			utf16Pos++
		} else {
			utf16Pos += 2 // Surrogate pair
		}
		runeCount++
	}
	
	return runeCount
}

// convertStringOperations modifies string operations to handle JS-Go offset differences
func convertStringOperations(operations []map[string]any, originalDoc map[string]any) error {
	for _, op := range operations {
		opType, ok := op["op"].(string)
		if !ok {
			continue
		}
		
		if opType == "str_ins" || opType == "str_del" {
			pathRaw, ok := op["path"].(string)
			if !ok {
				continue
			}
			
			// Get current string value at path
			currentString, err := getStringAtPath(originalDoc, pathRaw)
			if err != nil {
				continue // Skip if we can't get the string
			}
			
			// Convert JS offset to rune offset
			if posVal, ok := op["pos"]; ok {
				if posFloat, ok := posVal.(float64); ok {
					jsOffset := int(posFloat)
					runeOffset := convertJSOffsetsToRunes(currentString, jsOffset)
					op["pos"] = runeOffset
				}
			}
		}
	}
	return nil
}

// getStringAtPath retrieves a string value at the given JSON pointer path
func getStringAtPath(doc map[string]any, pathRaw string) (string, error) {
	if pathRaw == "" {
		return "", fmt.Errorf("empty path")
	}
	
	pathSegments := strings.Split(strings.TrimPrefix(pathRaw, "/"), "/")
	current := any(doc)
	
	for _, segment := range pathSegments {
		if currentMap, ok := current.(map[string]any); ok {
			val, exists := currentMap[segment]
			if !exists {
				return "", fmt.Errorf("path segment not found")
			}
			current = val
		} else if currentSlice, ok := current.([]interface{}); ok {
			idx := 0
			if segment != "-" {
				var err error
				idxInt64, err := json.Number(segment).Int64()
				if err != nil {
					return "", fmt.Errorf("invalid array index")
				}
				idx = int(idxInt64)
			}
			if idx < 0 || int(idx) >= len(currentSlice) {
				return "", fmt.Errorf("array index out of bounds")
			}
			current = currentSlice[idx]
		} else {
			return "", fmt.Errorf("path traverses non-container")
		}
	}
	
	if str, ok := current.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("value at path is not a string")
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
		
		// Convert JavaScript string offsets to Go rune offsets
		if err := convertStringOperations(testCase.Operations, docCopy); err != nil {
			result := TestResult{
				TestID:  testCase.TestID,
				Success: false,
				Error:   fmt.Sprintf("Failed to convert string operations: %v", err),
			}
			encoder.Encode(result)
			continue
		}
		
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