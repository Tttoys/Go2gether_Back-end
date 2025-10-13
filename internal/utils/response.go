package utils

import (
	"encoding/json"
	"net/http"
)

// WriteJSONResponse writes a JSON response to the HTTP response writer
func WriteJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteErrorResponse writes an error JSON response to the HTTP response writer
func WriteErrorResponse(w http.ResponseWriter, status int, error, message string) {
	response := map[string]string{
		"error":   error,
		"message": message,
	}
	WriteJSONResponse(w, status, response)
}
