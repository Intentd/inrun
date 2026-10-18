package httpapi

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data == nil {
		return
	}

	// Encode response as JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

}

// WriteJSONError writes a structured JSON error response.
// This is a convenience wrapper around writeJSON for common error patterns.
func WriteJSONError(w http.ResponseWriter, status int, errMsg string, details ...string) {
	resp := H{
		"error": errMsg,
	}
	if len(details) > 0 && details[0] != "" {
		resp["message"] = details[0]
	}
	WriteJSON(w, status, resp)
}

// H is a shortcut for map[string]interface{} used in JSON responses.
// Prefer this over raw map literals for consistency and readability.
type H map[string]interface{}
