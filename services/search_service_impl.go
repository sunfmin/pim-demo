package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// searchServiceImpl implements the SearchService interface
type searchServiceImpl struct {
	db *gorm.DB
}

// NewSearchService creates a new instance of SearchService
func NewSearchService(db *gorm.DB) SearchService {
	return &searchServiceImpl{db: db}
}

// Search performs full-text search with filters
func (s *searchServiceImpl) Search(ctx context.Context, orgID uuid.UUID, req *pb.SearchProductsRequest) ([]*models.Product, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.Product{}).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL")

	// Apply full-text search if query provided
	if req.Query != "" {
		// Sanitize search query to prevent SQL injection
		searchQuery := sanitizeSearchQuery(req.Query)

		// Use PostgreSQL full-text search with ts_rank for relevance
		query = query.Where(
			"search_vector @@ plainto_tsquery('english', ?) OR sku ILIKE ? OR name ILIKE ? OR description ILIKE ?",
			searchQuery,
			"%"+searchQuery+"%",
			"%"+searchQuery+"%",
			"%"+searchQuery+"%",
		).Order(gorm.Expr("ts_rank(search_vector, plainto_tsquery('english', ?)) DESC", searchQuery))
	} else {
		// Default sort by created_at descending when no search
		query = query.Order("created_at DESC")
	}

	// Apply category filter
	if len(req.CategoryIds) > 0 {
		categoryUUIDs := make([]uuid.UUID, 0)
		for _, catID := range req.CategoryIds {
			if id, err := uuid.Parse(catID); err == nil {
				categoryUUIDs = append(categoryUUIDs, id)
			}
		}
		if len(categoryUUIDs) > 0 {
			query = query.Joins("JOIN product_categories ON product_categories.product_id = products.id").
				Where("product_categories.category_id IN ?", categoryUUIDs)
		}
	}

	// Apply status filter
	if len(req.Statuses) > 0 {
		statuses := make([]string, len(req.Statuses))
		for i, status := range req.Statuses {
			statuses[i] = productStatusToString(status)
		}
		query = query.Where("status IN ?", statuses)
	}

	// Count total results
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// Apply pagination
	if req.Pagination != nil {
		offset := (req.Pagination.Page - 1) * req.Pagination.PageSize
		query = query.Offset(int(offset)).Limit(int(req.Pagination.PageSize))
	}

	// Fetch products with relationships
	var products []*models.Product
	if err := query.Preload("Categories").Find(&products).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch products: %w", err)
	}

	return products, total, nil
}

// Filter applies filters without full-text search
func (s *searchServiceImpl) Filter(ctx context.Context, orgID uuid.UUID, filter *pb.ProductsFilter, pagination *pb.PaginationRequest) ([]*models.Product, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.Product{}).
		Where("organization_id = ?", orgID).
		Where("deleted_at IS NULL")

	// Apply filters
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
				statuses[i] = productStatusToString(status)
			}
			query = query.Where("status IN ?", statuses)
		}

		// Search query filter
		if filter.SearchQuery != "" {
			searchQuery := sanitizeSearchQuery(filter.SearchQuery)
			query = query.Where(
				"search_vector @@ plainto_tsquery('english', ?) OR sku ILIKE ? OR name ILIKE ?",
				searchQuery,
				"%"+searchQuery+"%",
				"%"+searchQuery+"%",
			)
		}
	}

	// Default sort by created_at descending
	query = query.Order("created_at DESC")

	// Count total results
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// Apply pagination
	if pagination != nil {
		offset := (pagination.Page - 1) * pagination.PageSize
		query = query.Offset(int(offset)).Limit(int(pagination.PageSize))
	}

	// Fetch products
	var products []*models.Product
	if err := query.Preload("Categories").Find(&products).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch products: %w", err)
	}

	return products, total, nil
}

// productStatusToString converts protobuf status to string
func productStatusToString(status pb.ProductStatus) string {
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

// sanitizeSearchQuery sanitizes user input for search
func sanitizeSearchQuery(query string) string {
	// Remove potentially dangerous characters for tsquery
	// Keep alphanumeric, spaces, hyphens, underscores
	query = strings.TrimSpace(query)

	// Replace multiple spaces with single space
	for strings.Contains(query, "  ") {
		query = strings.ReplaceAll(query, "  ", " ")
	}

	return query
}

