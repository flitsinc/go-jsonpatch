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

// decodePointerSegment unescapes "~0" and "~1" according to RFC 6901.
func decodePointerSegment(segment string) (string, error) {
	if strings.IndexByte(segment, '~') == -1 {
		return segment, nil
	}

	var builder strings.Builder
	builder.Grow(len(segment))
	for i := 0; i < len(segment); i++ {
		ch := segment[i]
		if ch != '~' {
			builder.WriteByte(ch)
			continue
		}
		if i+1 >= len(segment) {
			return "", fmt.Errorf("invalid escape sequence \"~\" at end of segment %q", segment)
		}
		switch segment[i+1] {
		case '0':
			builder.WriteByte('~')
		case '1':
			builder.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid escape sequence \"~%c\" in segment %q", segment[i+1], segment)
		}
		i++
	}

	return builder.String(), nil
}

// resolvePath walks doc using a JSON Pointer and returns the container that owns
// the final segment along with the leaf key/index plus its parent container info.
func resolvePath(doc map[string]any, pathRaw string) (parentContainer any, finalKey string, finalIndex int, containerParent any, containerParentKey string, containerParentIndex int, err error) {
	if pathRaw == "" {
		parentContainer = doc
		return
	}

	pathSegments := strings.Split(strings.TrimPrefix(pathRaw, "/"), "/")
	traversalCurrent := any(doc)
	var prevContainer any
	var prevKey string
	var prevIndex int
	last := len(pathSegments) - 1

	for i, rawSegment := range pathSegments {
		segment, decErr := decodePointerSegment(rawSegment)
		if decErr != nil {
			err = fmt.Errorf("invalid JSON pointer %q: %w", pathRaw, decErr)
			return
		}

		if i == last {
			containerParent = prevContainer
			containerParentKey = prevKey
			containerParentIndex = prevIndex
			parentContainer = traversalCurrent
			leaf := segment
			switch current := parentContainer.(type) {
			case map[string]any:
				finalKey = leaf
			case []any:
				if leaf == "-" {
					finalIndex = len(current)
				} else {
					idx, convErr := strconv.Atoi(leaf)
					if convErr != nil {
						err = fmt.Errorf("path segment %q is not a valid integer index for slice in path %q", leaf, pathRaw)
						return
					}
					finalIndex = idx
				}
			default:
				err = fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}
			return
		}

		switch current := traversalCurrent.(type) {
		case map[string]any:
			val, exists := current[segment]
			if !exists {
				err = fmt.Errorf("path segment %q not found in map for path %q", segment, pathRaw)
				return
			}
			prevContainer = current
			prevKey = segment
			prevIndex = -1
			traversalCurrent = val
		case []any:
			idx, convErr := strconv.Atoi(segment)
			if convErr != nil {
				err = fmt.Errorf("path segment %q is not a valid integer index for slice in path %q", segment, pathRaw)
				return
			}
			if idx < 0 || idx >= len(current) {
				err = fmt.Errorf("index %d out of bounds for slice (len %d) at segment %q in path %q", idx, len(current), segment, pathRaw)
				return
			}
			prevContainer = current
			prevKey = ""
			prevIndex = idx
			traversalCurrent = current[idx]
		default:
			err = fmt.Errorf("path %q traverses a non-container (neither map nor slice) at segment %q (value type: %T)", pathRaw, segment, traversalCurrent)
			return
		}
	}
	return
}

func insertValueIntoSlice(slice []any, index int, value any) []any {
	if index == len(slice) {
		return append(slice, value)
	}
	slice = append(slice, nil)
	copy(slice[index+1:], slice[index:])
	slice[index] = value
	return slice
}

func removeValueFromSlice(slice []any, index int) ([]any, any) {
	val := slice[index]
	copy(slice[index:], slice[index+1:])
	last := len(slice) - 1
	slice[last] = nil
	return slice[:last], val
}

func assignSliceToParent(parent any, key string, index int, updated []any, op string) error {
	switch p := parent.(type) {
	case map[string]any:
		p[key] = updated
		return nil
	case []any:
		if index < 0 || index >= len(p) {
			return fmt.Errorf("internal error: cannot assign updated slice for op %q", op)
		}
		p[index] = updated
		return nil
	default:
		return fmt.Errorf("internal error: cannot assign updated slice for op %q", op)
	}
}

