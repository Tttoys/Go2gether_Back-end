package utils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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

// GetUserIDFromContext extracts user ID from request context
// Supports both "userID" and "user_id" keys, and both UUID and string types
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if v := ctx.Value("userID"); v != nil {
		switch t := v.(type) {
		case uuid.UUID:
			return t, true
		case string:
			if id, err := uuid.Parse(t); err == nil {
				return id, true
			}
		}
	}
	if v := ctx.Value("user_id"); v != nil {
		switch t := v.(type) {
		case uuid.UUID:
			return t, true
		case string:
			if id, err := uuid.Parse(t); err == nil {
				return id, true
			}
		}
	}
	return uuid.Nil, false
}

// ValidateJSONRequest validates that the request has proper Content-Type and non-empty body
func ValidateJSONRequest(w http.ResponseWriter, r *http.Request) bool {
	// Check Content-Type for POST, PUT, PATCH requests
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Content-Type header is required")
			return false
		}

		// Allow Content-Type to be "application/json" or "application/json; charset=utf-8"
		if !strings.HasPrefix(contentType, "application/json") {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Content-Type must be application/json")
			return false
		}
	}

	// Check if body is empty (for POST, PUT, PATCH)
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		// Read first byte to check if body is empty
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Failed to read request body")
			return false
		}
		// Restore body for actual decoding
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		// Check if body is empty or only whitespace
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if bodyStr == "" {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Request body is required")
			return false
		}

		// Check if it's a valid JSON (at least starts with { or [)
		if !strings.HasPrefix(bodyStr, "{") && !strings.HasPrefix(bodyStr, "[") {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Request body must be valid JSON")
			return false
		}
	}

	return true
}

// DecodeJSONRequest decodes JSON request body with validation
func DecodeJSONRequest(w http.ResponseWriter, r *http.Request, v interface{}) error {
	// Validate request first
	if !ValidateJSONRequest(w, r) {
		// Return a generic error to indicate validation failed
		// Error response already sent by ValidateJSONRequest
		return errors.New("validation failed")
	}

	// Read body to decode and check for unknown fields
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Failed to read request body")
		return err
	}

	// Use DisallowUnknownFields to detect unknown JSON fields
	decoder := json.NewDecoder(strings.NewReader(string(bodyBytes)))
	decoder.DisallowUnknownFields()

	// Decode JSON
	if err := decoder.Decode(v); err != nil {
		if err == io.EOF {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Request body is required")
			return err
		}
		if _, ok := err.(*json.SyntaxError); ok {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Invalid JSON format: "+err.Error())
			return err
		}
		// Check for unknown field error
		if strings.Contains(err.Error(), "unknown field") {
			WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Unknown field in request body: "+err.Error())
			return err
		}
		WriteErrorResponse(w, http.StatusBadRequest, "Bad Request", "Failed to decode request body: "+err.Error())
		return err
	}

	return nil
}

// Date format constants (ISO 8601 standard)
const (
	// ISO 8601 Date format: YYYY-MM-DD
	// Example: "1990-05-15"
	ISO8601Date = "2006-01-02"
	
	// ISO 8601 Year-Month format: YYYY-MM
	// Example: "2025-05"
	ISO8601Month = "2006-01"
	
	// ISO 8601 DateTime format: RFC3339 (subset of ISO 8601)
	// Example: "2025-01-27T10:30:00Z" or "2025-01-27T10:30:00+07:00"
	// time.RFC3339 = "2006-01-02T15:04:05Z07:00"
)

// ParseDate parses an ISO 8601 date string
// Supports:
//   - ISO 8601 Date format: YYYY-MM-DD (e.g., "1990-05-15")
//   - ISO 8601 DateTime format: RFC3339 (e.g., "2025-01-27T10:30:00Z")
// Returns the parsed time and error if any
func ParseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, errors.New("date string is empty")
	}

	// Try ISO 8601 Date format first (YYYY-MM-DD)
	if len(dateStr) == 10 {
		if t, err := time.Parse(ISO8601Date, dateStr); err == nil {
			return t, nil
		}
	}

	// Try ISO 8601 DateTime format (RFC3339)
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, errors.New("invalid ISO 8601 date format, expected YYYY-MM-DD or RFC3339")
}

// FormatDate formats a time.Time to ISO 8601 Date format (YYYY-MM-DD)
// Used for date fields: birth_date, start_date, end_date
// Example output: "1990-05-15"
func FormatDate(t time.Time) string {
	return t.Format(ISO8601Date)
}

// FormatTimestamp formats a time.Time to ISO 8601 DateTime format (RFC3339)
// RFC3339 is a profile of ISO 8601
// Used for timestamp fields: created_at, updated_at, InvitedAt, JoinedAt, ExpiresAt
// Example output: "2025-01-27T10:30:00Z"
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseMonth parses an ISO 8601 Year-Month string (YYYY-MM)
// Returns the parsed time (first day of month in UTC) and error if any
// Example input: "2025-05"
func ParseMonth(monthStr string) (time.Time, error) {
	if monthStr == "" {
		return time.Time{}, errors.New("month string is empty")
	}

	t, err := time.Parse(ISO8601Month, monthStr)
	if err != nil {
		return time.Time{}, errors.New("invalid ISO 8601 month format, expected YYYY-MM")
	}

	// Return first day of the month in UTC
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// FormatMonth formats a time.Time to ISO 8601 Year-Month format (YYYY-MM)
// Used for month fields: start_month, end_month
// Example output: "2025-05"
func FormatMonth(t time.Time) string {
	return t.Format(ISO8601Month)
}
