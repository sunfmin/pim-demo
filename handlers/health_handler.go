package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db *gorm.DB
}

// NewHealthHandler creates a new health check handler
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
}

// Health handles the health check endpoint
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	dbStatus := "healthy"
	if h.db == nil {
		dbStatus = "unhealthy"
	} else {
		sqlDB, err := h.db.DB()
		if err != nil {
			dbStatus = "unhealthy"
		} else if err := sqlDB.Ping(); err != nil {
			dbStatus = "unhealthy"
		}
	}

	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "0.1.0",
		Database:  dbStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
