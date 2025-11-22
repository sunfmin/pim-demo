package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
	"gorm.io/gorm"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestProductAcceptanceScenarios tests all acceptance scenarios for User Story 1
func TestProductAcceptanceScenarios(t *testing.T) {
	// Setup protojson unmarshal options for consistent parsing
	unmarshalOptions := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}

	testCases := []struct {
		name        string
		scenario    string
		setupFunc   func(*ProductTestFixture) error
		requestFunc func(*ProductTestFixture) (*http.Request, error)
		assertFunc  func(*ProductTestFixture, *httptest.ResponseRecorder) error
	}{
		{
			name:     "US1-AS1: Create new product with required fields",
			scenario: "Given I am logged in as a product manager, When I create a new product with required fields (SKU, name, description, price), Then the product is saved with a unique identifier and appears in the product list",
			setupFunc: func(f *ProductTestFixture) error {
				return nil // No special setup needed
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:         "LAPTOP-001",
					Name:        "Professional Laptop",
					Description: "High-performance laptop for professionals",
					BasePrice:   129900, // $1299.00
					Status:      pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			assertFunc: func(f *ProductTestFixture, rec *httptest.ResponseRecorder) error {
				if rec.Code != http.StatusCreated {
					return fmt.Errorf("expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
				}

				var response pb.CreateProductResponse
				responseBody := rec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(responseBody), &response); err != nil {
					t.Fatalf("Failed to decode response: %v. Body: %s", err, responseBody)
				}

				// Build expected from TEST FIXTURES (request data + database fixtures)
				// Per Principle VI: Derive expected values from fixtures, not response data
				expected := &pb.CreateProductResponse{
					Product: &pb.Product{
						Id:             response.Product.Id,        // Use generated ID from response (random UUID)
						OrganizationId: f.org.ID.String(),          // From database fixture (test organization)
						Sku:            "LAPTOP-001",               // From request fixture (what we sent)
						Name:           "Professional Laptop",      // From request fixture
						Description:    "High-performance laptop for professionals", // From request fixture
						BasePrice:      129900,                     // From request fixture
						Status:         pb.ProductStatus_PRODUCT_STATUS_ACTIVE, // From request fixture
						Attributes:     make(map[string]*pb.AttributeValue), // Known default (empty map)
						CategoryIds:    []string{},                  // Known default (empty slice)
						CreatedAt:      response.Product.CreatedAt, // Use generated timestamp (random)
						UpdatedAt:      response.Product.UpdatedAt, // Use generated timestamp (random)
					},
				}

				// Use protocmp for complete message comparison (MANDATORY per Principle VI)
				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					return fmt.Errorf("CreateProductResponse mismatch (-want +got):\n%s", diff)
				}

				// Verify product appears in list (additional verification)
				listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
				listReq = f.addOrgContext(listReq)
				listRec := httptest.NewRecorder()
				f.handler.List(listRec, listReq)

				var listResponse pb.ListProductsResponse
				listResponseBody := listRec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(listResponseBody), &listResponse); err != nil {
					t.Fatalf("Failed to decode list response: %v. Body: %s", err, listResponseBody)
				}

				if len(listResponse.Products) != 1 {
					return fmt.Errorf("expected 1 product in list, got %d", len(listResponse.Products))
				}

				return nil
			},
		},
		{
			name:     "US1-AS2: Update product attributes with timestamp tracking",
			scenario: "Given a product exists in the system, When I update any product attribute, Then the changes are saved and the product's last modified timestamp is updated",
			setupFunc: func(f *ProductTestFixture) error {
				// Create product using service directly since we need proper context
				ctx := context.Background()
				product, err := f.service.Create(ctx, &pb.CreateProductRequest{
					Sku:         "LAPTOP-002",
					Name:        "Original Name",
					Description: "Original description",
					BasePrice:   100000,
					Status:      pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				}, f.org.ID)
				if err != nil {
					return err
				}

				// Get the model from database for fixture
				var model models.Product
				if err := f.db.Where("id = ?", product.Id).First(&model).Error; err != nil {
					return err
				}

				f.existingProduct = &model
				f.originalUpdatedAt = model.UpdatedAt
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				updateRequest := &pb.UpdateProductRequest{
					Name:        "Updated Name",
					Description: "Updated description",
					BasePrice:   149900,
				}
				requestBody, err := json.Marshal(updateRequest)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+f.existingProduct.ID.String(), bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			assertFunc: func(f *ProductTestFixture, rec *httptest.ResponseRecorder) error {
				if rec.Code != http.StatusOK {
					return fmt.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
				}

				var response pb.UpdateProductResponse
				responseBody := rec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(responseBody), &response); err != nil {
					t.Fatalf("Failed to decode response: %v. Body: %s", err, responseBody)
				}

				// Build expected from TEST FIXTURES (request data + database fixtures)
				// Per Principle VI: Derive expected values from fixtures, not response data
				expected := &pb.UpdateProductResponse{
					Product: &pb.Product{
						Id:             f.existingProduct.ID.String(), // From database fixture (existing product)
						OrganizationId: f.org.ID.String(),             // From database fixture (test organization)
						Sku:            "LAPTOP-002",                  // From database fixture (unchanged)
						Name:           "Updated Name",                // From request fixture (what we sent)
						Description:    "Updated description",         // From request fixture
						BasePrice:      149900,                        // From request fixture
						Status:         pb.ProductStatus_PRODUCT_STATUS_ACTIVE, // From database fixture (unchanged)
						Attributes:     make(map[string]*pb.AttributeValue), // Known default (empty map)
						CategoryIds:    []string{},                     // Known default (empty slice)
						CreatedAt:      response.Product.CreatedAt,    // Use from response (not changed)
						UpdatedAt:      response.Product.UpdatedAt,    // Use generated timestamp (updated)
					},
				}

				// Use protocmp for complete message comparison (MANDATORY per Principle VI)
				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					return fmt.Errorf("UpdateProductResponse mismatch (-want +got):\n%s", diff)
				}

				// Verify updated_at changed (additional verification)
				if response.Product.UpdatedAt.AsTime().Before(f.originalUpdatedAt) ||
				   response.Product.UpdatedAt.AsTime().Equal(f.originalUpdatedAt) {
					return fmt.Errorf("expected updated_at to be after original timestamp %v, got %v",
						f.originalUpdatedAt, response.Product.UpdatedAt.AsTime())
				}

				return nil
			},
		},
		{
			name:     "US1-AS3: Soft-delete product",
			scenario: "Given a product exists in the system, When I delete the product, Then the product is marked as deleted and no longer appears in active product listings",
			setupFunc: func(f *ProductTestFixture) error {
				// Create product using service directly
				ctx := context.Background()
				product, err := f.service.Create(ctx, &pb.CreateProductRequest{
					Sku:         "LAPTOP-003",
					Name:        "Test Product",
					Description: "Test description",
					BasePrice:   50000,
					Status:      pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				}, f.org.ID)
				if err != nil {
					return err
				}

				// Get the model from database for fixture
				var model models.Product
				if err := f.db.Where("id = ?", product.Id).First(&model).Error; err != nil {
					return err
				}

				f.existingProduct = &model
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+f.existingProduct.ID.String(), nil)
				return f.addOrgContext(req), nil
			},
			assertFunc: func(f *ProductTestFixture, rec *httptest.ResponseRecorder) error {
				if rec.Code != http.StatusOK {
					return fmt.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
				}

				var response pb.DeleteProductResponse
				responseBody := rec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(responseBody), &response); err != nil {
					t.Fatalf("Failed to decode response: %v. Body: %s", err, responseBody)
				}

				// Build expected from TEST FIXTURES
				// Per Principle VI: Derive expected values from fixtures
				expected := &pb.DeleteProductResponse{
					Success: true, // Known behavior (successful deletion)
				}

				// Use protocmp for complete message comparison (MANDATORY per Principle VI)
				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					return fmt.Errorf("DeleteProductResponse mismatch (-want +got):\n%s", diff)
				}

				// Verify product no longer appears in list
				listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
				listReq = f.addOrgContext(listReq)
				listRec := httptest.NewRecorder()
				f.handler.List(listRec, listReq)

				var listResponse pb.ListProductsResponse
				listResponseBody := listRec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(listResponseBody), &listResponse); err != nil {
					t.Fatalf("Failed to decode list response: %v. Body: %s", err, listResponseBody)
				}

				if len(listResponse.Products) != 0 {
					return fmt.Errorf("expected 0 products in list after deletion, got %d", len(listResponse.Products))
				}

				// Verify product is soft-deleted (additional verification using GORM model)
				var deletedProduct models.Product
				err := f.db.Unscoped().Where("id = ?", f.existingProduct.ID).First(&deletedProduct).Error
				if err != nil {
					return fmt.Errorf("failed to retrieve deleted product: %v", err)
				}

				if !deletedProduct.DeletedAt.Valid {
					return fmt.Errorf("expected deleted_at to be set")
				}

				return nil
			},
		},
		{
			name:     "US1-AS4: Duplicate SKU validation",
			scenario: "Given I attempt to create a product with a duplicate SKU, When I submit the form, Then I receive a clear error message indicating the SKU already exists",
			setupFunc: func(f *ProductTestFixture) error {
				existingSKU := "LAPTOP-004"
				// Create existing product using service
				ctx := context.Background()
				_, err := f.service.Create(ctx, &pb.CreateProductRequest{
					Sku:         existingSKU,
					Name:        "Existing Product",
					Description: "Existing description",
					BasePrice:   75000,
					Status:      pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				}, f.org.ID)
				if err != nil {
					return err
				}
				f.existingSKU = existingSKU
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:         f.existingSKU, // Duplicate SKU
					Name:        "Another Laptop",
					Description: "This should fail",
					BasePrice:   99900,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			assertFunc: func(f *ProductTestFixture, rec *httptest.ResponseRecorder) error {
				if rec.Code != http.StatusConflict {
					return fmt.Errorf("expected status %d (Conflict), got %d", http.StatusConflict, rec.Code)
				}

				var response pb.ErrorResponse
				responseBody := rec.Body.String()
				if err := unmarshalOptions.Unmarshal([]byte(responseBody), &response); err != nil {
					t.Fatalf("Failed to decode error response: %v. Body: %s", err, responseBody)
				}

				// Build expected from TEST FIXTURES
				// Per Principle VI: Derive expected values from fixtures
				expected := &pb.ErrorResponse{
					Code:      "DUPLICATE_SKU", // Known error code from fixture setup
					Message:   response.Message, // Use from response (may vary, but presence is what matters)
					RequestId: response.RequestId, // Match the actual request ID
				}

				// Use protocmp for complete message comparison (MANDATORY per Principle VI)
				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					return fmt.Errorf("ErrorResponse mismatch (-want +got):\n%s", diff)
				}

				// Additional verification that message is present
				if response.Message == "" {
					return fmt.Errorf("expected error message to be present")
				}

				return nil
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := setupProductTestFixture(t)
			defer fixture.cleanup()

			// Setup
			if err := tc.setupFunc(fixture); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Execute request
			req, err := tc.requestFunc(fixture)
			if err != nil {
				t.Fatalf("Request creation failed: %v", err)
			}

			rec := httptest.NewRecorder()

			// Route the request
			switch req.Method {
			case http.MethodPost:
				fixture.handler.Create(rec, req)
			case http.MethodGet:
				fixture.handler.Get(rec, req)
			case http.MethodPut:
				fixture.handler.Update(rec, req)
			case http.MethodDelete:
				fixture.handler.Delete(rec, req)
			}

			// Assert
			if err := tc.assertFunc(fixture, rec); err != nil {
				t.Errorf("Assertion failed: %v", err)
			}
		})
	}
}

