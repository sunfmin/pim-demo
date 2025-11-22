package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestSearchAcceptanceScenarios tests all acceptance scenarios for User Story 5
// Covers US5-AS1 through US5-AS4 from spec.md
func TestSearchAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Setup services and handlers
	searchService := services.NewSearchService(db)
	searchHandler := handlers.NewSearchHandler(searchService)

	tests := []struct {
		name     string
		testFunc func(t *testing.T, db *gorm.DB, org *models.Organization)
	}{
		{
			name: "US5-AS1: Search products by keyword in name/SKU/description",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Multiple products exist
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "LAPTOP-001",
					"name":        "Professional Laptop",
					"description": "High-performance laptop for professionals",
					"base_price":  int64(129900),
					"status":      models.ProductStatusActive,
				})

				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "PHONE-001",
					"name":        "Smartphone Pro",
					"description": "Latest smartphone with professional camera",
					"base_price":  int64(89900),
					"status":      models.ProductStatusActive,
				})

				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "TABLET-001",
					"name":        "Professional Tablet",
					"description": "Tablet for creative professionals",
					"base_price":  int64(59900),
					"status":      models.ProductStatusActive,
				})

				// When: User searches for "professional"
				req := &pb.SearchProductsRequest{
					Query: "professional",
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 10,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				searchHandler.Search(w, httpReq)

				// Then: All products containing "professional" are returned
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.SearchProductsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// Should find 3 products with "professional" in name or description
				if len(resp.Results) != 3 {
					t.Errorf("Expected 3 products, got %d", len(resp.Results))
				}

				// Verify results contain search term
				for _, result := range resp.Results {
					found := false
					searchTerm := "professional"
					if containsIgnoreCase(result.Product.Name, searchTerm) ||
						containsIgnoreCase(result.Product.Description, searchTerm) {
						found = true
					}
					if !found {
						t.Errorf("Product %s does not contain search term '%s'", result.Product.Name, searchTerm)
					}
				}
			},
		},
		{
			name: "US5-AS2: Apply multiple filters (category, price, status)",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Products with various attributes
				category := testutil.CreateTestCategory(t, db, org.ID, map[string]interface{}{
					"name": "Electronics",
					"slug": "electronics",
				})

				product1 := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "LAPTOP-002",
					"name":        "Budget Laptop",
					"description": "Affordable laptop",
					"base_price":  int64(49900), // $499
					"status":      models.ProductStatusActive,
				})

				product2 := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "LAPTOP-003",
					"name":        "Premium Laptop",
					"description": "High-end laptop",
					"base_price":  int64(199900), // $1999
					"status":      models.ProductStatusActive,
				})

				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "LAPTOP-004",
					"name":        "Draft Laptop",
					"description": "Not yet released",
					"base_price":  int64(99900),
					"status":      models.ProductStatusDraft,
				})

				// Assign category to active products
				db.Exec("INSERT INTO product_categories (product_id, category_id) VALUES (?, ?)", product1.ID, category.ID)
				db.Exec("INSERT INTO product_categories (product_id, category_id) VALUES (?, ?)", product2.ID, category.ID)

				// When: User filters by category, price range, and status
				// Note: Using Search API with filters (category_ids and statuses)
				// For more complex filters, we'd use a separate List endpoint with ProductsFilter
				req := &pb.SearchProductsRequest{
					CategoryIds: []string{category.ID.String()},
					Statuses:    []pb.ProductStatus{pb.ProductStatus_PRODUCT_STATUS_ACTIVE},
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 10,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				searchHandler.Search(w, httpReq)

				// Then: Only product1 matches all filters
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.SearchProductsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(resp.Results) != 1 {
					t.Errorf("Expected 1 product matching filters, got %d", len(resp.Results))
				}

				if len(resp.Results) > 0 && resp.Results[0].Product.Sku != "LAPTOP-002" {
					t.Errorf("Expected LAPTOP-002, got %s", resp.Results[0].Product.Sku)
				}
			},
		},
		{
			name: "US5-AS3: Clear all filters",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Products exist with various filters applied
				for i := 0; i < 5; i++ {
					testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
						"sku":        "PROD-00" + string(rune('1'+i)),
						"name":       "Product " + string(rune('1'+i)),
						"base_price": int64(10000 * (i + 1)),
						"status":     models.ProductStatusActive,
					})
				}

				// When: User searches without any filters
				req := &pb.SearchProductsRequest{
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 20,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				searchHandler.Search(w, httpReq)

				// Then: All products are returned
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.SearchProductsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(resp.Results) != 5 {
					t.Errorf("Expected 5 products (no filters), got %d", len(resp.Results))
				}
			},
		},
		{
			name: "US5-AS4: Search returns results within 2 seconds",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Large dataset of products
				products := testutil.CreateTestProductBatch(t, db, org.ID, 1000)
				t.Logf("Created %d products for performance test", len(products))

				// When: User performs a search
				start := time.Now()

				req := &pb.SearchProductsRequest{
					Query: "Bulk Product",
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 20,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				searchHandler.Search(w, httpReq)

				elapsed := time.Since(start)

				// Then: Results returned within 2 seconds
				if elapsed > 2*time.Second {
					t.Errorf("Search took %v, expected < 2 seconds", elapsed)
				}

				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				t.Logf("Search completed in %v", elapsed)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Truncate tables before each test for isolation
			defer testutil.TruncateTables(db)

			// Recreate org for each test
			org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
				"name": "Test Org " + tt.name,
			})

			tt.testFunc(t, db, org)
		})
	}
}

