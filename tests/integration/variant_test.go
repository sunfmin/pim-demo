package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestVariantAcceptanceScenarios tests all acceptance scenarios for User Story 3
// Covers US3-AS1 through US3-AS4 from spec.md
func TestVariantAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Setup services and handlers
	productService := services.NewProductService(db)
	variantService := services.NewVariantService(db)
	variantHandler := handlers.NewVariantHandler(variantService, productService)

	// Create test organization and parent product
	tests := []struct {
		name     string
		testFunc func(t *testing.T, db *gorm.DB, org *models.Organization, parent *models.Product)
	}{
		{
			name: "US3-AS1: Generate variants from attributes",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, parent *models.Product) {
				// Given: A parent product exists
				// When: Product manager defines variant attributes (Size: S/M/L, Color: Red/Blue)
				req := &pb.GenerateVariantsRequest{
					ParentProductId: parent.ID.String(),
					Attributes: []*pb.VariantAttributeDefinition{
						{
							AttributeName:  "size",
							PossibleValues: []string{"S", "M", "L"},
						},
						{
							AttributeName:  "color",
							PossibleValues: []string{"Red", "Blue"},
						},
					},
					SkuPattern:             "{parent_sku}-{size}-{color}",
					DefaultPriceAdjustment: 0,
				}

				reqBody, _ := protojson.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/variants/generate", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				variantHandler.Generate(w, httpReq)

				// Then: 6 variants are generated (3 sizes × 2 colors)
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.GenerateVariantsResponse
				testutil.UnmarshalProtoResponse(t, w.Body, &resp)

				// Verify 6 variants created
				if len(resp.Variants) != 6 {
					t.Errorf("Expected 6 variants, got %d", len(resp.Variants))
				}

				// Verify SKU pattern applied
				expectedSKUs := []string{
					"TSHIRT-001-S-Red", "TSHIRT-001-S-Blue",
					"TSHIRT-001-M-Red", "TSHIRT-001-M-Blue",
					"TSHIRT-001-L-Red", "TSHIRT-001-L-Blue",
				}

				gotSKUs := make(map[string]bool)
				for _, v := range resp.Variants {
					gotSKUs[v.VariantSku] = true
				}

				for _, expectedSKU := range expectedSKUs {
					if !gotSKUs[expectedSKU] {
						t.Errorf("Expected variant SKU %s not found", expectedSKU)
					}
				}

				// Verify variant attributes stored correctly
				for _, v := range resp.Variants {
					if v.VariantAttributes == nil {
						t.Error("Variant attributes should not be nil")
						continue
					}
					// Each variant should have size and color attributes
					if _, ok := v.VariantAttributes["size"]; !ok {
						t.Errorf("Variant %s missing size attribute", v.VariantSku)
					}
					if _, ok := v.VariantAttributes["color"]; !ok {
						t.Errorf("Variant %s missing color attribute", v.VariantSku)
					}
				}
			},
		},
		{
			name: "US3-AS2: Parent update cascades to variants",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, parent *models.Product) {
				// Given: Product has variants
				variantAttrs, _ := json.Marshal(map[string]string{
					"size":  "M",
					"color": "Red",
				})
				variant := testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "TSHIRT-001-M-RED-V1",
					"variant_attributes": datatypes.JSON(variantAttrs),
					"price_adjustment":   int64(0),
					"inventory_quantity": int32(100),
				})

				// When: Shared attribute (description) is updated on parent
				updateReq := &pb.UpdateProductRequest{
					Id:          parent.ID.String(),
					Description: "Premium quality cotton t-shirt - updated",
				}

				reqBody, _ := protojson.Marshal(updateReq)
				httpReq := httptest.NewRequest("PUT", "/api/v1/products/"+parent.ID.String(), bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				productHandler := handlers.NewProductHandler(productService)
				w := httptest.NewRecorder()
				productHandler.Update(w, httpReq)

				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				// Then: Variant inherits updated description
				// Verify by fetching variant and checking it inherits parent description
				var updatedVariant models.ProductVariant
				err := db.Preload("ParentProduct").First(&updatedVariant, "id = ?", variant.ID).Error
				if err != nil {
					t.Fatalf("Failed to fetch variant: %v", err)
				}

				if updatedVariant.ParentProduct.Description != "Premium quality cotton t-shirt - updated" {
					t.Errorf("Expected parent description to be updated, got %s", updatedVariant.ParentProduct.Description)
				}
			},
		},
		{
			name: "US3-AS3: Variant-specific update",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, parent *models.Product) {
				// Given: Product has variants
				variantAttrs, _ := json.Marshal(map[string]string{
					"size":  "L",
					"color": "Blue",
				})
				variant := testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "TSHIRT-001-L-BLUE-V1",
					"variant_attributes": datatypes.JSON(variantAttrs),
					"price_adjustment":   int64(0),
					"inventory_quantity": int32(50),
				})

				// When: Variant-specific attribute is updated (inventory, price adjustment)
				updateReq := &pb.UpdateVariantRequest{
					Id:                variant.ID.String(),
					PriceAdjustment:   200, // +$2.00
					InventoryQuantity: 75,
				}

				reqBody, _ := protojson.Marshal(updateReq)
				httpReq := httptest.NewRequest("PUT", "/api/v1/variants/"+variant.ID.String(), bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				variantHandler.Update(w, httpReq)

				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.UpdateVariantResponse
				testutil.UnmarshalProtoResponse(t, w.Body, &resp)

				// Then: Only variant is updated, parent unchanged
				expected := &pb.ProductVariant{
					Id:                variant.ID.String(),
					OrganizationId:    org.ID.String(),
					ParentProductId:   parent.ID.String(),
					VariantSku:        "TSHIRT-001-L-BLUE-V1",
					PriceAdjustment:   200,
					InventoryQuantity: 75,
					IsActive:          true,
				}

				// Compare relevant fields (ignore timestamps, CreatedAt, UpdatedAt)
				if resp.Variant.PriceAdjustment != expected.PriceAdjustment {
					t.Errorf("PriceAdjustment mismatch: expected %d, got %d", expected.PriceAdjustment, resp.Variant.PriceAdjustment)
				}
				if resp.Variant.InventoryQuantity != expected.InventoryQuantity {
					t.Errorf("InventoryQuantity mismatch: expected %d, got %d", expected.InventoryQuantity, resp.Variant.InventoryQuantity)
				}

				// Verify parent product price unchanged
				var parentProduct models.Product
				db.First(&parentProduct, "id = ?", parent.ID)
				if parentProduct.BasePrice != 1999 {
					t.Errorf("Parent base price should remain 1999, got %d", parentProduct.BasePrice)
				}
			},
		},
		{
			name: "US3-AS4: View all variants for product",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, parent *models.Product) {
				// Given: Product has multiple variants
				variantAttrs1, _ := json.Marshal(map[string]string{
					"size":  "S",
					"color": "Red",
				})
				variant1 := testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "TSHIRT-001-S-RED-V2",
					"variant_attributes": datatypes.JSON(variantAttrs1),
					"price_adjustment":   int64(0),
					"inventory_quantity": int32(30),
				})

				variantAttrs2, _ := json.Marshal(map[string]string{
					"size":  "M",
					"color": "Red",
				})
				variant2 := testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "TSHIRT-001-M-RED-V2",
					"variant_attributes": datatypes.JSON(variantAttrs2),
					"price_adjustment":   int64(100),
					"inventory_quantity": int32(40),
				})

				// When: Product manager requests all variants for product
				httpReq := httptest.NewRequest("GET", "/api/v1/products/"+parent.ID.String()+"/variants", nil)
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				variantHandler.ListByProduct(w, httpReq)

				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.ListVariantsResponse
				testutil.UnmarshalProtoResponse(t, w.Body, &resp)

				// Then: All variants displayed with attributes and inventory
				if len(resp.Variants) < 2 {
					t.Errorf("Expected at least 2 variants, got %d", len(resp.Variants))
				}

				// Verify both variants present
				foundVariants := make(map[string]bool)
				for _, v := range resp.Variants {
					foundVariants[v.Id] = true
				}

				if !foundVariants[variant1.ID.String()] {
					t.Errorf("Variant %s not found in list", variant1.ID)
				}
				if !foundVariants[variant2.ID.String()] {
					t.Errorf("Variant %s not found in list", variant2.ID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Truncate tables before each test for isolation
			defer testutil.TruncateTables(db)

			// Recreate org and parent product for each test
			org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
				"name": "Test Org " + tt.name,
			})
			parentProduct := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
				"sku":         "TSHIRT-001",
				"name":        "Basic T-Shirt",
				"description": "Plain cotton t-shirt",
				"base_price":  int64(1999),
				"status":      models.ProductStatusActive,
			})

			tt.testFunc(t, db, org, parentProduct)
		})
	}
}

