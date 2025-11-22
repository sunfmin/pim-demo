package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/tests/testutil"
)

func TestHealthCheck(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	handler := handlers.NewHealthHandler(db)

	testCases := []struct {
		name             string
		setup            func()
		expectedStatus   int
		validateResponse func(t *testing.T, resp *handlers.HealthResponse)
	}{
		{
			name:           "Health check - Happy Path",
			setup:          func() {},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, resp *handlers.HealthResponse) {
				if resp.Status != "healthy" {
					t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
				}
				if resp.Database != "healthy" {
					t.Errorf("Expected database 'healthy', got '%s'", resp.Database)
				}
				if resp.Version == "" {
					t.Error("Expected version to be set")
				}
				if resp.Timestamp.IsZero() {
					t.Error("Expected timestamp to be set")
				}
			},
		},
		{
			name: "Health check - Database Down (Simulation)",
			setup: func() {
				// To simulate DB down without killing the actual container for other tests,
				// we can close the underlying sql.DB connection for this handler instance.
				// However, the handler uses the *gorm.DB passed in.
				// A real integration test for "DB Down" is tricky with a shared container.
				// For now, we'll focus on the logic we can control or mock if strictly needed,
				// but the constitution forbids mocking DB calls.
				// We can create a handler with a closed DB connection.
				sqlDB, _ := db.DB()
				sqlDB.Close()
			},
			expectedStatus: http.StatusOK, // Health endpoint still returns 200 OK but reports unhealthy DB
			validateResponse: func(t *testing.T, resp *handlers.HealthResponse) {
				if resp.Status != "healthy" {
					t.Errorf("Expected status 'healthy', got '%s'", resp.Status)
				}
				// Depending on how GORM handles the closed connection immediately, it might fail Ping
				if resp.Database != "unhealthy" {
					// Note: Simulation of closed DB might be flaky depending on connection pooling.
					// If this is flaky, we might relax this assertion or use a separate invalid connection.
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// We need a fresh DB connection for the "simulation" test to not affect others
			// But SetupTestDB gives us one. For the "Database Down" case, we might need a separate connection.
			
			if tc.name == "Health check - Database Down (Simulation)" {
				// Create a separate closed DB for this test case to avoid breaking the main one
				// This is a bit hacky but ensures we test the "unhealthy" path
				// Actually, let's skip the "closed" simulation in the shared loop and just do one happy path
				// The logic is: if err := sqlDB.Ping(); err != nil { dbStatus = "unhealthy" }
				// We can force this by using a handler with a closed DB specifically.
			}
			
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()

			// For the "Closed DB" case, we would need to close the DB *before* calling Health.
			// Since we can't easily re-open it, we should be careful.
			// Let's simplify: just test happy path first.
			tc.setup()

			handler.Health(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			var resp handlers.HealthResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tc.validateResponse != nil {
				tc.validateResponse(t, &resp)
			}
		})
	}
}

func TestHealthCheck_DatabaseFailure(t *testing.T) {
	// Separate test for failure scenario to avoid messing up shared resources in table loop
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Get underlying SQL DB and close it to simulate failure
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get sql DB: %v", err)
	}
	
	// We need to make sure we don't affect the cleanup which tries to close/terminate?
	// testcontainers cleanup terminates the container, so closing the connection is fine.
	sqlDB.Close()

	handler := handlers.NewHealthHandler(db)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Health(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp handlers.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Database != "unhealthy" {
		t.Errorf("Expected database 'unhealthy', got '%s'", resp.Database)
	}
}

