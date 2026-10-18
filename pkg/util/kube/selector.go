package kube

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/intentd/inrun/pkg/util/structured"
)

// MatchOption defines how to compare values
type MatchOption int

const (
	MatchExact MatchOption = iota
	MatchContains
	MatchPrefix
	MatchSuffix
	MatchRegex
	MatchIgnoreCase
)

// FieldSelector represents a selector with comparison options
type FieldSelector struct {
	Path    string
	Value   interface{}
	Options MatchOption
}

// MatchesAllFieldSelectors checks if the given object matches all field selectors.
// Supports various comparison strategies and value types.
func MatchesAllFieldSelectors(obj map[string]interface{}, selectors map[string]interface{}) bool {
	for path, expectedValue := range selectors {
		actualValue, ok := structured.GetNestedPath(obj, path)
		if !ok {
			return false
		}
		if !compareValues(actualValue, expectedValue) {
			return false
		}
	}
	return true
}

// MatchesAllFieldSelectorsWithOptions checks if the given object matches all field selectors with options.
func MatchesAllFieldSelectorsWithOptions(obj map[string]interface{}, selectors []FieldSelector) bool {
	for _, selector := range selectors {
		actualValue, ok := structured.GetNestedPath(obj, selector.Path)
		if !ok {
			return false
		}
		if !compareValuesWithOption(actualValue, selector.Value, selector.Options) {
			return false
		}
	}
	return true
}

// compareValues compares two values with type conversion
func compareValues(actual, expected interface{}) bool {
	// Handle nil
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Try direct equality first
	if actual == expected {
		return true
	}

	// Convert to string for comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return actualStr == expectedStr
}

// compareValuesWithOption compares values with comparison options
func compareValuesWithOption(actual, expected interface{}, option MatchOption) bool {
	// Handle nil
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	switch option {
	case MatchExact:
		return actualStr == expectedStr
	case MatchContains:
		return strings.Contains(actualStr, expectedStr)
	case MatchPrefix:
		return strings.HasPrefix(actualStr, expectedStr)
	case MatchSuffix:
		return strings.HasSuffix(actualStr, expectedStr)
	case MatchIgnoreCase:
		return strings.EqualFold(actualStr, expectedStr)
	case MatchRegex:
		matched, _ := regexp.MatchString(expectedStr, actualStr)
		return matched
	default:
		return actualStr == expectedStr
	}
}

// MatchesAllServeTargetFieldSelectors keeps the original for backward compatibility
func MatchesAllServeTargetFieldSelectors(obj map[string]interface{}, selectors map[string]string) bool {
	return MatchesAllFieldSelectors(obj, convertStringMapToInterface(selectors))
}

// Helper to convert map[string]string to map[string]interface{}
func convertStringMapToInterface(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// MatchesAnyFieldSelector checks if the object matches ANY of the selectors (OR logic)
func MatchesAnyFieldSelector(obj map[string]interface{}, selectors map[string]interface{}) bool {
	for path, expectedValue := range selectors {
		actualValue, ok := structured.GetNestedPath(obj, path)
		if ok && compareValues(actualValue, expectedValue) {
			return true
		}
	}
	return len(selectors) == 0 // Empty selectors returns true (matches nothing)
}

// MatchesAllTypedSelectors supports typed comparisons (int, bool, float, etc.)
func MatchesAllTypedSelectors(obj map[string]interface{}, selectors map[string]interface{}) bool {
	for path, expectedValue := range selectors {
		actualValue, ok := structured.GetNestedPath(obj, path)
		if !ok {
			return false
		}
		if !typedCompare(actualValue, expectedValue) {
			return false
		}
	}
	return true
}

// typedCompare compares values with type awareness
func typedCompare(actual, expected interface{}) bool {
	switch exp := expected.(type) {
	case int, int32, int64:
		// Convert actual to int64 if possible
		actualInt, err := toInt64(actual)
		if err != nil {
			return false
		}
		expectedInt, err := toInt64(exp)
		if err != nil {
			return false
		}
		return actualInt == expectedInt

	case float32, float64:
		actualFloat, err := toFloat64(actual)
		if err != nil {
			return false
		}
		expectedFloat, err := toFloat64(exp)
		if err != nil {
			return false
		}
		return actualFloat == expectedFloat

	case bool:
		actualBool, ok := actual.(bool)
		if !ok {
			return false
		}
		return actualBool == exp

	case string:
		actualStr, ok := actual.(string)
		if !ok {
			return false
		}
		return actualStr == exp

	default:
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}
}

// Helper functions for type conversion
func toInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case float32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
