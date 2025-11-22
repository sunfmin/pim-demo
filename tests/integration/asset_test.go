package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestAssetAcceptanceScenarios tests all acceptance scenarios for User Story 4
// Covers US4-AS1 through US4-AS4 from spec.md
func TestAssetAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Create test storage directory
	storageDir := filepath.Join(os.TempDir(), "pim-test-assets-"+uuid.New().String())
	defer os.RemoveAll(storageDir)

	// Setup services and handlers
	assetService := services.NewAssetService(db, storageDir)
	productService := services.NewProductService(db)
	assetHandler := handlers.NewAssetHandler(assetService, productService)

	// Create test organization and product
	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Test Org",
	})
	_ = testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
		"sku":  "LAPTOP-001",
		"name": "Test Laptop",
	})

	tests := []struct {
		name     string
		testFunc func(t *testing.T, db *gorm.DB, org *models.Organization, product *models.Product)
	}{
		{
			name: "US4-AS1: Upload image and associate with product",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, product *models.Product) {
				// Given: A product exists
				// When: Product manager uploads an image
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Add product_id field
				writer.WriteField("product_id", product.ID.String())
				writer.WriteField("asset_type", "image")
				writer.WriteField("alt_text", "Laptop front view")

				// Add file with explicit Content-Type
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", "laptop.jpg"))
				h.Set("Content-Type", "image/jpeg")
				part, err := writer.CreatePart(h)
				if err != nil {
					t.Fatalf("Failed to create form file: %v", err)
				}
				// Write fake image data (valid header)
				part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
				writer.Close()

				httpReq := httptest.NewRequest("POST", "/api/v1/assets", body)
				httpReq.Header.Set("Content-Type", writer.FormDataContentType())
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				assetHandler.Upload(w, httpReq)

				// Then: Asset is created and associated with product
				if w.Code != http.StatusCreated {
					t.Fatalf("Expected status 201, got %d: %s", w.Code, w.Body.String())
				}

				var resp pb.UploadAssetResponse
				testutil.UnmarshalProtoResponse(t, w.Body, &resp)

				if resp.Asset == nil {
					t.Fatal("Asset should not be nil")
				}
				if resp.Asset.ProductId != product.ID.String() {
					t.Errorf("Expected product_id %s, got %s", product.ID.String(), resp.Asset.ProductId)
				}
				if resp.Asset.AssetType != pb.AssetType_ASSET_TYPE_IMAGE {
					t.Errorf("Expected asset type IMAGE, got %v", resp.Asset.AssetType)
				}
				if resp.Asset.AltText != "Laptop front view" {
					t.Errorf("Expected alt text 'Laptop front view', got %s", resp.Asset.AltText)
				}

				// Verify asset stored in database
				var dbAsset models.Asset
				err = db.First(&dbAsset, "id = ?", resp.Asset.Id).Error
				if err != nil {
					t.Errorf("Asset should be in database: %v", err)
				}
			},
		},
		{
			name: "US4-AS2: Set primary image",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, product *models.Product) {
				// Given: Product has multiple images
				asset1 := testutil.CreateTestAsset(t, db, product.ID, org.ID, storageDir, map[string]interface{}{
					"file_name":  "image1.jpg",
					"asset_type": "image",
					"is_primary": false,
				})

				asset2 := testutil.CreateTestAsset(t, db, product.ID, org.ID, storageDir, map[string]interface{}{
					"file_name":  "image2.jpg",
					"asset_type": "image",
					"is_primary": false,
				})

				// When: Product manager sets asset2 as primary
				req := &pb.SetPrimaryAssetRequest{
					Id: asset2.ID.String(),
				}

				reqBody, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/assets/"+asset2.ID.String()+"/set-primary", bytes.NewReader(reqBody))
				httpReq.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				assetHandler.SetPrimary(w, httpReq)

				// Then: Asset2 is primary, asset1 is not
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				// Verify in database
				var updatedAsset1, updatedAsset2 models.Asset
				db.First(&updatedAsset1, "id = ?", asset1.ID)
				db.First(&updatedAsset2, "id = ?", asset2.ID)

				if updatedAsset1.IsPrimary {
					t.Error("Asset1 should not be primary")
				}
				if !updatedAsset2.IsPrimary {
					t.Error("Asset2 should be primary")
				}
			},
		},
		{
			name: "US4-AS3: Delete asset from product",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, product *models.Product) {
				// Given: Product has an asset
				asset := testutil.CreateTestAsset(t, db, product.ID, org.ID, storageDir, map[string]interface{}{
					"file_name":  "image-to-delete.jpg",
					"asset_type": "image",
				})

				// When: Product manager deletes the asset
				httpReq := httptest.NewRequest("DELETE", "/api/v1/assets/"+asset.ID.String(), nil)
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				assetHandler.Delete(w, httpReq)

				// Then: Asset is removed from database and storage
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
				}

				// Verify asset deleted from database
				var count int64
				db.Model(&models.Asset{}).Where("id = ?", asset.ID).Count(&count)
				if count != 0 {
					t.Error("Asset should be deleted from database")
				}

				// Verify file deleted from storage
				if _, err := os.Stat(asset.StoragePath); !os.IsNotExist(err) {
					t.Error("Asset file should be deleted from storage")
				}
			},
		},
		{
			name: "US4-AS4: Reject unsupported file format",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization, product *models.Product) {
				// Given: A product exists
				// When: Product manager uploads an unsupported file type
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				writer.WriteField("product_id", product.ID.String())
				writer.WriteField("asset_type", "image")

				// Add executable file (unsupported)
				part, err := writer.CreateFormFile("file", "malware.exe")
				if err != nil {
					t.Fatalf("Failed to create form file: %v", err)
				}
				part.Write([]byte("fake-exe-data"))
				writer.Close()

				httpReq := httptest.NewRequest("POST", "/api/v1/assets", body)
				httpReq.Header.Set("Content-Type", writer.FormDataContentType())
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				assetHandler.Upload(w, httpReq)

				// Then: Upload is rejected with error
				if w.Code != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", w.Code)
				}

				var errResp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &errResp)
				if errCode, ok := errResp["code"].(string); !ok || errCode != "INVALID_FILE_FORMAT" {
					t.Errorf("Expected error code INVALID_FILE_FORMAT, got %v", errResp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Truncate tables before each test for isolation
			defer testutil.TruncateTables(db)

			// Recreate org and product for each test
			org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
				"name": "Test Org " + tt.name,
			})
			product := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
				"sku":  "LAPTOP-001",
				"name": "Test Laptop",
			})

			tt.testFunc(t, db, org, product)
		})
	}
}

