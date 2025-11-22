package integration

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestImportExportAcceptanceScenarios tests all acceptance scenarios for User Story 6
// Covers US6-AS1 through US6-AS6 from spec.md
func TestImportExportAcceptanceScenarios(t *testing.T) {
	// Setup test database
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Setup services and handlers
	productService := services.NewProductService(db)
	importExportService := services.NewImportExportService(db)
	importExportHandler := handlers.NewImportExportHandler(importExportService, productService)

	tests := []struct {
		name     string
		testFunc func(t *testing.T, db *gorm.DB, org *models.Organization)
	}{
		{
			name: "US6-AS1: Upload valid CSV and create 100 products",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: A valid CSV file with 100 products
				csvContent := generateValidCSV(100)

				// When: User uploads the CSV
				resp := uploadCSV(t, importExportHandler, org.ID, csvContent)

				// Then: All 100 products are created
				if resp.Result.TotalRows != 100 {
					t.Errorf("Expected 100 total rows, got %d", resp.Result.TotalRows)
				}
				if resp.Result.SuccessfulRows != 100 {
					t.Errorf("Expected 100 successful imports, got %d", resp.Result.SuccessfulRows)
				}
				if resp.Result.FailedRows != 0 {
					t.Errorf("Expected 0 errors, got %d", resp.Result.FailedRows)
				}

				// Verify products in database
				var count int64
				db.Model(&models.Product{}).Where("organization_id = ?", org.ID).Count(&count)
				if count != 100 {
					t.Errorf("Expected 100 products in database, got %d", count)
				}
			},
		},
		{
			name: "US6-AS2: Import with validation errors",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: CSV with some invalid rows
				csvContent := `sku,name,description,base_price,status
VALID-001,Valid Product,Good product,10000,active
INVALID-001,,Missing name,5000,active
VALID-002,Another Valid,Good product,8000,active
INVALID-002,Invalid Price,Bad price,not-a-number,active
VALID-003,Third Valid,Good product,12000,active`

				// When: User uploads the CSV
				resp := uploadCSV(t, importExportHandler, org.ID, csvContent)

				// Then: Valid rows imported, invalid rows reported
				if resp.Result.TotalRows != 5 {
					t.Errorf("Expected 5 total rows, got %d", resp.Result.TotalRows)
				}
				if resp.Result.SuccessfulRows != 3 {
					t.Errorf("Expected 3 successful imports, got %d", resp.Result.SuccessfulRows)
				}
				if resp.Result.FailedRows != 2 {
					t.Errorf("Expected 2 errors, got %d", resp.Result.FailedRows)
				}

				// Verify error details
				if len(resp.Result.Errors) != 2 {
					t.Errorf("Expected 2 error details, got %d", len(resp.Result.Errors))
				}

				// Verify only valid products created
				var count int64
				db.Model(&models.Product{}).Where("organization_id = ?", org.ID).Count(&count)
				if count != 3 {
					t.Errorf("Expected 3 valid products in database, got %d", count)
				}
			},
		},
		{
			name: "US6-AS3: Export all products to CSV",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: 10 products exist
				for i := 0; i < 10; i++ {
					testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
						"sku":         testutil.GenerateSKU(),
						"name":        "Export Test Product " + string(rune('A'+i)),
						"description": "Product for export test",
						"base_price":  int64((i + 1) * 1000),
						"status":      models.ProductStatusActive,
					})
				}

				// When: User exports products
				csvData := exportProducts(t, importExportHandler, org.ID, nil)

				// Then: CSV contains all 10 products
				lines := strings.Split(strings.TrimSpace(csvData), "\n")
				if len(lines) != 11 { // 1 header + 10 data rows
					t.Errorf("Expected 11 lines (header + 10 products), got %d", len(lines))
				}

				// Verify CSV structure
				reader := csv.NewReader(strings.NewReader(csvData))
				records, err := reader.ReadAll()
				if err != nil {
					t.Fatalf("Failed to parse CSV: %v", err)
				}

				// Check header
				expectedHeaders := []string{"sku", "name", "description", "base_price", "status", "attributes", "category_ids"}
				if !cmp.Equal(records[0], expectedHeaders) {
					t.Errorf("CSV headers mismatch:\n%s", cmp.Diff(expectedHeaders, records[0]))
				}

				// Verify all products present
				if len(records)-1 != 10 {
					t.Errorf("Expected 10 product records, got %d", len(records)-1)
				}
			},
		},
		{
			name: "US6-AS4: Export filtered products",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Products with different statuses
				category := testutil.CreateTestCategory(t, db, org.ID, map[string]interface{}{
					"name": "Electronics",
					"slug": "electronics",
				})

				activeProduct := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":    "ACTIVE-001",
					"name":   "Active Product",
					"status": models.ProductStatusActive,
				})

				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":    "DRAFT-001",
					"name":   "Draft Product",
					"status": models.ProductStatusDraft,
				})

				// Assign category to active product
				db.Exec("INSERT INTO product_categories (product_id, category_id) VALUES (?, ?)", activeProduct.ID, category.ID)

				// When: User exports only active products in specific category
				filter := &pb.ExportProductsRequest{
					Filter: &pb.ProductsFilter{
						Statuses:    []pb.ProductStatus{pb.ProductStatus_PRODUCT_STATUS_ACTIVE},
						CategoryIds: []string{category.ID.String()},
					},
				}
				csvData := exportProducts(t, importExportHandler, org.ID, filter)

				// Then: Only active product in category is exported
				lines := strings.Split(strings.TrimSpace(csvData), "\n")
				if len(lines) != 2 { // 1 header + 1 data row
					t.Errorf("Expected 2 lines (header + 1 product), got %d", len(lines))
				}

				// Verify exported product is the active one
				if !strings.Contains(csvData, "ACTIVE-001") {
					t.Error("Expected CSV to contain ACTIVE-001")
				}
				if strings.Contains(csvData, "DRAFT-001") {
					t.Error("Expected CSV not to contain DRAFT-001")
				}
			},
		},
		{
			name: "US6-AS5: Import completes within 10 seconds for 1000 products",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: A CSV with 1000 valid products
				csvContent := generateValidCSV(1000)

				// When: User uploads the CSV
				start := time.Now()
				resp := uploadCSV(t, importExportHandler, org.ID, csvContent)
				elapsed := time.Since(start)

				// Then: Import completes within 10 seconds
				if elapsed > 10*time.Second {
					t.Errorf("Import took %v, expected < 10 seconds", elapsed)
				}

				t.Logf("Import of 1000 products completed in %v", elapsed)

				// Verify all products imported
				if resp.Result.SuccessfulRows != 1000 {
					t.Errorf("Expected 1000 successful imports, got %d", resp.Result.SuccessfulRows)
				}
			},
		},
		{
			name: "US6-AS6: Download export file",
			testFunc: func(t *testing.T, db *gorm.DB, org *models.Organization) {
				// Given: Products exist
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "DOWNLOAD-001",
					"name": "Download Test",
				})

				// When: User requests export download
				csvData := exportProducts(t, importExportHandler, org.ID, nil)

				// Then: CSV file is downloadable with correct headers
				if csvData == "" {
					t.Error("Expected non-empty CSV data")
				}

				// Verify CSV can be parsed
				reader := csv.NewReader(strings.NewReader(csvData))
				records, err := reader.ReadAll()
				if err != nil {
					t.Fatalf("Failed to parse downloaded CSV: %v", err)
				}

				if len(records) < 2 {
					t.Error("Expected at least header and one data row")
				}
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