// jsonEqual compares two values according to JSON Patch "test" semantics.
func jsonEqual(a, b any) bool {
	if af, aok := getNumericValue(a); aok {
		if bf, bok := getNumericValue(b); bok {
			return af == bf
		}
		return false
	}

	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	case map[string]any:
		bm, ok := b.(map[string]any)
		if !ok || len(av) != len(bm) {
			return false
		}
		for k, v := range av {
			bv, exists := bm[k]
			if !exists || !jsonEqual(v, bv) {
				return false
			}
		}
		return true
	case []any:
		bs, ok := b.([]any)
		if !ok {
			bsIface, ok2 := b.([]interface{})
			if !ok2 {
				return false
			}
			bs = bsIface
		}
		if len(av) != len(bs) {
			return false
		}
		for i := range av {
			if !jsonEqual(av[i], bs[i]) {
				return false
			}
		}
		return true
	default:
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
}

// utf16OffsetToRuneIndex converts a JavaScript UTF-16 offset to a Go rune index.
func utf16OffsetToRuneIndex(text string, jsOffset int) int {
	if jsOffset <= 0 {
		return 0
	}
	runeIndex := 0
	codeUnits := 0
	for _, r := range text {
		unit := 1
		if r > 0xFFFF {
			unit = 2
		}
		if codeUnits+unit > jsOffset {
			break
		}
		codeUnits += unit
		runeIndex++
	}
	return runeIndex
}

// utf16LenToRuneLen converts a JavaScript UTF-16 length starting at jsStart to a rune length.
func utf16LenToRuneLen(text string, jsStart, jsLen int) int {
	if jsLen <= 0 {
		return 0
	}
	startRune := utf16OffsetToRuneIndex(text, jsStart)
	endRune := utf16OffsetToRuneIndex(text, jsStart+jsLen)
	return endRune - startRune
}

