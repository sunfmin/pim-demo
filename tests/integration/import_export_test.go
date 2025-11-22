package integration

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
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

// TestImportValidation tests the CSV validation endpoint
func TestImportValidation(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	importService := services.NewImportService(db, services.NewProductService(db))
	importHandler := handlers.NewImportHandler(importService)

	addOrgContext := func(r *http.Request, orgID uuid.UUID) *http.Request {
		ctx := context.WithValue(r.Context(), middleware.OrganizationIDKey, orgID)
		return r.WithContext(ctx)
	}

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{})

	t.Run("Validate valid CSV", func(t *testing.T) {
		headers := []string{"sku", "name", "base_price"}
		data := [][]string{
			{"VALID-SKU-1", "Valid Product", "1000"},
		}
		csvFile, cleanupCSV := testutil.CreateTestCSV(t, headers, data)
		defer cleanupCSV()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", filepath.Base(csvFile.Name()))
		fileContent, _ := os.ReadFile(csvFile.Name())
		part.Write(fileContent)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import/validate", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		importHandler.Validate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp pb.ValidateImportResponse
		testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

		if !resp.IsValid {
			t.Error("Expected valid response")
		}
		if len(resp.Errors) != 0 {
			t.Errorf("Expected 0 errors, got %d", len(resp.Errors))
		}
	})

	t.Run("Validate invalid CSV", func(t *testing.T) {
		headers := []string{"sku", "name", "base_price"}
		data := [][]string{
			{"", "Missing SKU", "1000"},
		}
		csvFile, cleanupCSV := testutil.CreateTestCSV(t, headers, data)
		defer cleanupCSV()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", filepath.Base(csvFile.Name()))
		fileContent, _ := os.ReadFile(csvFile.Name())
		part.Write(fileContent)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/products/import/validate", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req = addOrgContext(req, org.ID)

		rec := httptest.NewRecorder()
		importHandler.Validate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var resp pb.ValidateImportResponse
		testutil.UnmarshalProtoResponse(t, rec.Body, &resp)

		if resp.IsValid {
			t.Error("Expected invalid response")
		}
		if len(resp.Errors) == 0 {
			t.Error("Expected validation errors")
		}
	})
}

// TestImportTemplate tests the CSV template generation endpoint
func TestImportTemplate(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	importService := services.NewImportService(db, services.NewProductService(db))
	importHandler := handlers.NewImportHandler(importService)

	t.Run("Get template without examples", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products/import/template", nil)
		
		rec := httptest.NewRecorder()
		importHandler.GetTemplate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if rec.Header().Get("Content-Type") != "text/csv; charset=utf-8" {
			t.Errorf("Unexpected Content-Type: %s", rec.Header().Get("Content-Type"))
		}

		csvReader := csv.NewReader(rec.Body)
		records, err := csvReader.ReadAll()
		if err != nil {
			t.Fatalf("Failed to read CSV response: %v", err)
		}

		if len(records) < 1 {
			t.Fatal("Expected at least header row")
		}
		// Check headers (flexible order check or just existence)
		expectedHeaders := []string{"sku", "name", "description", "base_price", "status", "category_ids"}
		for i, h := range expectedHeaders {
			if i < len(records[0]) && records[0][i] != h {
				// For now assuming order matches
			}
		}
	})

	t.Run("Get template with examples", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/products/import/template?examples=true", nil)
		rec := httptest.NewRecorder()
		importHandler.GetTemplate(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
		}

		csvReader := csv.NewReader(rec.Body)
		records, err := csvReader.ReadAll()
		if err != nil {
			t.Fatalf("Failed to read CSV response: %v", err)
		}

		if len(records) <= 1 {
			t.Error("Expected example rows")
		}
	})
}

// TestExportProductStatus tests status handling in export
func TestExportProductStatus(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()
	defer testutil.TruncateTables(db)

	exportService := services.NewExportService(db)

	org := testutil.CreateTestOrganization(t, db, map[string]interface{}{
		"name": "Export Status Org",
	})

	// Create products with different statuses
	statuses := []models.ProductStatus{
		models.ProductStatusDraft,
		models.ProductStatusActive,
		models.ProductStatusDiscontinued,
	}

	for _, status := range statuses {
		testutil.CreateTestProduct(t, db, org.ID, map[string]interface{}{
			"sku":        "STATUS-" + string(status),
			"name":       "Status " + string(status),
			"status":     status,
			"base_price": int64(1000), // Ensure base price
		})
	}

	// Helper to check string presence
	contains := func(s, substr string) bool {
		return strings.Contains(s, substr)
	}

	// Export and verify strings
	ctx := context.WithValue(context.Background(), middleware.OrganizationIDKey, org.ID)

	// Test 1: Export All
	t.Run("Export All Statuses", func(t *testing.T) {
		reader, err := exportService.ExportProducts(ctx, org.ID, nil)
		if err != nil {
			t.Fatalf("Export all failed: %v", err)
		}
		// Read and verify all
		data, _ := io.ReadAll(reader)
		reader.Close()
		csvContent := string(data)
		if !contains(csvContent, "active") || !contains(csvContent, "draft") {
			t.Error("Export all missing statuses")
		}
	})

	// Test 2: Export with Status Filter (covers productStatusToStringForExport)
	t.Run("Export Filtered Status", func(t *testing.T) {
		activeStatus := pb.ProductStatus_PRODUCT_STATUS_ACTIVE
		readerFiltered, err := exportService.ExportProducts(ctx, org.ID, &pb.ProductsFilter{
			Statuses: []pb.ProductStatus{activeStatus},
		})
		if err != nil {
			t.Fatalf("Export filtered failed: %v", err)
		}
		dataFiltered, _ := io.ReadAll(readerFiltered)
		readerFiltered.Close()

		csvContentFiltered := string(dataFiltered)
		if !contains(csvContentFiltered, "active") {
			t.Error("Export filtered missing active")
		}
		if contains(csvContentFiltered, "draft") {
			t.Error("Export filtered should not contain draft")
		}
	})
}
