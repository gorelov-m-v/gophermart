package http

import (
	"net/http"

	"gophermart/internal/httputil"
)

// ErrorResponse is an alias for httputil.ErrorResponse.
type ErrorResponse = httputil.ErrorResponse

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	httputil.WriteError(w, statusCode, message)
}

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	httputil.WriteJSON(w, statusCode, data)
}
