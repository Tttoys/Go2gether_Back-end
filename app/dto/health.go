package dto

// HealthResponse represents the response structure for health checks
type HealthResponse struct {
	Status  string `json:"status"`
	Details any    `json:"details,omitempty"`
}
