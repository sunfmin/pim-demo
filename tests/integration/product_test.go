package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestProductAcceptanceScenarios tests all acceptance scenarios for User Story 1
func TestProductAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Create test organization
	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

	// Create service and handler
	productService := services.NewProductService(db)
	productHandler := handlers.NewProductHandler(productService)

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
			name:     "US1-AS1: Create new product with required fields",
			scenario: "Given I am logged in as a product manager, When I create a new product with required fields (SKU, name, description, price), Then the product is saved with a unique identifier and appears in the product list",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)

				// Prepare request
				requestData := &pb.CreateProductRequest{
					Sku:         "LAPTOP-001",
					Name:        "Professional Laptop",
					Description: "High-performance laptop for professionals",
					BasePrice:   129900, // $1299.00
					Status:      pb.ProductStatus_PRODUCT_STATUS_ACTIVE,
				}

				requestBody, _ := protojson.Marshal(requestData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, org.ID)

				// Execute request
				rec := httptest.NewRecorder()
				productHandler.Create(rec, req)

				// Verify response
				if rec.Code != http.StatusCreated {
					t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
				}

				var response pb.CreateProductResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &response)

				// Build expected from fixtures (request data)
				expected := &pb.CreateProductResponse{
					Product: &pb.Product{
						Id:             response.Product.Id,                 // Use generated ID (random)
						OrganizationId: org.ID.String(),                     // From test fixture
						Sku:            requestData.Sku,                     // From request fixture
						Name:           requestData.Name,                    // From request fixture
						Description:    requestData.Description,             // From request fixture
						BasePrice:      requestData.BasePrice,               // From request fixture
						Status:         requestData.Status,                  // From request fixture
						Attributes:     make(map[string]*pb.AttributeValue), // Empty map for MVP
						CategoryIds:    []string{},                          // Empty for MVP
						CreatedAt:      response.Product.CreatedAt,          // Use generated timestamp (random)
						UpdatedAt:      response.Product.UpdatedAt,          // Use generated timestamp (random)
					},
				}

				// Compare using protocmp
				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					t.Errorf("Response mismatch (-want +got):\n%s", diff)
				}

				// Verify product appears in list
				listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
				listReq = addOrgContext(listReq, org.ID)
				listRec := httptest.NewRecorder()
				productHandler.List(listRec, listReq)

				var listResponse pb.ListProductsResponse
				testutil.UnmarshalProtoResponse(t, listRec.Body, &listResponse)

				if len(listResponse.Products) != 1 {
					t.Errorf("Expected 1 product in list, got %d", len(listResponse.Products))
				}
			},
		},
		{
			name:     "US1-AS2: Update product attributes with timestamp tracking",
			scenario: "Given a product exists in the system, When I update any product attribute, Then the changes are saved and the product's last modified timestamp is updated",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)

				// Recreate organization for this test
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create existing product
				existingProduct := testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku":  "LAPTOP-002",
					"name": "Original Name",
				})

				originalUpdatedAt := existingProduct.UpdatedAt

				// Prepare update request
				updateRequest := &pb.UpdateProductRequest{
					Name:        "Updated Name",
					Description: "Updated description",
					BasePrice:   149900,
				}

				requestBody, _ := protojson.Marshal(updateRequest)
				req := httptest.NewRequest(http.MethodPut, "/api/v1/products/"+existingProduct.ID.String(), bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				// Execute request
				rec := httptest.NewRecorder()
				productHandler.Update(rec, req)

				// Verify response
				if rec.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
				}

				var response pb.UpdateProductResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &response)

				// Build expected from fixtures
				// Convert model status to proto status
				var protoStatus pb.ProductStatus
				switch existingProduct.Status {
				case models.ProductStatusActive:
					protoStatus = pb.ProductStatus_PRODUCT_STATUS_ACTIVE
				case models.ProductStatusDiscontinued:
					protoStatus = pb.ProductStatus_PRODUCT_STATUS_DISCONTINUED
				default:
					protoStatus = pb.ProductStatus_PRODUCT_STATUS_DRAFT
				}

				expected := &pb.UpdateProductResponse{
					Product: &pb.Product{
						Id:             existingProduct.ID.String(), // From database fixture
						OrganizationId: testOrg.ID.String(),         // From test fixture
						Sku:            existingProduct.SKU,         // From database fixture (unchanged)
						Name:           updateRequest.Name,          // From request fixture
						Description:    updateRequest.Description,   // From request fixture
						BasePrice:      updateRequest.BasePrice,     // From request fixture
						Status:         protoStatus,                 // From database fixture (unchanged)
						Attributes:     make(map[string]*pb.AttributeValue),
						CategoryIds:    []string{},
						CreatedAt:      response.Product.CreatedAt, // Use from response (not changed)
						UpdatedAt:      response.Product.UpdatedAt, // Use from response (generated)
					},
				}

				if diff := cmp.Diff(expected, &response, protocmp.Transform()); diff != "" {
					t.Errorf("Response mismatch (-want +got):\n%s", diff)
				}

				// Verify updated_at changed
				if response.Product.UpdatedAt.AsTime().Before(originalUpdatedAt) || response.Product.UpdatedAt.AsTime().Equal(originalUpdatedAt) {
					t.Error("Expected updated_at to be after original timestamp")
				}
			},
		},
		{
			name:     "US1-AS3: Soft-delete product",
			scenario: "Given a product exists in the system, When I delete the product, Then the product is marked as deleted and no longer appears in active product listings",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)

				// Recreate organization for this test
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create existing product
				existingProduct := testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": "LAPTOP-003",
				})

				// Delete product
				req := httptest.NewRequest(http.MethodDelete, "/api/v1/products/"+existingProduct.ID.String(), nil)
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				productHandler.Delete(rec, req)

				// Verify response
				if rec.Code != http.StatusOK {
					t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
				}

				var response pb.DeleteProductResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &response)

				if !response.Success {
					t.Error("Expected success to be true")
				}

				// Verify product no longer appears in list
				listReq := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
				listReq = addOrgContext(listReq, testOrg.ID)
				listRec := httptest.NewRecorder()
				productHandler.List(listRec, listReq)

				var listResponse pb.ListProductsResponse
				testutil.UnmarshalProtoResponse(t, listRec.Body, &listResponse)

				if len(listResponse.Products) != 0 {
					t.Errorf("Expected 0 products in list after deletion, got %d", len(listResponse.Products))
				}

				// Verify product is soft-deleted (deleted_at is set)
				var deletedProduct models.Product
				err := db.Unscoped().Where("id = ?", existingProduct.ID).First(&deletedProduct).Error
				if err != nil {
					t.Fatalf("Failed to retrieve deleted product: %v", err)
				}

				if !deletedProduct.DeletedAt.Valid {
					t.Error("Expected deleted_at to be set")
				}
			},
		},
		{
			name:     "US1-AS4: Duplicate SKU validation",
			scenario: "Given I attempt to create a product with a duplicate SKU, When I submit the form, Then I receive a clear error message indicating the SKU already exists",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)

				// Recreate organization for this test
				testOrg := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create existing product with SKU
				existingSKU := "LAPTOP-004"
				testutil.CreateTestProduct(t, db, testOrg.ID, map[string]interface{}{
					"sku": existingSKU,
				})

				// Attempt to create product with duplicate SKU
				requestData := &pb.CreateProductRequest{
					Sku:         existingSKU, // Duplicate SKU
					Name:        "Another Laptop",
					Description: "This should fail",
					BasePrice:   99900,
				}

				requestBody, _ := protojson.Marshal(requestData)
				req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				req = addOrgContext(req, testOrg.ID)

				rec := httptest.NewRecorder()
				productHandler.Create(rec, req)

				// Verify error response
				if rec.Code != http.StatusConflict {
					t.Errorf("Expected status %d (Conflict), got %d", http.StatusConflict, rec.Code)
				}

				var errorResponse pb.ErrorResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &errorResponse)

				if errorResponse.Code != "DUPLICATE_SKU" {
					t.Errorf("Expected error code 'DUPLICATE_SKU', got '%s'", errorResponse.Code)
				}

				if errorResponse.Message == "" {
					t.Error("Expected error message to be present")
				}
			},
		},
	}

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFunc(t)
		})
	}
}