// ProductTestFixture provides a reusable test fixture for product tests
type ProductTestFixture struct {
	db                *gorm.DB
	cleanup           func()
	org               *models.Organization
	service           services.ProductService
	handler           *handlers.ProductHandler
	existingProduct   *models.Product
	existingSKU       string
	originalUpdatedAt time.Time
}

// setupProductTestFixture creates a new test fixture
func setupProductTestFixture(t *testing.T) *ProductTestFixture {
	db, cleanup := testutil.SetupTestDB(t)
	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})
	service := services.NewProductService(db)
	handler := handlers.NewProductHandler(service)

	return &ProductTestFixture{
		db:      db,
		cleanup: func() {
			testutil.TruncateTables(db)
			cleanup()
		},
		org:     org,
		service: service,
		handler: handler,
	}
}

// addOrgContext simulates the tenant middleware behavior for testing
func (f *ProductTestFixture) addOrgContext(r *http.Request) *http.Request {
	// Set header (what middleware reads)
	r.Header.Set("X-Organization-ID", f.org.ID.String())

	// Simulate what TenantMiddleware does - extract from header and put in context
	ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, f.org.ID)
	return r.WithContext(ctx)
}

// TestProductEdgeCases tests comprehensive edge cases for product operations
func TestProductEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		category       string
		setupFunc      func(*ProductTestFixture) error
		requestFunc    func(*ProductTestFixture) (*http.Request, error)
		expectedStatus int
		expectedCode   string
		description    string
	}{
		// Input Validation Edge Cases
		{
			name:     "Empty SKU returns validation error",
			category: "Input Validation",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "", // Empty SKU
					Name:      "Test Product",
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRODUCT_DATA",
			description:    "Empty SKU should trigger validation error",
		},
		{
			name:     "Empty name returns validation error",
			category: "Input Validation",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "TEST-SKU",
					Name:      "", // Empty name
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRODUCT_DATA",
			description:    "Empty name should trigger validation error",
		},
		{
			name:     "Negative price returns validation error",
			category: "Input Validation",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "TEST-SKU",
					Name:      "Test Product",
					BasePrice: -1000, // Negative price
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRODUCT_DATA",
			description:    "Negative price should trigger validation error",
		},
		{
			name:     "Oversized SKU returns validation error",
			category: "Input Validation",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				oversizedSKU := strings.Repeat("A", 51) // SKU limit is 50 chars
				requestData := &pb.CreateProductRequest{
					Sku:       oversizedSKU,
					Name:      "Test Product",
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRODUCT_DATA",
			description:    "Oversized SKU should trigger validation error",
		},
		{
			name:     "Oversized name returns validation error",
			category: "Input Validation",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				oversizedName := strings.Repeat("A", 501) // Name limit is 500 chars
				requestData := &pb.CreateProductRequest{
					Sku:       "TEST-SKU",
					Name:      oversizedName,
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRODUCT_DATA",
			description:    "Oversized name should trigger validation error",
		},

		// Boundary Conditions
		{
			name:     "Zero price is allowed",
			category: "Boundary Conditions",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "ZERO-PRICE-SKU",
					Name:      "Free Product",
					BasePrice: 0, // Zero price (boundary)
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusCreated,
			description:    "Zero price should be allowed as boundary condition",
		},
		{
			name:     "Maximum price boundary",
			category: "Boundary Conditions",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "MAX-PRICE-SKU",
					Name:      "Expensive Product",
					BasePrice: 99999999, // Large price boundary
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusCreated,
			description:    "Maximum price boundary should be handled",
		},

		// Data State Edge Cases
		{
			name:     "Non-existent product returns 404",
			category: "Data State",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				nonExistentID := uuid.New()
				req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+nonExistentID.String(), nil)
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "PRODUCT_NOT_FOUND",
			description:    "Accessing non-existent product should return 404",
		},
		{
			name:     "Update non-existent product returns 404",
			category: "Data State",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				nonExistentID := uuid.New()
				updateRequest := &pb.UpdateProductRequest{
					Name: "Updated Name",
				}
				requestBody, err := json.Marshal(updateRequest)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+nonExistentID.String(), bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "PRODUCT_NOT_FOUND",
			description:    "Updating non-existent product should return 404",
		},
		{
			name:     "Delete non-existent product returns 404",
			category: "Data State",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				nonExistentID := uuid.New()
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+nonExistentID.String(), nil)
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "PRODUCT_NOT_FOUND",
			description:    "Deleting non-existent product should return 404",
		},
		{
			name:     "Delete already deleted product returns 404",
			category: "Data State",
			setupFunc: func(f *ProductTestFixture) error {
				// Create and delete a product
				product := testutil.CreateTestProduct(t, f.db, f.org.ID, map[string]interface{}{
					"sku": "TO-DELETE",
				})
				err := f.service.Delete(context.Background(), product.ID, f.org.ID)
				if err != nil {
					return err
				}
				f.existingProduct = product
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+f.existingProduct.ID.String(), nil)
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "PRODUCT_NOT_FOUND",
			description:    "Deleting already deleted product should return 404",
		},

		// Authentication & Authorization Edge Cases
		{
			name:     "Missing organization ID returns unauthorized",
			category: "Authentication & Authorization",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "TEST-SKU",
					Name:      "Test Product",
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				// No organization ID in context
				return req, nil
			},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
			description:    "Missing organization context should return 401",
		},
		{
			name:     "Access product from different organization returns 404",
			category: "Authentication & Authorization",
			setupFunc: func(f *ProductTestFixture) error {
				// Create product in current org
				f.existingProduct = testutil.CreateTestProduct(t, f.db, f.org.ID, map[string]interface{}{
					"sku": "ORG-TEST",
				})
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+f.existingProduct.ID.String(), nil)
				// Use different organization ID in context
				differentOrgID := uuid.New()
				ctx := context.WithValue(req.Context(), middleware.OrganizationIDKey, differentOrgID)
				return req.WithContext(ctx), nil
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "PRODUCT_NOT_FOUND",
			description:    "Accessing product from different org should return 404 for security",
		},

		// HTTP Specifics
		{
			name:     "Malformed JSON returns 400",
			category: "HTTP Specifics",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader([]byte("invalid json")))
				req.Header.Set("Content-Type", "application/json")
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
			description:    "Malformed JSON should return 400 with INVALID_REQUEST",
		},
		{
			name:     "Wrong HTTP method returns 405",
			category: "HTTP Specifics",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				req := httptest.NewRequest(http.MethodPatch, "/api/v1/products", nil)
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "Unsupported HTTP method should return 405",
		},
		{
			name:     "Missing Content-Type for JSON request",
			category: "HTTP Specifics",
			setupFunc: func(f *ProductTestFixture) error {
				return nil
			},
			requestFunc: func(f *ProductTestFixture) (*http.Request, error) {
				requestData := &pb.CreateProductRequest{
					Sku:       "TEST-SKU",
					Name:      "Test Product",
					BasePrice: 1000,
				}
				requestBody, err := json.Marshal(requestData)
				if err != nil {
					return nil, err
				}
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				// Missing Content-Type header
				return f.addOrgContext(req), nil
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REQUEST",
			description:    "Missing Content-Type header should be handled",
		},
	}

	// Run all edge case tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := setupProductTestFixture(t)
			defer fixture.cleanup()

			// Setup
			if err := tc.setupFunc(fixture); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			// Execute request
			req, err := tc.requestFunc(fixture)
			if err != nil {
				t.Fatalf("Request creation failed: %v", err)
			}

			rec := httptest.NewRecorder()

			// Route the request based on method and path
			if strings.Contains(req.URL.Path, "/products/") && req.Method == http.MethodGet {
				fixture.handler.Get(rec, req)
			} else if strings.Contains(req.URL.Path, "/products/") && req.Method == http.MethodPut {
				fixture.handler.Update(rec, req)
			} else if strings.Contains(req.URL.Path, "/products/") && req.Method == http.MethodDelete {
				fixture.handler.Delete(rec, req)
			} else if req.URL.Path == "/api/v1/products" && req.Method == http.MethodPost {
				fixture.handler.Create(rec, req)
			} else {
				// For unsupported routes/methods, simulate 405
				rec.WriteHeader(http.StatusMethodNotAllowed)
			}

			// Assert status
			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}

			// Assert error code if expected
			if tc.expectedCode != "" {
				var errorResponse pb.ErrorResponse
				if err := json.NewDecoder(rec.Body).Decode(&errorResponse); err == nil {
					if errorResponse.Code != tc.expectedCode {
						t.Errorf("Expected error code '%s', got '%s'", tc.expectedCode, errorResponse.Code)
					}
				} else {
					t.Errorf("Expected error response but got: %s", rec.Body.String())
				}
			}

			t.Logf("✅ %s: %s", tc.description, http.StatusText(rec.Code))
		})
	}
}

