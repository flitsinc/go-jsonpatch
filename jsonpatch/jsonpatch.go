package jsonpatch

import (
	"fmt"
	"strconv"
	"strings"
)

// getNumericValue safely converts an any to float64 if it's a known numeric type.
func getNumericValue(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// Apply applies a slice of JSON Patch operations to a document represented as a map.
// The operations should conform to RFC 6902.
// Supported operations: "replace", "str_ins", "str_del", "inc".
// "add" and "remove" on the root are supported. Other ops like "test", "move", "copy" are not.
func Apply(doc map[string]any, operations []map[string]any) error {
	for _, op := range operations {
		opType, opTypeOk := op["op"].(string)
		pathRaw, pathRawOk := op["path"].(string)

		if !opTypeOk || !pathRawOk {
			return fmt.Errorf("invalid op format: op missing or not a string, or path missing or not a string: %+v", op)
		}

		// Handle operations on the root document itself.
		if pathRaw == "" {
			switch opType {
			case "replace", "add": // "add" on root is same as "replace" for a map document
				newValue, valExists := op["value"]
				if !valExists {
					return fmt.Errorf("op %q on root path \"\" requires a \"value\" field", opType)
				}
				newMapValue, newIsMap := newValue.(map[string]any)
				if !newIsMap {
					return fmt.Errorf("op %q on root path \"\" with value of type %T; expected map[string]any", opType, newValue)
				}
				// Clear existing doc and replace with new content
				for k := range doc {
					delete(doc, k)
				}
				for k, v := range newMapValue {
					doc[k] = v
				}
				continue // Next operation
			case "remove":
				// Removing the root means clearing the map.
				for k := range doc {
					delete(doc, k)
				}
				continue
			default:
				// Other ops like "inc", "str_ins", "str_del" are not meaningful for the root map itself.
				return fmt.Errorf("op %q on root path \"\" is not supported or not meaningful for a map document", opType)
			}
		}

		pathSegments := strings.Split(strings.TrimPrefix(pathRaw, "/"), "/")

		var parentContainer any = doc // container (map or slice) that contains the final segment
		var finalKey string           // final segment when parentContainer is a map
		var finalIndex int = -1       // final segment when parentContainer is a slice
		var finalSegment string       // raw text of the final segment

		// Traversal logic to find the parent container and the final key/index
		// traversalCurrent starts as the document itself.
		traversalCurrent := any(doc)

		for i, segment := range pathSegments {
			if i == len(pathSegments)-1 {
				// We reached the final segment; remember its raw value and record the parent container.
				parentContainer = traversalCurrent
				finalSegment = segment
				break
			}

			// Navigate to the next segment
			if currentMap, ok := traversalCurrent.(map[string]any); ok {
				val, exists := currentMap[segment]
				if !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", segment, pathRaw)
				}
				traversalCurrent = val
			} else if currentSlice, ok := traversalCurrent.([]any); ok {
				idx, err := strconv.Atoi(segment)
				if err != nil {
					return fmt.Errorf("path segment %q is not a valid integer index for slice in path %q", segment, pathRaw)
				}
				if idx < 0 || idx >= len(currentSlice) {
					return fmt.Errorf("index %d out of bounds for slice (len %d) at segment %q in path %q", idx, len(currentSlice), segment, pathRaw)
				}
				traversalCurrent = currentSlice[idx]
			} else {
				// Path traverses a non-container type (e.g., string, number) before reaching the final segment.
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) at segment %q (value type: %T)", pathRaw, segment, traversalCurrent)
			}
		}

		// Determine how to interpret the final segment depending on the parent container type.
		if _, ok := parentContainer.(map[string]any); ok {
			finalKey = finalSegment
		} else if _, ok := parentContainer.([]any); ok {
			idx, err := strconv.Atoi(finalSegment)
			if err != nil {
				return fmt.Errorf("path segment %q is not a valid integer index for slice in path %q", finalSegment, pathRaw)
			}
			finalIndex = idx
		} else {
			return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
		}

		// Apply the operation based on the type
		switch opType {
		case "replace":
			value, valueExists := op["value"]
			if !valueExists {
				return fmt.Errorf("op \"replace\" missing \"value\" field for path %q", pathRaw)
			}
			if targetMap, ok := parentContainer.(map[string]any); ok {
				// Replace requires that the key already exists according to RFC 6902.
				if _, exists := targetMap[finalKey]; !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", finalKey, pathRaw)
				}
				targetMap[finalKey] = value
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for \"replace\" op at path %q (slice len %d)", finalIndex, pathRaw, len(targetSlice))
				}
				targetSlice[finalIndex] = value
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}

		case "str_ins":
			posAny, posPresent := op["pos"]
			strToInsert, strOk := op["str"].(string)
			posFloat, posOk := getNumericValue(posAny)
			if !posPresent || !posOk || !strOk {
				return fmt.Errorf("invalid \"str_ins\" op parameters (pos/str missing or wrong type) for path %q", pathRaw)
			}
			pos := int(posFloat)

			var currentString string
			var getStringOk bool
			var valAtPathForError any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				if val, exists := targetMap[finalKey]; exists {
					currentString, getStringOk = val.(string)
					valAtPathForError = val
				} else { // Key must exist to insert into its string value
					return fmt.Errorf("target key %q for \"str_ins\" not found in map at path %q", finalKey, pathRaw)
				}
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex >= 0 && finalIndex < len(targetSlice) {
					currentString, getStringOk = targetSlice[finalIndex].(string)
					valAtPathForError = targetSlice[finalIndex]
				} else { // Index must be valid to get string for insertion
					return fmt.Errorf("index %d out of bounds for \"str_ins\" (getting string) at path %q", finalIndex, pathRaw)
				}
			} else {
				return fmt.Errorf("parent for \"str_ins\" op at path %q is not a map or slice (type %T)", pathRaw, parentContainer)
			}

			if !getStringOk { // Target was found but was not a string
				return fmt.Errorf("target of \"str_ins\" at path %q is not a string (actual type: %T, value: %+v)", pathRaw, valAtPathForError, valAtPathForError)
			}

			runes := []rune(currentString)
			if pos < 0 || pos > len(runes) { // pos can be equal to len(runes) for appending
				return fmt.Errorf("invalid \"pos\" %d for \"str_ins\" (string len %d) on path %q", pos, len(runes), pathRaw)
			}
			resultStr := string(runes[:pos]) + strToInsert + string(runes[pos:])

			// Update the value in the parent container
			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = resultStr
			} else if targetSlice, ok := parentContainer.([]any); ok {
				// Index already validated when getting string
				targetSlice[finalIndex] = resultStr
			}
			// No else needed here, parentContainer type already checked.

		case "str_del":
			posAny, posPresent := op["pos"]
			lenAny, lenPresent := op["len"]
			posFloat, posOk := getNumericValue(posAny)
			lenFloat, lenOk := getNumericValue(lenAny)
			if !posPresent || !lenPresent || !posOk || !lenOk {
				return fmt.Errorf("invalid \"str_del\" op parameters (pos/len missing or wrong type) for path %q", pathRaw)
			}
			pos := int(posFloat)
			length := int(lenFloat)

			var currentString string
			var getStringOk bool
			var valAtPathForError any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				if val, exists := targetMap[finalKey]; exists {
					currentString, getStringOk = val.(string)
					valAtPathForError = val
				} else {
					return fmt.Errorf("target key %q for \"str_del\" not found in map at path %q", finalKey, pathRaw)
				}
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex >= 0 && finalIndex < len(targetSlice) {
					currentString, getStringOk = targetSlice[finalIndex].(string)
					valAtPathForError = targetSlice[finalIndex]
				} else {
					return fmt.Errorf("index %d out of bounds for \"str_del\" (getting string) at path %q", finalIndex, pathRaw)
				}
			} else {
				return fmt.Errorf("parent for \"str_del\" op at path %q is not a map or slice (type %T)", pathRaw, parentContainer)
			}

			if !getStringOk {
				return fmt.Errorf("target of \"str_del\" at path %q is not a string (actual type: %T, value: %+v)", pathRaw, valAtPathForError, valAtPathForError)
			}
			runes := []rune(currentString)
			if pos < 0 || length < 0 || pos+length > len(runes) {
				return fmt.Errorf("invalid \"pos\" %d or \"len\" %d for \"str_del\" (string len %d) on path %q", pos, length, len(runes), pathRaw)
			}
			resultStr := string(runes[:pos]) + string(runes[pos+length:])

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = resultStr
			} else if targetSlice, ok := parentContainer.([]any); ok {
				targetSlice[finalIndex] = resultStr
			}

		case "inc":
			incValueFromOp, incFieldExists := op["inc"]
			if !incFieldExists {
				return fmt.Errorf("op \"inc\" missing \"inc\" field for path %q", pathRaw)
			}
			incOpValFloat, incOpValIsNumber := getNumericValue(incValueFromOp)
			if !incOpValIsNumber {
				return fmt.Errorf("op \"inc\" \"inc\" field is not a recognized number (got %T) for path %q", incValueFromOp, pathRaw)
			}

			var currentValueHolder any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				if finalKey == "" && finalIndex != -1 {
					return fmt.Errorf("internal error: path %q resolved to index %d for a map parent in \"inc\"", pathRaw, finalIndex)
				}
				val, exists := targetMap[finalKey]
				if !exists {
					return fmt.Errorf("target key %q for \"inc\" not found in map at path %q", finalKey, pathRaw)
				}
				currentValueHolder = val
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for \"inc\" at path %q (slice len %d)", finalIndex, pathRaw, len(targetSlice))
				}
				currentValueHolder = targetSlice[finalIndex]
			} else {
				return fmt.Errorf("parent container for \"inc\" at path %q is neither a map nor a slice (type %T)", pathRaw, parentContainer)
			}

			currentNumAsFloat, successfullyReadCurrentValue := getNumericValue(currentValueHolder)
			if !successfullyReadCurrentValue {
				var targetIdentifier string
				if finalKey != "" {
					targetIdentifier = fmt.Sprintf("key %q", finalKey)
				} else {
					targetIdentifier = fmt.Sprintf("index %d", finalIndex)
				}
				return fmt.Errorf("target %q of \"inc\" at path %q is not a number. Value: %+v, Type: %T", targetIdentifier, pathRaw, currentValueHolder, currentValueHolder)
			}

			incrementedResult := currentNumAsFloat + incOpValFloat
			// Store as int, assuming counters are integers. Could be float if document uses floats.
			finalValueToStore := int(incrementedResult)

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = finalValueToStore
			} else if targetSlice, ok := parentContainer.([]any); ok {
				targetSlice[finalIndex] = finalValueToStore
			}

		default:
			return fmt.Errorf("unhandled op type %q for path %q", opType, pathRaw)
		}
	}
	return nil
}