// TestProductEdgeCases tests edge cases for product operations
func TestProductEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})
	productService := services.NewProductService(db)
	productHandler := handlers.NewProductHandler(productService)

	t.Run("Empty SKU returns validation error", func(t *testing.T) {
		requestData := &pb.CreateProductRequest{
			Sku:       "", // Empty SKU
			Name:      "Test Product",
			BasePrice: 1000,
		}

		requestBody, _ := json.Marshal(requestData)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
		ctx := context.WithValue(req.Context(), middleware.OrganizationIDKey, org.ID)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		productHandler.Create(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("Empty name returns validation error", func(t *testing.T) {
		requestData := &pb.CreateProductRequest{
			Sku:       "TEST-SKU",
			Name:      "", // Empty name
			BasePrice: 1000,
		}

		requestBody, _ := json.Marshal(requestData)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
		ctx := context.WithValue(req.Context(), middleware.OrganizationIDKey, org.ID)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		productHandler.Create(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("Non-existent product returns 404", func(t *testing.T) {
		nonExistentID := uuid.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products/"+nonExistentID.String(), nil)
		ctx := context.WithValue(req.Context(), middleware.OrganizationIDKey, org.ID)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		productHandler.Get(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("Missing organization ID returns unauthorized", func(t *testing.T) {
		requestData := &pb.CreateProductRequest{
			Sku:       "TEST-SKU",
			Name:      "Test Product",
			BasePrice: 1000,
		}

		requestBody, _ := json.Marshal(requestData)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/products", bytes.NewReader(requestBody))
		// No X-Organization-ID header

		rec := httptest.NewRecorder()
		productHandler.Create(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}