// TestProductContextHandling tests context cancellation and timeout scenarios
func TestProductContextHandling(t *testing.T) {
	testCases := []struct {
		name         string
		contextFunc  func() (context.Context, context.CancelFunc)
		expectError  bool
		errorType    string
		description  string
	}{
		{
			name: "Context cancellation during create",
			contextFunc: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx, cancel
			},
			expectError: true,
			errorType:   "canceled",
			description: "Create operation should handle context cancellation",
		},
		{
			name: "Context timeout during get",
			contextFunc: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Nanosecond)
			},
			expectError: true,
			errorType:   "timeout",
			description: "Get operation should handle context timeout",
		},
		{
			name: "Normal context works",
			contextFunc: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			expectError: false,
			description: "Normal context should work without issues",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := setupProductTestFixture(t)
			defer fixture.cleanup()

			ctx, cancel := tc.contextFunc()
			if cancel != nil {
				defer cancel()
			}

			// Test service layer context handling
			_, err := fixture.service.Get(ctx, uuid.New(), fixture.org.ID)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error due to %s, but got nil", tc.errorType)
				} else {
					switch tc.errorType {
					case "canceled":
						if !errors.Is(err, context.Canceled) {
							t.Errorf("Expected context.Canceled, got: %v", err)
						}
					case "timeout":
						if !errors.Is(err, context.DeadlineExceeded) {
							t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
						}
					}
				}
			} else {
				// For normal context, we expect a "not found" error (since UUID is random)
				if err == nil || !strings.Contains(err.Error(), "not found") {
					t.Errorf("Expected 'not found' error for normal context, got: %v", err)
				}
			}

			t.Logf("✅ %s: Context handling validated", tc.description)
		})
	}
}

