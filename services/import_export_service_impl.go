package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// importExportServiceImpl implements the ImportExportService interface
type importExportServiceImpl struct {
	db *gorm.DB
}

// NewImportExportService creates a new instance of ImportExportService
func NewImportExportService(db *gorm.DB) ImportExportService {
	return &importExportServiceImpl{db: db}
}

// ImportProducts imports products from CSV data
func (s *importExportServiceImpl) ImportProducts(ctx context.Context, orgID uuid.UUID, csvReader io.Reader, mode pb.ImportMode) (*pb.ImportResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ImportExportService.ImportProducts")
	defer span.Finish()

	startTime := time.Now()

	// Parse CSV
	reader := csv.NewReader(csvReader)
	reader.TrimLeadingSpace = true

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Validate headers
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	// Check required columns
	requiredCols := []string{"sku", "name", "base_price"}
	for _, col := range requiredCols {
		if _, ok := headerMap[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	// Import data with transaction
	result := &pb.ImportResult{
		TotalRows:       0,
		SuccessfulRows:  0,
		FailedRows:      0,
		SkippedRows:     0,
		Errors:          make([]*pb.ImportError, 0),
		StartedAt:       nil, // Will be set later
		CompletedAt:     nil,
		DurationSeconds: 0,
	}

	rowNumber := 1 // Start from 1 (after header)

	// Use transaction for all imports
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("failed to read CSV row %d: %w", rowNumber, err)
			}

			rowNumber++
			result.TotalRows++

			// Parse row
			product, parseErr := s.parseCSVRow(record, headerMap, orgID)
			if parseErr != nil {
				result.FailedRows++
				result.Errors = append(result.Errors, &pb.ImportError{
					RowNumber:    int32(rowNumber),
					Sku:          getSKUFromRecord(record, headerMap),
					ErrorCode:    "PARSE_ERROR",
					ErrorMessage: parseErr.Error(),
				})
				continue
			}

			// Check import mode and handle duplicates
			var existing models.Product
			existsErr := tx.Where("organization_id = ? AND sku = ?", orgID, product.SKU).First(&existing).Error

			if existsErr == nil {
				// Product exists
				if mode == pb.ImportMode_IMPORT_MODE_CREATE_ONLY {
					result.FailedRows++
					result.Errors = append(result.Errors, &pb.ImportError{
						RowNumber:    int32(rowNumber),
						Sku:          product.SKU,
						ErrorCode:    "DUPLICATE_SKU",
						ErrorMessage: fmt.Sprintf("product with SKU '%s' already exists (create-only mode)", product.SKU),
					})
					continue
				} else if mode == pb.ImportMode_IMPORT_MODE_UPDATE_ONLY || mode == pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE {
					// Update existing product
					product.ID = existing.ID
					product.CreatedAt = existing.CreatedAt
					if err := tx.Save(product).Error; err != nil {
						result.FailedRows++
						result.Errors = append(result.Errors, &pb.ImportError{
							RowNumber:    int32(rowNumber),
							Sku:          product.SKU,
							ErrorCode:    "UPDATE_ERROR",
							ErrorMessage: err.Error(),
						})
						continue
					}
					result.SuccessfulRows++
				}
			} else if existsErr == gorm.ErrRecordNotFound {
				// Product doesn't exist
				if mode == pb.ImportMode_IMPORT_MODE_UPDATE_ONLY {
					result.FailedRows++
					result.Errors = append(result.Errors, &pb.ImportError{
						RowNumber:    int32(rowNumber),
						Sku:          product.SKU,
						ErrorCode:    "NOT_FOUND",
						ErrorMessage: fmt.Sprintf("product with SKU '%s' not found (update-only mode)", product.SKU),
					})
					continue
				} else {
					// Create new product
					if err := tx.Create(product).Error; err != nil {
						result.FailedRows++
						result.Errors = append(result.Errors, &pb.ImportError{
							RowNumber:    int32(rowNumber),
							Sku:          product.SKU,
							ErrorCode:    "CREATE_ERROR",
							ErrorMessage: err.Error(),
						})
						continue
					}
					result.SuccessfulRows++
				}
			} else {
				// Database error
				return fmt.Errorf("failed to check for existing product: %w", existsErr)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("import transaction failed: %w", err)
	}

	result.DurationSeconds = time.Since(startTime).Seconds()

	return result, nil
}

// ExportProducts exports products to CSV format
func (s *importExportServiceImpl) ExportProducts(ctx context.Context, orgID uuid.UUID, filter *pb.ProductsFilter) (io.ReadCloser, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ImportExportService.ExportProducts")
	defer span.Finish()

	// Build query with filters
	query := s.db.WithContext(ctx).Model(&models.Product{}).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL")

	// Apply filters (reuse from search service)
	if filter != nil {
		// Category filter
		if len(filter.CategoryIds) > 0 {
			categoryUUIDs := make([]uuid.UUID, 0)
			for _, catID := range filter.CategoryIds {
				if id, err := uuid.Parse(catID); err == nil {
					categoryUUIDs = append(categoryUUIDs, id)
				}
			}
			if len(categoryUUIDs) > 0 {
				query = query.Joins("JOIN product_categories ON product_categories.product_id = products.id").
					Where("product_categories.category_id IN ?", categoryUUIDs)
			}
		}

		// Price range filter
		if filter.MinPrice > 0 {
			query = query.Where("base_price >= ?", filter.MinPrice)
		}
		if filter.MaxPrice > 0 {
			query = query.Where("base_price <= ?", filter.MaxPrice)
		}

		// Status filter
		if len(filter.Statuses) > 0 {
			statuses := make([]string, len(filter.Statuses))
			for i, status := range filter.Statuses {
				statuses[i] = productStatusToStringForExport(status)
			}
			query = query.Where("status IN ?", statuses)
		}
	}

	// Fetch products
	var products []models.Product
	if err := query.Preload("Categories").Order("created_at DESC").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch products for export: %w", err)
	}

	// Generate CSV
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	headers := []string{"sku", "name", "description", "base_price", "status", "attributes", "category_ids"}
	if err := writer.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write data rows
	for _, product := range products {
		// Convert attributes to JSON string
		attributesJSON := "{}"
		if len(product.Attributes) > 0 {
			if jsonBytes, err := json.Marshal(product.Attributes); err == nil {
				attributesJSON = string(jsonBytes)
			}
		}

		// Convert categories to comma-separated IDs
		categoryIDs := make([]string, len(product.Categories))
		for i, cat := range product.Categories {
			categoryIDs[i] = cat.ID.String()
		}
		categoryIDsStr := strings.Join(categoryIDs, ",")

		row := []string{
			product.SKU,
			product.Name,
			product.Description,
			strconv.FormatInt(product.BasePrice, 10),
			string(product.Status),
			attributesJSON,
			categoryIDsStr,
		}

		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// ValidateImport validates CSV data without importing
func (s *importExportServiceImpl) ValidateImport(ctx context.Context, orgID uuid.UUID, csvReader io.Reader) (*pb.ValidateImportResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ImportExportService.ValidateImport")
	defer span.Finish()

	reader := csv.NewReader(csvReader)
	reader.TrimLeadingSpace = true

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	// Check required columns
	requiredCols := []string{"sku", "name", "base_price"}
	for _, col := range requiredCols {
		if _, ok := headerMap[col]; !ok {
			return &pb.ValidateImportResponse{
				IsValid:      false,
				TotalRows:    0,
				ValidRows:    0,
				InvalidRows:  0,
				Errors:       []*pb.ImportError{{RowNumber: 0, ErrorCode: "MISSING_COLUMN", ErrorMessage: fmt.Sprintf("missing required column: %s", col)}},
				Warnings:     []string{},
			}, nil
		}
	}

	response := &pb.ValidateImportResponse{
		IsValid:      true,
		TotalRows:    0,
		ValidRows:    0,
		InvalidRows:  0,
		Errors:       make([]*pb.ImportError, 0),
		Warnings:     make([]string, 0),
	}

	rowNumber := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row %d: %w", rowNumber, err)
		}

		rowNumber++
		response.TotalRows++

		// Validate row
		_, parseErr := s.parseCSVRow(record, headerMap, orgID)
		if parseErr != nil {
			response.InvalidRows++
			response.IsValid = false
			response.Errors = append(response.Errors, &pb.ImportError{
				RowNumber:    int32(rowNumber),
				Sku:          getSKUFromRecord(record, headerMap),
				ErrorCode:    "VALIDATION_ERROR",
				ErrorMessage: parseErr.Error(),
			})
		} else {
			response.ValidRows++
		}
	}

	return response, nil
}