// TestAssetEdgeCases tests edge cases for asset functionality
func TestAssetEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	storageDir := filepath.Join(os.TempDir(), "pim-test-assets-edge-"+uuid.New().String())
	defer os.RemoveAll(storageDir)

	assetService := services.NewAssetService(db, storageDir)
	productService := services.NewProductService(db)
	assetHandler := handlers.NewAssetHandler(assetService, productService)

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
			name: "File size exceeds limit (image > 10MB)",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				product := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-001",
					"name": "Product",
				})

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.WriteField("product_id", product.ID.String())
				writer.WriteField("asset_type", "image")

				// Use CreatePart to set Content-Type
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "file", "huge-image.jpg"))
				h.Set("Content-Type", "image/jpeg")
				part, _ := writer.CreatePart(h)
				// Write 11MB of data
				largeData := make([]byte, 11*1024*1024)
				part.Write(largeData)
				writer.Close()

				httpReq := httptest.NewRequest("POST", "/api/v1/assets", body)
				httpReq.Header.Set("Content-Type", writer.FormDataContentType())
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedError:  "FILE_TOO_LARGE",
		},
		{
			name: "Max assets per product (50)",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				product := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-002",
					"name": "Product with many assets",
				})

				// Create 50 assets
				for i := 0; i < 50; i++ {
					testutil.CreateTestAsset(t, db, product.ID, org.ID, storageDir, map[string]interface{}{
						"file_name":  fmt.Sprintf("image%d.jpg", i),
						"asset_type": "image",
					})
				}

				// Try to add 51st asset
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.WriteField("product_id", product.ID.String())
				writer.WriteField("asset_type", "image")

				part, _ := writer.CreateFormFile("file", "image51.jpg")
				part.Write([]byte("data"))
				writer.Close()

				httpReq := httptest.NewRequest("POST", "/api/v1/assets", body)
				httpReq.Header.Set("Content-Type", writer.FormDataContentType())
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "ASSET_LIMIT_EXCEEDED",
		},
		{
			name: "Invalid product_id",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				writer.WriteField("product_id", uuid.New().String()) // Non-existent product
				writer.WriteField("asset_type", "image")

				part, _ := writer.CreateFormFile("file", "image.jpg")
				part.Write([]byte("data"))
				writer.Close()

				httpReq := httptest.NewRequest("POST", "/api/v1/assets", body)
				httpReq.Header.Set("Content-Type", writer.FormDataContentType())
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)
				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "PRODUCT_NOT_FOUND",
		},
		{
			name: "Delete product deletes assets (cascade)",
			setup: func(t *testing.T) (context.Context, *http.Request) {
				product := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "PROD-CASCADE",
					"name": "Product to delete",
				})

				asset := testutil.CreateTestAsset(t, db, product.ID, org.ID, storageDir, map[string]interface{}{
					"file_name":  "cascade-test.jpg",
					"asset_type": "image",
				})

				// Delete product
				httpReq := httptest.NewRequest("DELETE", "/api/v1/products/"+product.ID.String(), nil)
				ctx := context.WithValue(httpReq.Context(), middleware.OrganizationIDKey, org.ID)

				productHandler := handlers.NewProductHandler(productService)
				w := httptest.NewRecorder()
				productHandler.Delete(w, httpReq.WithContext(ctx))

				// Verify asset deleted
				var count int64
				db.Model(&models.Asset{}).Where("id = ?", asset.ID).Count(&count)
				if count != 0 {
					t.Error("Asset should be deleted when product is deleted")
				}

				return ctx, httpReq.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpReq := tt.setup(t)

			w := httptest.NewRecorder()

			// Route to appropriate handler
			switch httpReq.Method {
			case "POST":
				if httpReq.URL.Path == "/api/v1/assets" {
					assetHandler.Upload(w, httpReq)
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