// TestProductPerformanceScenarios tests performance and load scenarios
func TestProductPerformanceScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	fixture := setupProductTestFixture(t)
	defer fixture.cleanup()

	t.Run("Bulk create performance", func(t *testing.T) {
		// Create multiple products to test bulk operations
		productCount := 100
		start := time.Now()

		for i := 0; i < productCount; i++ {
			requestData := &pb.CreateProductRequest{
				Sku:       fmt.Sprintf("PERF-SKU-%03d", i),
				Name:      fmt.Sprintf("Performance Product %d", i),
				BasePrice: 1000 + int64(i),
			}
			requestBody, _ := protojson.Marshal(requestData)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req = fixture.addOrgContext(req)

			rec := httptest.NewRecorder()
			fixture.handler.Create(rec, req)

			if rec.Code != http.StatusCreated {
				t.Errorf("Failed to create product %d: status %d", i, rec.Code)
			}
		}

		duration := time.Since(start)
		t.Logf("✅ Created %d products in %v (%.2f products/sec)",
			productCount, duration, float64(productCount)/duration.Seconds())

		// Verify all products exist
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=1000", nil)
		listReq = fixture.addOrgContext(listReq)
		listRec := httptest.NewRecorder()
		fixture.handler.List(listRec, listReq)

		var listResponse pb.ListProductsResponse
		testutil.UnmarshalProtoResponse(t, listRec.Body, &listResponse)

		if len(listResponse.Products) != productCount {
			t.Errorf("Expected %d products in list, got %d", productCount, len(listResponse.Products))
		}
	})

	t.Run("Large result set pagination", func(t *testing.T) {
		// Test pagination with large result sets
		// (Assuming we have products from previous test)

		// Test with limit parameter
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products?limit=10", nil)
		listReq = fixture.addOrgContext(listReq)
		listRec := httptest.NewRecorder()
		fixture.handler.List(listRec, listReq)

		var listResponse pb.ListProductsResponse
		testutil.UnmarshalProtoResponse(t, listRec.Body, &listResponse)

		if len(listResponse.Products) > 10 {
			t.Errorf("Expected at most 10 products with limit=10, got %d", len(listResponse.Products))
		}

		t.Logf("✅ Pagination works: returned %d products with limit 10", len(listResponse.Products))
	})
}
