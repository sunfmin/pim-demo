package integration

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
		name     string
		scenario string
		testFunc func(t *testing.T)
	}{
		{
			name:     "US5-AS1: Keyword search",
			scenario: "Given products exist, When I search by keyword, Then I see relevant products",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Setup data
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "PHONE-001", "name": "Smart Phone X", "description": "Latest smart phone",
				})
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "LAPTOP-001", "name": "Pro Laptop", "description": "High performance laptop",
				})

				// Search for "phone"
				reqData := &pb.SearchProductsRequest{
					Query: "phone",
				}
				body, _ := protojson.Marshal(reqData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				searchHandler.Search(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("Expected status OK, got %d", rec.Code)
				}

				var resp pb.SearchProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

				if len(resp.Results) != 1 {
					t.Errorf("Expected 1 product, got %d", len(resp.Results))
				}
				if resp.Results[0].Product.Sku != "PHONE-001" {
					t.Errorf("Expected PHONE-001, got %s", resp.Results[0].Product.Sku)
				}
			},
		},
		{
			name:     "US5-AS2: Multiple filters",
			scenario: "Given products exist, When I filter by price and status, Then I see only matching products",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Setup data
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "P1", "name": "Cheap Active", "base_price": int64(1000), "status": models.ProductStatusActive,
				})
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "P2", "name": "Expensive Active", "base_price": int64(5000), "status": models.ProductStatusActive,
				})
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "P3", "name": "Cheap Draft", "base_price": int64(1000), "status": models.ProductStatusDraft,
				})

				// Filter: Active AND MaxPrice 2000
				reqData := &pb.SearchProductsRequest{
					Statuses: []pb.ProductStatus{pb.ProductStatus_PRODUCT_STATUS_ACTIVE},
					MaxPrice: 2000,
				}
				body, _ := protojson.Marshal(reqData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				searchHandler.Search(rec, req)

				var resp pb.SearchProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

				if len(resp.Results) != 1 {
					t.Errorf("Expected 1 product, got %d", len(resp.Results))
				}
				if resp.Results[0].Product.Sku != "P1" {
					t.Errorf("Expected P1, got %s", resp.Results[0].Product.Sku)
				}
			},
		},
		{
			name:     "US5-AS3: Clear all filters",
			scenario: "Given a filtered list, When I clear filters, Then I see all products",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Setup data
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{"sku": "P1", "name": "Product 1"})
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{"sku": "P2", "name": "Product 2"})

				// Empty request means no filters
				reqData := &pb.SearchProductsRequest{}
				body, _ := protojson.Marshal(reqData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				searchHandler.Search(rec, req)

				var resp pb.SearchProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

				if len(resp.Results) != 2 {
					t.Errorf("Expected 2 products, got %d", len(resp.Results))
				}
			},
		},
		{
			name:     "US5-AS4: Performance under 2s",
			scenario: "Given products exist, When I search, Then response is fast",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Simple check for now, real perf test requires bulk data
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{"sku": "P1", "name": "Product 1"})

				start := time.Now()
				reqData := &pb.SearchProductsRequest{Query: "Product"}
				body, _ := protojson.Marshal(reqData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/search", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				searchHandler.Search(rec, req)

				duration := time.Since(start)
				if duration.Seconds() > 2.0 {
					t.Errorf("Search took too long: %v", duration)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFunc(t)
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