// utf16Length returns the length of the string in UTF-16 code units.
func utf16Length(text string) int {
	l := 0
	for _, r := range text {
		if r > 0xFFFF {
			l += 2
		} else {
			l++
		}
	}
	return l
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
					return fmt.Errorf("op %q on root path %q requires a %q field", opType, pathRaw, "value")
				}
				newMapValue, newIsMap := newValue.(map[string]any)
				if !newIsMap {
					return fmt.Errorf("op %q on root path %q with value of type %T; expected map[string]any", opType, pathRaw, newValue)
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
				return fmt.Errorf("op %q on root path %q is not supported or not meaningful for a map document", opType, pathRaw)
			}
		}

		parentContainer, finalKey, finalIndex, containerParent, containerParentKey, containerParentIndex, err := resolvePath(doc, pathRaw)
		if err != nil {
			return err
		}

		switch opType {
		case "add":
			value, ok := op["value"]
			if !ok {
				return fmt.Errorf("op %q missing %q field for path %q", "add", "value", pathRaw)
			}
			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = value
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex > len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "add", pathRaw, len(targetSlice))
				}
				updatedSlice := insertValueIntoSlice(targetSlice, finalIndex, value)
				if err := assignSliceToParent(containerParent, containerParentKey, containerParentIndex, updatedSlice, "add"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}

		case "remove":
			if targetMap, ok := parentContainer.(map[string]any); ok {
				if _, exists := targetMap[finalKey]; !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", finalKey, pathRaw)
				}
				delete(targetMap, finalKey)
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "remove", pathRaw, len(targetSlice))
				}
				updatedSlice, _ := removeValueFromSlice(targetSlice, finalIndex)
				if err := assignSliceToParent(containerParent, containerParentKey, containerParentIndex, updatedSlice, "remove"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}

		case "replace":
			value, valueExists := op["value"]
			if !valueExists {
				return fmt.Errorf("op %q missing %q field for path %q", "replace", "value", pathRaw)
			}
			if targetMap, ok := parentContainer.(map[string]any); ok {
				if _, exists := targetMap[finalKey]; !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", finalKey, pathRaw)
				}
				targetMap[finalKey] = value
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "replace", pathRaw, len(targetSlice))
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
				return fmt.Errorf("invalid %q op parameters (pos/str missing or wrong type) for path %q", "str_ins", pathRaw)
			}
			var currentString string
			var getStringOk bool
			var valAtPathForError any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				if val, exists := targetMap[finalKey]; exists {
					currentString, getStringOk = val.(string)
					valAtPathForError = val
				} else {
					return fmt.Errorf("target key %q for %q not found in map at path %q", finalKey, "str_ins", pathRaw)
				}
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex >= 0 && finalIndex < len(targetSlice) {
					currentString, getStringOk = targetSlice[finalIndex].(string)
					valAtPathForError = targetSlice[finalIndex]
				} else {
					return fmt.Errorf("index %d out of bounds for %q (getting string) at path %q", finalIndex, "str_ins", pathRaw)
				}
			} else {
				return fmt.Errorf("parent for %q op at path %q is not a map or slice (type %T)", "str_ins", pathRaw, parentContainer)
			}

			if !getStringOk {
				return fmt.Errorf("target of %q at path %q is not a string (actual type: %T, value: %+v)", "str_ins", pathRaw, valAtPathForError, valAtPathForError)
			}

			if int(posFloat) > utf16Length(currentString) {
				return fmt.Errorf("invalid %q %d for %q (string len %d) on path %q", "pos", int(posFloat), "str_ins", utf16Length(currentString), pathRaw)
			}
			pos := utf16OffsetToRuneIndex(currentString, int(posFloat))
			runes := []rune(currentString)
			if pos < 0 || pos > len(runes) {
				return fmt.Errorf("invalid %q %d for %q (string len %d) on path %q", "pos", pos, "str_ins", len(runes), pathRaw)
			}
			resultStr := string(runes[:pos]) + strToInsert + string(runes[pos:])

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = resultStr
			} else if targetSlice, ok := parentContainer.([]any); ok {
				targetSlice[finalIndex] = resultStr
			}

		case "str_del":
			posAny, posPresent := op["pos"]
			strToDelete, strPresent := op["str"].(string)
			lenAny, lenPresent := op["len"]
			posFloat, posOk := getNumericValue(posAny)

			if !posPresent || !posOk {
				return fmt.Errorf("invalid %q op parameters (pos missing or wrong type) for path %q", "str_del", pathRaw)
			}

			var currentString string
			var getStringOk bool
			var valAtPathForError any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				if val, exists := targetMap[finalKey]; exists {
					currentString, getStringOk = val.(string)
					valAtPathForError = val
				} else {
					return fmt.Errorf("target key %q for %q not found in map at path %q", finalKey, "str_del", pathRaw)
				}
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex >= 0 && finalIndex < len(targetSlice) {
					currentString, getStringOk = targetSlice[finalIndex].(string)
					valAtPathForError = targetSlice[finalIndex]
				} else {
					return fmt.Errorf("index %d out of bounds for %q (getting string) at path %q", finalIndex, "str_del", pathRaw)
				}
			} else {
				return fmt.Errorf("parent for %q op at path %q is not a map or slice (type %T)", "str_del", pathRaw, parentContainer)
			}

			if !getStringOk {
				return fmt.Errorf("target of %q at path %q is not a string (actual type: %T, value: %+v)", "str_del", pathRaw, valAtPathForError, valAtPathForError)
			}

			if int(posFloat) > utf16Length(currentString) {
				return fmt.Errorf("invalid %q %d or %q %v for %q (string len %d) on path %q", "pos", int(posFloat), "len", lenAny, "str_del", utf16Length(currentString), pathRaw)
			}

			pos := utf16OffsetToRuneIndex(currentString, int(posFloat))
			var length int
			if strPresent {
				length = len([]rune(strToDelete))
			} else if lenPresent {
				lenFloat, lenOk := getNumericValue(lenAny)
				if !lenOk {
					return fmt.Errorf("invalid %q op parameters (len wrong type) for path %q", "str_del", pathRaw)
				}
				length = utf16LenToRuneLen(currentString, int(posFloat), int(lenFloat))
			} else {
				return fmt.Errorf("invalid %q op parameters (str or len required) for path %q", "str_del", pathRaw)
			}

			runes := []rune(currentString)
			if pos < 0 || length < 0 || pos+length > len(runes) {
				return fmt.Errorf("invalid %q %d or %q %d for %q (string len %d) on path %q", "pos", pos, "len", length, "str_del", len(runes), pathRaw)
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
				return fmt.Errorf("op %q missing %q field for path %q", "inc", "inc", pathRaw)
			}
			incOpValFloat, incOpValIsNumber := getNumericValue(incValueFromOp)
			if !incOpValIsNumber {
				return fmt.Errorf("op %q %q field is not a recognized number (got %T) for path %q", "inc", "inc", incValueFromOp, pathRaw)
			}

			var currentValue any

			if targetMap, ok := parentContainer.(map[string]any); ok {
				val, exists := targetMap[finalKey]
				if !exists {
					return fmt.Errorf("target key %q for %q not found in map at path %q", finalKey, "inc", pathRaw)
				}
				currentValue = val
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q at path %q (slice len %d)", finalIndex, "inc", pathRaw, len(targetSlice))
				}
				currentValue = targetSlice[finalIndex]
			} else {
				return fmt.Errorf("parent container for %q at path %q is neither a map nor a slice (type %T)", "inc", pathRaw, parentContainer)
			}

			currentNumAsFloat, successfullyReadCurrentValue := getNumericValue(currentValue)
			if !successfullyReadCurrentValue {
				var targetIdentifier string
				if finalKey != "" {
					targetIdentifier = fmt.Sprintf("key %q", finalKey)
				} else {
					targetIdentifier = fmt.Sprintf("index %d", finalIndex)
				}
				return fmt.Errorf("target %s of %q at path %q is not a number. Value: %+v, Type: %T", targetIdentifier, "inc", pathRaw, currentValue, currentValue)
			}

			incrementedResult := currentNumAsFloat + incOpValFloat
			finalValueToStore := int(incrementedResult)

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = finalValueToStore
			} else if targetSlice, ok := parentContainer.([]any); ok {
				targetSlice[finalIndex] = finalValueToStore
			}

		case "copy":
			fromRaw, ok := op["from"].(string)
			if !ok {
				return fmt.Errorf("op %q missing %q field for path %q", "copy", "from", pathRaw)
			}
			fromParent, fromKey, fromIdx, _, _, _, err := resolvePath(doc, fromRaw)
			if err != nil {
				return err
			}
			var valToCopy any
			if fromMap, ok := fromParent.(map[string]any); ok {
				v, exists := fromMap[fromKey]
				if !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", fromKey, fromRaw)
				}
				valToCopy = v
			} else if fromSlice, ok := fromParent.([]any); ok {
				if fromIdx < 0 || fromIdx >= len(fromSlice) {
					return fmt.Errorf("index %d out of bounds for slice (len %d) at segment %q in path %q", fromIdx, len(fromSlice), fromKey, fromRaw)
				}
				valToCopy = fromSlice[fromIdx]
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", fromRaw, fromParent)
			}

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = valToCopy
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex > len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "copy", pathRaw, len(targetSlice))
				}
				updatedSlice := insertValueIntoSlice(targetSlice, finalIndex, valToCopy)
				if err := assignSliceToParent(containerParent, containerParentKey, containerParentIndex, updatedSlice, "copy"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}

		case "move":
			fromRaw, ok := op["from"].(string)
			if !ok {
				return fmt.Errorf("op %q missing %q field for path %q", "move", "from", pathRaw)
			}
			if strings.HasPrefix(pathRaw+"/", fromRaw+"/") {
				return fmt.Errorf("from path %q is a proper prefix of path %q", fromRaw, pathRaw)
			}
			fromParent, fromKey, fromIdx, fromContainerParent, fromContainerKey, fromContainerIndex, err := resolvePath(doc, fromRaw)
			if err != nil {
				return err
			}
			var valToMove any
			if fromMap, ok := fromParent.(map[string]any); ok {
				v, exists := fromMap[fromKey]
				if !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", fromKey, fromRaw)
				}
				valToMove = v
				delete(fromMap, fromKey)
			} else if fromSlice, ok := fromParent.([]any); ok {
				if fromIdx < 0 || fromIdx >= len(fromSlice) {
					return fmt.Errorf("index %d out of bounds for slice (len %d) at segment %q in path %q", fromIdx, len(fromSlice), fromKey, fromRaw)
				}
				updatedFrom, removed := removeValueFromSlice(fromSlice, fromIdx)
				valToMove = removed
				if err := assignSliceToParent(fromContainerParent, fromContainerKey, fromContainerIndex, updatedFrom, "move"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", fromRaw, fromParent)
			}

			parentContainer, finalKey, finalIndex, containerParent, containerParentKey, containerParentIndex, err = resolvePath(doc, pathRaw)
			if err != nil {
				return err
			}

			if targetMap, ok := parentContainer.(map[string]any); ok {
				targetMap[finalKey] = valToMove
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex > len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "move", pathRaw, len(targetSlice))
				}
				updatedSlice := insertValueIntoSlice(targetSlice, finalIndex, valToMove)
				if err := assignSliceToParent(containerParent, containerParentKey, containerParentIndex, updatedSlice, "move"); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}

		case "test":
			value, ok := op["value"]
			if !ok {
				return fmt.Errorf("op %q missing %q field for path %q", "test", "value", pathRaw)
			}
			var currentVal any
			if targetMap, ok := parentContainer.(map[string]any); ok {
				v, exists := targetMap[finalKey]
				if !exists {
					return fmt.Errorf("path segment %q not found in map for path %q", finalKey, pathRaw)
				}
				currentVal = v
			} else if targetSlice, ok := parentContainer.([]any); ok {
				if finalIndex < 0 || finalIndex >= len(targetSlice) {
					return fmt.Errorf("index %d out of bounds for %q op at path %q (slice len %d)", finalIndex, "test", pathRaw, len(targetSlice))
				}
				currentVal = targetSlice[finalIndex]
			} else {
				return fmt.Errorf("path %q traverses a non-container (neither map nor slice) before final segment; parent is type %T", pathRaw, parentContainer)
			}
			if !jsonEqual(currentVal, value) {
				return fmt.Errorf("test operation failed at path %q", pathRaw)
			}

		default:
			return fmt.Errorf("unhandled op type %q for path %q", opType, pathRaw)
		}
	}
	return nil
}