// TestVariantEdgeCases tests edge cases for variant functionality
func TestVariantEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	variantService := services.NewVariantService(db)
	productService := services.NewProductService(db)
	variantHandler := handlers.NewVariantHandler(variantService, productService)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Test Org Edge Cases",
	})

	tests := []struct {
		name           string
		setup          func(t *testing.T) (context.Context, *http.Request)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Invalid parent_id",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				req := &pb.CreateVariantRequest{
					ParentProductId: uuid.New().String(), // Non-existent parent
					VariantSku:      "INVALID-001",
					VariantAttributes: map[string]*pb.AttributeValue{
						"size": {Value: &pb.AttributeValue_StringValue{StringValue: "M"}},
					},
				}
				reqBody, _ := protojson.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/variants", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "PRODUCT_NOT_FOUND",
		},
		{
			name: "Duplicate variant SKU",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				parent := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":        "PARENT-001",
					"name":       "Parent Product",
					"base_price": int64(1000),
				})

				// Create first variant
				variantAttrs, _ := json.Marshal(map[string]string{
					"size": "M",
				})
				testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "DUP-SKU-001",
					"variant_attributes": datatypes.JSON(variantAttrs),
				})

				// Try to create second variant with same SKU
				req := &pb.CreateVariantRequest{
					ParentProductId: parent.ID.String(),
					VariantSku:      "DUP-SKU-001",
					VariantAttributes: map[string]*pb.AttributeValue{
						"size": {Value: &pb.AttributeValue_StringValue{StringValue: "L"}},
					},
				}
				reqBody, _ := protojson.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/variants", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "DUPLICATE_SKU",
		},
		{
			name: "Negative price adjustment",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				parent := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":        "PARENT-002",
					"name":       "Parent Product",
					"base_price": int64(1000),
				})

				req := &pb.CreateVariantRequest{
					ParentProductId: parent.ID.String(),
					VariantSku:      "NEG-PRICE-001",
					VariantAttributes: map[string]*pb.AttributeValue{
						"size": {Value: &pb.AttributeValue_StringValue{StringValue: "S"}},
					},
					PriceAdjustment: -500, // -$5.00 discount
				}
				reqBody, _ := protojson.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/variants", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusCreated, // Negative price adjustments are allowed
		},
		{
			name: "Delete parent deletes variants (cascade)",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				parent := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":        "CASCADE-PARENT",
					"name":       "Parent to Delete",
					"base_price": int64(1000),
				})

				variantAttrs, _ := json.Marshal(map[string]string{
					"size": "M",
				})
				variant := testutil.CreateTestVariant(t, db, parent.ID, org.ID, map[string]interface{}{
					"variant_sku":        "CASCADE-VARIANT",
					"variant_attributes": datatypes.JSON(variantAttrs),
				})

				// Delete parent
				httpReq := httptest.NewRequest("DELETE", "/api/v1/products/"+parent.ID.String(), nil)
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)

				productHandler := handlers.NewProductHandler(productService)
				w := httptest.NewRecorder()
				productHandler.Delete(w, httpReq.WithContext(ctx))

				// Verify variant also deleted
				var count int64
				db.Model(&models.ProductVariant{}).Where("id = ?", variant.ID).Count(&count)
				if count != 0 {
					t.Error("Variant should be deleted when parent is deleted")
				}

				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Empty variant attributes",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				parent := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":        "PARENT-003",
					"name":       "Parent Product",
					"base_price": int64(1000),
				})

				req := &pb.CreateVariantRequest{
					ParentProductId:   parent.ID.String(),
					VariantSku:        "EMPTY-ATTR-001",
					VariantAttributes: map[string]*pb.AttributeValue{}, // Empty attributes
				}
				reqBody, _ := protojson.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/variants", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_VARIANT_DATA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpReq := tt.setup(t)

			w := httptest.NewRecorder()

			// Route to appropriate handler based on method
			switch httpReq.Method {
			case "POST":
				if httpReq.URL.Path == "/api/v1/variants" {
					variantHandler.Create(w, httpReq)
				}
			case "DELETE":
				// Handled in setup
			}

			if tt.expectedStatus > 0 && w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedError != "" && w.Code >= 400 {
				var errResp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &errResp)
				if errCode, ok := errResp["code"].(string); !ok || errCode != tt.expectedError {
					t.Errorf("Expected error code %s, got %v", tt.expectedError, errResp)
				}
			}
		})
	}
}
