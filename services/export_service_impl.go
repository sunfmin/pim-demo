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

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// exportServiceImpl implements the ExportService interface
type exportServiceImpl struct {
	db *gorm.DB
}

// NewExportService creates a new instance of ExportService
func NewExportService(db *gorm.DB) ExportService {
	return &exportServiceImpl{db: db}
}

// ExportProducts exports products to CSV format
func (s *exportServiceImpl) ExportProducts(ctx context.Context, orgID uuid.UUID, filter *pb.ProductsFilter) (io.ReadCloser, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ExportService.ExportProducts")
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