// TestImportExportEdgeCases tests edge cases for import/export
func TestImportExportEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	importExportService := services.NewImportExportService(db)
	productService := services.NewProductService(db)
	importExportHandler := handlers.NewImportExportHandler(importExportService, productService)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Test Org Edge Cases",
	})

	tests := []struct {
		name          string
		csvContent    string
		expectedError bool
		checkResponse func(t *testing.T, resp *pb.ImportProductsResponse)
	}{
		{
			name:          "Empty CSV file",
			csvContent:    "",
			expectedError: true,
		},
		{
			name:          "CSV with only headers",
			csvContent:    "sku,name,description,base_price,status\n",
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				if resp.Result.TotalRows != 0 {
					t.Errorf("Expected 0 rows, got %d", resp.Result.TotalRows)
				}
			},
		},
		{
			name: "Duplicate SKUs in CSV",
			csvContent: `sku,name,description,base_price,status
DUP-001,Product 1,First,10000,active
DUP-001,Product 2,Duplicate,20000,active`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				if resp.Result.FailedRows == 0 {
					t.Error("Expected at least one error for duplicate SKU")
				}
			},
		},
		{
			name: "Missing required fields",
			csvContent: `sku,name,description,base_price,status
MISS-001,,Description only,10000,active`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				if resp.Result.FailedRows == 0 {
					t.Error("Expected error for missing required name field")
				}
			},
		},
		{
			name: "Invalid status value",
			csvContent: `sku,name,description,base_price,status
INV-001,Product,Description,10000,invalid-status`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				// Should either error or default to draft
				if resp.Result.FailedRows == 0 {
					// Check if it defaulted
					var product models.Product
					if err := db.Where("sku = ?", "INV-001").First(&product).Error; err == nil {
						if product.Status != models.ProductStatusDraft {
							t.Errorf("Expected status to default to draft, got %s", product.Status)
						}
					}
				}
			},
		},
		{
			name: "Very long field values",
			csvContent: `sku,name,description,base_price,status
LONG-001,` + strings.Repeat("A", 300) + `,Description,10000,active`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				// May error due to validation, or truncate
				if resp.Result.FailedRows > 0 {
					t.Logf("Long field rejected as expected: %v", resp.Result.Errors)
				}
			},
		},
		{
			name: "Special characters in fields",
			csvContent: `sku,name,description,base_price,status
SPEC-001,"Product with ""quotes""","Description with, comma",10000,active`,
			expectedError: false,
			checkResponse: func(t *testing.T, resp *pb.ImportProductsResponse) {
				if resp.Result.SuccessfulRows != 1 {
					t.Errorf("Expected 1 success with special characters, got %d", resp.Result.SuccessfulRows)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := uploadCSV(t, importExportHandler, org.ID, tt.csvContent)

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// Helper: generateValidCSV creates a CSV with N valid products
func generateValidCSV(count int) string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"sku", "name", "description", "base_price", "status"})

	// Write data rows
	for i := 0; i < count; i++ {
		sku := testutil.GenerateSKU()
		writer.Write([]string{
			sku,
			"Bulk Product " + sku,
			"Generated product for bulk import",
			"10000",
			"active",
		})
	}

	writer.Flush()
	return buf.String()
}