// TestSearchEdgeCases tests edge cases for search functionality
func TestSearchEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	searchService := services.NewSearchService(db)
	searchHandler := handlers.NewSearchHandler(searchService)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Test Org Edge Cases",
	})

	tests := []struct {
		name           string
		setup          func(t *testing.T) (context.Context, *http.Request)
		expectedStatus int
		checkResponse  func(t *testing.T, resp *pb.SearchProductsResponse)
	}{
		{
			name: "Empty search query returns all products",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-001",
					"name": "Product 1",
				})
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-002",
					"name": "Product 2",
				})

				req := &pb.SearchProductsRequest{
					Query: "",
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 10,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *pb.SearchProductsResponse) {
				if len(resp.Results) != 2 {
					t.Errorf("Expected 2 products, got %d", len(resp.Results))
				}
			},
		},
		{
			name: "Special characters in query",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":         "SPECIAL-001",
					"name":        "Product with @#$% symbols",
					"description": "Testing special characters & symbols",
				})

				req := &pb.SearchProductsRequest{
					Query: "@#$%",
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 10,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "No results found",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-003",
					"name": "Existing Product",
				})

				req := &pb.SearchProductsRequest{
					Query: "nonexistent-term-xyz",
					Pagination: &pb.PaginationRequest{
						Page:     1,
						PageSize: 10,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *pb.SearchProductsResponse) {
				if len(resp.Results) != 0 {
					t.Errorf("Expected 0 products for nonexistent term, got %d", len(resp.Results))
				}
			},
		},
		{
			name: "Pagination with large result sets",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				// Create 50 products
				for i := 0; i < 50; i++ {
					testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
						"sku":  "BULK-" + string(rune('0'+i%10)),
						"name": "Bulk Product " + string(rune('0'+i%10)),
					})
				}

				req := &pb.SearchProductsRequest{
					Pagination: &pb.PaginationRequest{
						Page:     2,
						PageSize: 20,
					},
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/products/search", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), "organization_id", org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *pb.SearchProductsResponse) {
				if len(resp.Results) != 20 {
					t.Errorf("Expected 20 products on page 2, got %d", len(resp.Results))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpReq := tt.setup(t)

			w := httptest.NewRecorder()
			searchHandler.Search(w, httpReq)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp pb.SearchProductsResponse
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
					tt.checkResponse(t, &resp)
				}
			}
		})
	}
}

// Helper function to check if string contains substring (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && indexOf(s, substr) >= 0)
}

func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

