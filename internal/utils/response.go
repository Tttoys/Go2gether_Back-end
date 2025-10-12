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