// Helper: uploadCSV uploads a CSV file and returns the import response
func uploadCSV(t *testing.T, handler *handlers.ImportExportHandler, orgID interface{}, csvContent string) *pb.ImportProductsResponse {
	t.Helper()

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "products.csv")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	fileWriter.Write([]byte(csvContent))
	writer.Close()

	// Create HTTP request
	req := httptest.NewRequest("POST", "/api/v1/products/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx := context.WithValue(req.Context(), "organization_id", orgID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Import(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Fatalf("Import failed with status %d: %s", w.Code, w.Body.String())
	}

	var resp pb.ImportProductsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	return &resp
}

// Helper: exportProducts exports products and returns CSV content
func exportProducts(t *testing.T, handler *handlers.ImportExportHandler, orgID interface{}, filter *pb.ExportProductsRequest) string {
	t.Helper()

	var reqBody []byte
	if filter != nil {
		reqBody, _ = json.Marshal(filter)
	} else {
		reqBody = []byte("{}")
	}

	req := httptest.NewRequest("POST", "/api/v1/products/export", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "organization_id", orgID)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Export failed with status %d: %s", w.Code, w.Body.String())
	}

	// Check Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" && contentType != "text/csv; charset=utf-8" {
		t.Errorf("Expected Content-Type text/csv, got %s", contentType)
	}

	return w.Body.String()
}

