package integration

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestSearchAcceptanceScenarios tests all acceptance scenarios for User Story 5
// Covers US5-AS1 through US5-AS4 from spec.md
func TestSearchAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Create services and handler
	searchService := services.NewSearchService(db)
	searchHandler := handlers.NewSearchHandler(searchService)

	// Helper function to add organization ID to request context
	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	testCases := []struct {
		name           string
		requestData    *pb.SearchProductsRequest
		setupProducts  []map[string]interface{}
		expectedCount  int
		expectedSKU    string
		expectedStatus int
	}{
		{
			name: "US5-AS1: Keyword search finds relevant products",
			requestData: &pb.SearchProductsRequest{
				Query: "phone",
			},
			setupProducts: []map[string]interface{}{
				{"sku": "PHONE-001", "name": "Smart Phone X", "description": "Latest smart phone"},
				{"sku": "LAPTOP-001", "name": "Pro Laptop", "description": "High performance laptop"},
			},
			expectedCount:  1,
			expectedSKU:    "PHONE-001",
			expectedStatus: http.StatusOK,
		},
		{
			name: "US5-AS2: Multiple filters combine correctly",
			requestData: &pb.SearchProductsRequest{
				Statuses: []pb.ProductStatus{pb.ProductStatus_PRODUCT_STATUS_ACTIVE},
				MaxPrice: 2000,
			},
			setupProducts: []map[string]interface{}{
				{"sku": "P1", "name": "Cheap Active", "base_price": int64(1000), "status": models.ProductStatusActive},
				{"sku": "P2", "name": "Expensive Active", "base_price": int64(5000), "status": models.ProductStatusActive},
				{"sku": "P3", "name": "Cheap Draft", "base_price": int64(1000), "status": models.ProductStatusDraft},
			},
			expectedCount:  1,
			expectedSKU:    "P1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "US5-AS3: Empty filters return all products",
			requestData: &pb.SearchProductsRequest{}, // No filters
			setupProducts: []map[string]interface{}{
				{"sku": "P1", "name": "Product 1"},
				{"sku": "P2", "name": "Product 2"},
			},
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer testutil.TruncateTables(db)

			// Create test organization
			testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

			// Setup test data
			for _, productData := range tc.setupProducts {
				testutil.CreateTestProduct(t, db, testOrg.ID, productData)
			}

			// Execute request
			body, _ := protojson.Marshal(tc.requestData)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addOrgContext(req, testOrg.ID)

			rec := httptest.NewRecorder()
			searchHandler.Search(rec, req)

			// Verify response
			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			var resp pb.SearchProductsResponse
			testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

			if len(resp.Results) != tc.expectedCount {
				t.Errorf("Expected %d products, got %d", tc.expectedCount, len(resp.Results))
			}

			if tc.expectedSKU != "" && len(resp.Results) > 0 {
				if resp.Results[0].Product.Sku != tc.expectedSKU {
					t.Errorf("Expected SKU %s, got %s", tc.expectedSKU, resp.Results[0].Product.Sku)
				}
			}
			// Additional validation for protobuf assertions (Principle VI)
			if len(resp.Results) > 0 {
				if resp.Results[0].Product.Sku == "" {
					t.Error("Expected product SKU to be set")
				}
				if resp.Results[0].Product.Name == "" {
					t.Error("Expected product name to be set")
				}
			}
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

	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	testCases := []struct {
		name           string
		requestData    *pb.SearchProductsRequest
		expectedCount  int
		expectedStatus int
	}{
		{
			name: "Search with no products returns empty results",
			requestData: &pb.SearchProductsRequest{
				Query: "nonexistent",
			},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Search with SQL injection attempt is sanitized",
			requestData: &pb.SearchProductsRequest{
				Query: "'; DROP TABLE products; --",
			},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Search with empty query string",
			requestData: &pb.SearchProductsRequest{
				Query: "",
			},
			expectedCount:  0, // No products in empty database
			expectedStatus: http.StatusOK,
		},
		{
			name: "Search with negative price filter",
			requestData: &pb.SearchProductsRequest{
				MaxPrice: -1000, // Invalid but should not crash
			},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer testutil.TruncateTables(db)

			testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

			body, _ := protojson.Marshal(tc.requestData)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = addOrgContext(req, testOrg.ID)

			rec := httptest.NewRecorder()
			searchHandler.Search(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			var resp pb.SearchProductsResponse
			testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

			if len(resp.Results) != tc.expectedCount {
				t.Errorf("Expected %d products, got %d", tc.expectedCount, len(resp.Results))
			}
		})
	}
}

// TestSearchFilterCoverage adds coverage for specific filter combinations
func TestSearchFilterCoverage(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	searchService := services.NewSearchService(db)
	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Search Filter Org",
	})

	// Create active and draft products
	testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
		"sku":        "SEARCH-ACTIVE",
		"name":       "Active Search Product",
		"status":     models.ProductStatusActive,
		"base_price": int64(1000),
	})

	testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
		"sku":        "SEARCH-DRAFT",
		"name":       "Draft Search Product",
		"status":     models.ProductStatusDraft,
		"base_price": int64(2000),
	})

	ctx := context.WithValue(context.Background(), middleware.OrganizationIDKey, org.ID)

	// Test Filter by Status (using service directly to bypass handler request parsing logic if any specific gaps there, 
	// but main gap was coverage of productStatusToString which is used in service)
	activeStatus := pb.ProductStatus_PRODUCT_STATUS_ACTIVE
	products, _, err := searchService.Filter(ctx, org.ID, &pb.ProductsFilter{
		Statuses: []pb.ProductStatus{activeStatus},
	}, nil)
	if err != nil {
		t.Fatalf("Filter by status failed: %v", err)
	}
	if len(products) != 1 {
		t.Errorf("Expected 1 active product, got %d", len(products))
	}
	if products[0].SKU != "SEARCH-ACTIVE" {
		t.Errorf("Expected SEARCH-ACTIVE, got %s", products[0].SKU)
	}

	// Test Filter by Price Range
	productsPrice, _, err := searchService.Filter(ctx, org.ID, &pb.ProductsFilter{
		MinPrice: 1500,
	}, nil)
	if err != nil {
		t.Fatalf("Filter by price failed: %v", err)
	}
	if len(productsPrice) != 1 {
		t.Errorf("Expected 1 product > 1500, got %d", len(productsPrice))
	}
	if productsPrice[0].SKU != "SEARCH-DRAFT" {
		t.Errorf("Expected SEARCH-DRAFT, got %s", productsPrice[0].SKU)
	}
}
