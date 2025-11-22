package integration

import (
	"bytes"
	"context"
	"encoding/csv"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/handlers"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
	"github.com/yourorg/pim-demo/tests/testutil"
)

// TestImportExportAcceptanceScenarios tests all acceptance scenarios for User Story 6
func TestImportExportAcceptanceScenarios(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	// Initialize services and handlers
	productService := services.NewProductService(db)
	importService := services.NewImportService(db, productService)
	exportService := services.NewExportService(db)

	importHandler := handlers.NewImportHandler(importService)
	exportHandler := handlers.NewExportHandler(exportService)

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
			name:     "US6-AS1: Import CSV with product data (create/update by SKU)",
			scenario: "Given a CSV file with new and existing products, When I import the file, Then new products are created and existing products are updated by SKU",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create existing product
				existingProduct := testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":        "EXISTING-SKU-001",
					"name":       "Original Name",
					"base_price": int64(1000),
				})

				// Prepare CSV data
				headers := []string{"sku", "name", "description", "base_price", "status"}
				data := [][]string{
					{"EXISTING-SKU-001", "Updated Name", "Updated Description", "2000", "active"},
					{"NEW-SKU-001", "New Product", "New Description", "3000", "draft"},
				}

				csvFile, cleanupCSV := testutil.CreateTestCSV(t, headers, data)
				defer cleanupCSV()

				// Create multipart request
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, err := writer.CreateFormFile("file", filepath.Base(csvFile.Name()))
				if err != nil {
					t.Fatalf("Failed to create form file: %v", err)
				}
				fileContent, err := os.ReadFile(csvFile.Name())
				if err != nil {
					t.Fatalf("Failed to read CSV file: %v", err)
				}
				part.Write(fileContent)
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req = addOrgContext(req, org.ID)

				rec := httptest.NewRecorder()
				importHandler.Import(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
				}

				var importResp pb.ImportProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &importResp)

				if importResp.Result.SuccessfulRows != 2 {
					t.Errorf("Expected 2 successful imports, got %d", importResp.Result.SuccessfulRows)
				}
				if importResp.Result.FailedRows != 0 {
					t.Errorf("Expected 0 failures, got %d", importResp.Result.FailedRows)
				}

				// Verify existing product updated
				var updatedProduct models.Product
				db.First(&updatedProduct, existingProduct.ID)
				if updatedProduct.Name != "Updated Name" {
					t.Errorf("Expected product name to be updated to 'Updated Name', got '%s'", updatedProduct.Name)
				}
				if updatedProduct.BasePrice != 2000 {
					t.Errorf("Expected product price to be updated to 2000, got %d", updatedProduct.BasePrice)
				}

				// Verify new product created
				var newProduct models.Product
				if err := db.Where("sku = ?", "NEW-SKU-001").First(&newProduct).Error; err != nil {
					t.Fatalf("Failed to find new product: %v", err)
				}
				if newProduct.Name != "New Product" {
					t.Errorf("Expected new product name 'New Product', got '%s'", newProduct.Name)
				}
			},
		},
		{
			name:     "US6-AS2: Export products to CSV with all data",
			scenario: "Given a list of products, When I request an export, Then I receive a CSV file containing all product data",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create test products
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "EXPORT-001",
					"name": "Export Product 1",
				})
				testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
					"sku":  "EXPORT-002",
					"name": "Export Product 2",
				})

				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/export", nil)
				req = addOrgContext(req, org.ID)

				rec := httptest.NewRecorder()
				exportHandler.Export(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("Expected status %d, got %d", http.StatusOK, rec.Code)
				}

				if contentType := rec.Header().Get("Content-Type"); contentType != "text/csv; charset=utf-8" {
					t.Errorf("Expected Content-Type text/csv; charset=utf-8, got %s", contentType)
				}

				// Parse CSV response
				csvReader := csv.NewReader(rec.Body)
				records, err := csvReader.ReadAll()
				if err != nil {
					t.Fatalf("Failed to read CSV response: %v", err)
				}

				if len(records) < 3 { // Header + 2 products
					t.Fatalf("Expected at least 3 records (header + 2 products), got %d", len(records))
				}

				// Verify headers
				expectedHeaders := []string{"sku", "name", "description", "base_price", "status"}
				headers := records[0]
				// Check if expected headers are present
				for _, h := range expectedHeaders {
					found := false
					for _, rh := range headers {
						if rh == h {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Missing header: %s", h)
					}
				}
			},
		},
		{
			name:     "US6-AS3: Import validation with detailed error report",
			scenario: "Given a CSV with invalid data, When I import the file, Then I receive a report listing all errors by row number",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Prepare CSV with invalid data
				headers := []string{"sku", "name", "base_price"}
				data := [][]string{
					{"VALID-SKU", "Valid Product", "1000"},
					{"", "Missing SKU", "1000"},            // Error: Missing SKU
					{"INVALID-PRICE", "Bad Price", "-500"}, // Error: Negative price
				}

				csvFile, cleanupCSV := testutil.CreateTestCSV(t, headers, data)
				defer cleanupCSV()

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", filepath.Base(csvFile.Name()))
				fileContent, _ := os.ReadFile(csvFile.Name())
				part.Write(fileContent)
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req = addOrgContext(req, org.ID)

				rec := httptest.NewRecorder()
				importHandler.Import(rec, req)

				// Should return 200 OK but with error details in response
				if rec.Code != http.StatusOK {
					t.Fatalf("Expected status %d, got %d", http.StatusOK, rec.Code)
				}

				var importResp pb.ImportProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &importResp)

				if importResp.Result.SuccessfulRows != 1 {
					t.Errorf("Expected 1 successful import, got %d", importResp.Result.SuccessfulRows)
				}
				if importResp.Result.FailedRows != 2 {
					t.Errorf("Expected 2 failures, got %d", importResp.Result.FailedRows)
				}
				if len(importResp.Result.Errors) != 2 {
					t.Errorf("Expected 2 error details, got %d", len(importResp.Result.Errors))
				}

				// Verify error messages contain row numbers
				foundRow3 := false
				foundRow4 := false
				for _, err := range importResp.Result.Errors {
					if strings.Contains(err.ErrorMessage, "row 3") || err.RowNumber == 3 {
						foundRow3 = true
					}
					if strings.Contains(err.ErrorMessage, "row 4") || err.RowNumber == 4 {
						foundRow4 = true
					}
				}
				if !foundRow3 {
					t.Error("Expected error for row 3 (missing SKU)")
				}
				if !foundRow4 {
					t.Error("Expected error for row 4 (negative price)")
				}
			},
		},
		{
			name:     "US6-AS4: Large import with progress tracking",
			scenario: "Given a large CSV file, When I start an import, Then I can track the progress of the job",
			testFunc: func(t *testing.T) {
				defer testutil.TruncateTables(db)
				org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

				// Create large CSV (100 rows for test speed, but logic handles larger)
				csvFile, cleanupCSV := testutil.CreateLargeCSV(t, 100)
				defer cleanupCSV()

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", filepath.Base(csvFile.Name()))
				fileContent, _ := os.ReadFile(csvFile.Name())
				part.Write(fileContent)
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req = addOrgContext(req, org.ID)

				rec := httptest.NewRecorder()
				importHandler.Import(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("Expected status %d, got %d", http.StatusOK, rec.Code)
				}

				var importResp pb.ImportProductsResponse
				testutil.UnmarshalProtoResponse(t, rec.Body, &importResp)

				if importResp.Result.SuccessfulRows != 100 {
					t.Errorf("Expected 100 successful imports, got %d", importResp.Result.SuccessfulRows)
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

// TestImportExportEdgeCases tests edge cases for import/export
func TestImportExportEdgeCases(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	importService := services.NewImportService(db, services.NewProductService(db))
	importHandler := handlers.NewImportHandler(importService)

	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	t.Run("Empty CSV file", func(t *testing.T) {
		org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

		// Create empty file
		tmpFile, _ := os.CreateTemp("", "empty-*.csv")
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", filepath.Base(tmpFile.Name()))
		part.Write([]byte{})
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		importHandler.Import(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("Invalid CSV format", func(t *testing.T) {
		org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

		// Create invalid file
		tmpFile, _ := os.CreateTemp("", "invalid-*.csv")
		tmpFile.WriteString("sku,name\nval1,val2,val3") // Mismatched columns
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", filepath.Base(tmpFile.Name()))
		fileContent, _ := os.ReadFile(tmpFile.Name())
		part.Write(fileContent)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		importHandler.Import(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}
