package handlers

import (
	"context"
	"net/http"
	"time"

	"GO2GETHER_BACK-END/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler handles health check related requests
type HealthHandler struct {
	db *pgxpool.Pool
}

// NewHealthHandler creates a new HealthHandler instance
func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

// HealthResp represents the response structure for health checks
type HealthResp struct {
	Status  string `json:"status"`
	Details any    `json:"details,omitempty"`
}

// HealthCheck handles basic health check (no database)
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSONResponse(w, http.StatusOK, HealthResp{Status: "ok"})
}

// LivenessCheck handles process liveness check
func (h *HealthHandler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSONResponse(w, http.StatusOK, HealthResp{Status: "alive"})
}

// ReadinessCheck handles readiness check (includes database connectivity)
func (h *HealthHandler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		utils.WriteJSONResponse(w, http.StatusServiceUnavailable, HealthResp{
			Status:  "degraded",
			Details: map[string]any{"db": err.Error()},
		})
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, HealthResp{
		Status:  "ready",
		Details: map[string]any{"db": "ok"},
	})
}