// GetImportTemplate generates a CSV template for imports
func (s *importExportServiceImpl) GetImportTemplate(ctx context.Context, includeExamples bool) (io.ReadCloser, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	headers := []string{"sku", "name", "description", "base_price", "status", "attributes", "category_ids"}
	writer.Write(headers)

	// Write example rows if requested
	if includeExamples {
		writer.Write([]string{
			"EXAMPLE-001",
			"Example Product",
			"This is an example product description",
			"9999",
			"active",
			`{"color":"blue","size":"M"}`,
			"",
		})
		writer.Write([]string{
			"EXAMPLE-002",
			"Another Example",
			"Optional description",
			"4999",
			"draft",
			"{}",
			"category-uuid-1,category-uuid-2",
		})
	}

	writer.Flush()
	return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// parseCSVRow parses a CSV row into a Product model
func (s *importExportServiceImpl) parseCSVRow(record []string, headerMap map[string]int, orgID uuid.UUID) (*models.Product, error) {
	product := &models.Product{
		OrganizationID: orgID,
	}

	// SKU (required)
	if idx, ok := headerMap["sku"]; ok && idx < len(record) {
		product.SKU = strings.TrimSpace(record[idx])
		if product.SKU == "" {
			return nil, fmt.Errorf("SKU is required")
		}
	} else {
		return nil, fmt.Errorf("SKU column not found")
	}

	// Name (required)
	if idx, ok := headerMap["name"]; ok && idx < len(record) {
		product.Name = strings.TrimSpace(record[idx])
		if product.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
	} else {
		return nil, fmt.Errorf("name column not found")
	}

	// Description (optional)
	if idx, ok := headerMap["description"]; ok && idx < len(record) {
		product.Description = strings.TrimSpace(record[idx])
	}

	// Base Price (required)
	if idx, ok := headerMap["base_price"]; ok && idx < len(record) {
		priceStr := strings.TrimSpace(record[idx])
		price, err := strconv.ParseInt(priceStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid base_price '%s': must be an integer", priceStr)
		}
		if price < 0 {
			return nil, fmt.Errorf("base_price must be non-negative")
		}
		product.BasePrice = price
	} else {
		return nil, fmt.Errorf("base_price column not found")
	}

	// Status (optional, defaults to draft)
	product.Status = models.ProductStatusDraft
	if idx, ok := headerMap["status"]; ok && idx < len(record) {
		statusStr := strings.ToLower(strings.TrimSpace(record[idx]))
		switch statusStr {
		case "active":
			product.Status = models.ProductStatusActive
		case "draft", "":
			product.Status = models.ProductStatusDraft
		case "discontinued":
			product.Status = models.ProductStatusDiscontinued
		default:
			// Default to draft for unknown status
			product.Status = models.ProductStatusDraft
		}
	}

	// Attributes (optional)
	product.Attributes = datatypes.JSON([]byte("{}"))
	if idx, ok := headerMap["attributes"]; ok && idx < len(record) {
		attrStr := strings.TrimSpace(record[idx])
		if attrStr != "" && attrStr != "{}" {
			// Validate JSON
			var attrs map[string]interface{}
			if err := json.Unmarshal([]byte(attrStr), &attrs); err != nil {
				return nil, fmt.Errorf("invalid attributes JSON: %w", err)
			}
			product.Attributes = datatypes.JSON([]byte(attrStr))
		}
	}

	// Category IDs (optional, handled separately after product creation)
	// For now, we'll skip category assignment in CSV import

	return product, nil
}

// getSKUFromRecord extracts SKU from record if available
func getSKUFromRecord(record []string, headerMap map[string]int) string {
	if idx, ok := headerMap["sku"]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}

// productStatusToStringForExport converts protobuf status to string for export
func productStatusToStringForExport(status pb.ProductStatus) string {
	switch status {
	case pb.ProductStatus_PRODUCT_STATUS_DRAFT:
		return "draft"
	case pb.ProductStatus_PRODUCT_STATUS_ACTIVE:
		return "active"
	case pb.ProductStatus_PRODUCT_STATUS_DISCONTINUED:
		return "discontinued"
	default:
		return "draft"
	}
}

