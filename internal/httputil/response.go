// Package httputil provides HTTP response utilities.
package httputil

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
