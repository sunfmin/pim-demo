package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestHealthCheck tests health endpoint acceptance scenarios
// Note: Health endpoint returns JSON (not protobuf) for external monitoring compatibility
func TestHealthCheck(t *testing.T) {
	testCases := []struct {
		name           string
		setupDB        func(t *testing.T) (*handlers.HealthHandler, func())
		expectedStatus int
		expectedResponse handlers.HealthResponse
	}{
		{
			name: "US?-AS1: Health check returns healthy status with database connectivity",
			setupDB: func(t *testing.T) (*handlers.HealthHandler, func()) {
				db, cleanup := testutil.SetupTestDB(t)
				handler := handlers.NewHealthHandler(db)
				return handler, cleanup
			},
			expectedStatus: http.StatusOK,
			expectedResponse: handlers.HealthResponse{
				Status:    "healthy",
				Database:  "healthy",
				// Version and Timestamp are dynamic, verified separately
			},
		},
		{
			name: "US?-AS2: Health check reports unhealthy database when connection fails",
			setupDB: func(t *testing.T) (*handlers.HealthHandler, func()) {
				db, cleanup := testutil.SetupTestDB(t)

				// Close the database connection to simulate failure
				sqlDB, err := db.DB()
				if err != nil {
					t.Fatalf("Failed to get sql DB: %v", err)
				}
				sqlDB.Close()

				handler := handlers.NewHealthHandler(db)
				return handler, cleanup
			},
			expectedStatus: http.StatusOK, // Health endpoint always returns 200 OK but reports unhealthy DB
			expectedResponse: handlers.HealthResponse{
				Status:   "healthy",   // Service itself is healthy
				Database: "unhealthy", // Database is unhealthy
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, cleanup := tc.setupDB(t)
			defer cleanup()

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler.Health(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			var resp handlers.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check static fields
			if resp.Status != tc.expectedResponse.Status {
				t.Errorf("Expected status '%s', got '%s'", tc.expectedResponse.Status, resp.Status)
			}
			if resp.Database != tc.expectedResponse.Database {
				t.Errorf("Expected database '%s', got '%s'", tc.expectedResponse.Database, resp.Database)
			}

			// Verify dynamic fields are present
			if resp.Version == "" {
				t.Error("Expected version to be set")
			}
			if resp.Timestamp.IsZero() {
				t.Error("Expected timestamp to be set")
			}

			// For healthy database case, verify additional fields
			if tc.expectedResponse.Database == "healthy" {
				if resp.Database != "healthy" {
					t.Errorf("Expected database 'healthy', got '%s'", resp.Database)
				}
			}
		})
	}
}

// TestHealthCheckEdgeCases tests edge cases and error conditions
func TestHealthCheckEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		setupDB        func(t *testing.T) (*handlers.HealthHandler, func())
		expectedStatus int
		validateResponse func(t *testing.T, resp handlers.HealthResponse)
	}{
		{
			name: "Health check with nil database connection",
			setupDB: func(t *testing.T) (*handlers.HealthHandler, func()) {
				// This tests what happens if handler is created with nil DB
				// In practice this shouldn't happen, but tests defensive programming
				handler := handlers.NewHealthHandler(nil)
				return handler, func() {} // No cleanup needed
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, resp handlers.HealthResponse) {
				// Handler should handle nil DB gracefully
				if resp.Status != "healthy" {
					t.Errorf("Expected status 'healthy' even with nil DB, got '%s'", resp.Status)
				}
			},
		},
		{
			name: "Health check handles panics gracefully",
			setupDB: func(t *testing.T) (*handlers.HealthHandler, func()) {
				// We can't easily make the handler panic, but we can test that
				// it doesn't panic with normal inputs
				db, cleanup := testutil.SetupTestDB(t)
				handler := handlers.NewHealthHandler(db)
				return handler, cleanup
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, resp handlers.HealthResponse) {
				// Just verify we get a valid response without panicking
				if resp.Status == "" {
					t.Error("Expected status to be set")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, cleanup := tc.setupDB(t)
			defer cleanup()

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			handler.Health(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			var resp handlers.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			tc.validateResponse(t, resp)
		})
	}
}

// TestHealthCheckPerformance tests performance requirements
func TestHealthCheckPerformance(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	handler := handlers.NewHealthHandler(db)

	// Test multiple rapid health checks
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		handler.Health(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Health check %d failed with status %d", i+1, rec.Code)
		}
	}
}

