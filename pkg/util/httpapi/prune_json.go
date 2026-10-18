package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WriteJSONPruned writes a JSON response with null/empty values removed
func WriteJSONPruned(w http.ResponseWriter, status int, v interface{}) {
	// Convert to JSON first
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to marshal response",
		})
		return
	}

	// Parse into generic structure
	var data interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to parse JSON",
		})
		return
	}

	// Prune null/empty values
	pruned := pruneJSONValue(data)

	// Write the pruned response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(pruned); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// pruneJSONValue recursively removes null values, empty slices, and empty maps
func pruneJSONValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{})
		for k, v := range val {
			if v == nil {
				continue
			}
			cleanedVal := pruneJSONValue(v)
			if cleanedVal != nil {
				// Check if it's an empty map or slice
				switch cleanedValTyped := cleanedVal.(type) {
				case map[string]interface{}:
					if len(cleanedValTyped) > 0 {
						cleaned[k] = cleanedValTyped
					}
				case []interface{}:
					if len(cleanedValTyped) > 0 {
						cleaned[k] = cleanedValTyped
					}
				default:
					cleaned[k] = cleanedValTyped
				}
			}
		}
		if len(cleaned) == 0 {
			return nil
		}
		return cleaned

	case []interface{}:
		cleaned := make([]interface{}, 0, len(val))
		for _, item := range val {
			if item == nil {
				continue
			}
			cleanedItem := pruneJSONValue(item)
			if cleanedItem != nil {
				cleaned = append(cleaned, cleanedItem)
			}
		}
		if len(cleaned) == 0 {
			return nil
		}
		return cleaned

	default:
		// Check for zero values
		if val == nil {
			return nil
		}
		// Check for empty string
		if s, ok := val.(string); ok && s == "" {
			return nil
		}
		// Check for zero number
		if f, ok := val.(float64); ok && f == 0 {
			return nil
		}
		// Check for false boolean
		if b, ok := val.(bool); ok && !b {
			return nil
		}
		return val
	}
}
