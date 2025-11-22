package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// productServiceImpl implements ProductService interface
type productServiceImpl struct {
	db *gorm.DB
}

// NewProductService creates a new product service instance
func NewProductService(db *gorm.DB) ProductService {
	return &productServiceImpl{db: db}
}

// Create creates a new product
func (s *productServiceImpl) Create(ctx context.Context, req *pb.CreateProductRequest, orgID uuid.UUID) (*pb.Product, error) {
	// Create child span for tracing
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.Create")
	defer span.Finish()

	// Validate required fields
	if req.Sku == "" {
		return nil, fmt.Errorf("SKU is required: %w", ErrInvalidProduct)
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required: %w", ErrInvalidProduct)
	}
	if len(req.Sku) > 50 {
		return nil, fmt.Errorf("SKU too long (max 50 characters): %w", ErrInvalidProduct)
	}
	if len(req.Name) > 500 {
		return nil, fmt.Errorf("name too long (max 500 characters): %w", ErrInvalidProduct)
	}
	if req.BasePrice < 0 {
		return nil, fmt.Errorf("base price cannot be negative: %w", ErrInvalidProduct)
	}

	// Check for duplicate SKU
	var existingProduct models.Product
	err := s.db.WithContext(ctx).
		Where("organization_id = ? AND sku = ?", orgID, req.Sku).
		First(&existingProduct).Error

	if err == nil {
		// Product with this SKU already exists
		return nil, fmt.Errorf("product with SKU '%s' already exists: %w", req.Sku, ErrDuplicateSKU)
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check for duplicate SKU: %w", err)
	}

	// Convert protobuf status to model status
	status := convertProtoStatusToModel(req.Status)

	// Convert attributes to JSONB
	attributesJSON, err := convertAttributesToJSON(req.Attributes)
	if err != nil {
		return nil, fmt.Errorf("invalid attributes: %w", ErrInvalidProduct)
	}

	// Create product model
	product := &models.Product{
		OrganizationID: orgID,
		SKU:            req.Sku,
		Name:           req.Name,
		Description:    req.Description,
		BasePrice:      req.BasePrice,
		Status:         status,
		Attributes:     attributesJSON,
	}

	// Save to database
	if err := s.db.WithContext(ctx).Create(product).Error; err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// Handle category assignments if provided
	if len(req.CategoryIds) > 0 {
		if err := s.assignCategories(ctx, product.ID, req.CategoryIds, orgID); err != nil {
			return nil, fmt.Errorf("failed to assign categories: %w", err)
		}
	}

	// Convert to protobuf response
	return convertModelToProto(product), nil
}

// Get retrieves a product by ID
func (s *productServiceImpl) Get(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*pb.Product, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.Get")
	defer span.Finish()

	var product models.Product
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		First(&product).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("product with ID '%s' not found: %w", id, ErrProductNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve product: %w", err)
	}

	return convertModelToProto(&product), nil
}

// Update updates an existing product
func (s *productServiceImpl) Update(ctx context.Context, id uuid.UUID, req *pb.UpdateProductRequest, orgID uuid.UUID) (*pb.Product, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.Update")
	defer span.Finish()

	// Retrieve existing product
	var product models.Product
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		First(&product).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("product with ID '%s' not found: %w", id, ErrProductNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve product: %w", err)
	}

	// Update fields if provided
	if req.Name != "" {
		product.Name = req.Name
	}
	if req.Description != "" {
		product.Description = req.Description
	}
	if req.BasePrice > 0 {
		product.BasePrice = req.BasePrice
	}
	if req.Status != pb.ProductStatus_PRODUCT_STATUS_UNSPECIFIED {
		product.Status = convertProtoStatusToModel(req.Status)
	}
	if req.Attributes != nil {
		attributesJSON, err := convertAttributesToJSON(req.Attributes)
		if err != nil {
			return nil, fmt.Errorf("invalid attributes: %w", ErrInvalidProduct)
		}
		product.Attributes = attributesJSON
	}

	// Save updates
	if err := s.db.WithContext(ctx).Save(&product).Error; err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	// Handle category assignments if provided
	if req.CategoryIds != nil {
		// Clear existing categories and assign new ones
		if err := s.db.WithContext(ctx).Exec("DELETE FROM product_categories WHERE product_id = ?", product.ID).Error; err != nil {
			return nil, fmt.Errorf("failed to clear existing categories: %w", err)
		}

		if len(req.CategoryIds) > 0 {
			if err := s.assignCategories(ctx, product.ID, req.CategoryIds, orgID); err != nil {
				return nil, fmt.Errorf("failed to assign categories: %w", err)
			}
		}
	}

	return convertModelToProto(&product), nil
}

// Delete soft-deletes a product
func (s *productServiceImpl) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.Delete")
	defer span.Finish()

	// Check if product exists
	var product models.Product
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		First(&product).Error

	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("product with ID '%s' not found: %w", id, ErrProductNotFound)
	} else if err != nil {
		return fmt.Errorf("failed to retrieve product: %w", err)
	}

	// Check for dependencies (variants, assets, etc.)
	// We perform a cascading delete within a transaction

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Delete variants (hard delete)
		if err := tx.Where("parent_product_id = ?", product.ID).Delete(&models.ProductVariant{}).Error; err != nil {
			return fmt.Errorf("failed to delete variants: %w", err)
		}

		// 2. Delete assets (soft delete via gorm model)
		if err := tx.Where("product_id = ?", product.ID).Delete(&models.Asset{}).Error; err != nil {
			return fmt.Errorf("failed to delete assets: %w", err)
		}

		// 3. Delete product (soft delete)
		if err := tx.Delete(&product).Error; err != nil {
			return fmt.Errorf("failed to delete product: %w", err)
		}

		return nil
	})
}

// List retrieves products with filtering and pagination
func (s *productServiceImpl) List(ctx context.Context, req *pb.ListProductsRequest, orgID uuid.UUID) (*pb.ListProductsResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProductService.List")
	defer span.Finish()

	query := s.db.WithContext(ctx).Where("organization_id = ?", orgID)

	// Apply filters if provided
	if req.Filter != nil {
		// Status filter
		if len(req.Filter.Statuses) > 0 {
			statuses := make([]models.ProductStatus, len(req.Filter.Statuses))
			for i, protoStatus := range req.Filter.Statuses {
				statuses[i] = convertProtoStatusToModel(protoStatus)
			}
			query = query.Where("status IN ?", statuses)
		}

		// Price range filter
		if req.Filter.MinPrice > 0 {
			query = query.Where("base_price >= ?", req.Filter.MinPrice)
		}
		if req.Filter.MaxPrice > 0 {
			query = query.Where("base_price <= ?", req.Filter.MaxPrice)
		}

		// Search query (simple LIKE search for MVP, full-text search later)
		if req.Filter.SearchQuery != "" {
			searchPattern := "%" + req.Filter.SearchQuery + "%"
			query = query.Where("name ILIKE ? OR sku ILIKE ? OR description ILIKE ?",
				searchPattern, searchPattern, searchPattern)
		}
	}

	// Count total items
	var totalItems int64
	if err := query.Model(&models.Product{}).Count(&totalItems).Error; err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}

	// Apply pagination
	page := int32(1)
	pageSize := int32(20)
	if req.Pagination != nil {
		if req.Pagination.Page > 0 {
			page = req.Pagination.Page
		}
		if req.Pagination.PageSize > 0 {
			pageSize = req.Pagination.PageSize
		}
	}

	// Apply sorting
	orderBy := "created_at DESC" // Default sort
	if req.Sort != nil {
		switch req.Sort.Field {
		case pb.ProductsSort_SORT_FIELD_NAME:
			orderBy = "name"
		case pb.ProductsSort_SORT_FIELD_SKU:
			orderBy = "sku"
		case pb.ProductsSort_SORT_FIELD_PRICE:
			orderBy = "base_price"
		case pb.ProductsSort_SORT_FIELD_CREATED_AT:
			orderBy = "created_at"
		case pb.ProductsSort_SORT_FIELD_UPDATED_AT:
			orderBy = "updated_at"
		}

		if req.Sort.Order == pb.SortOrder_SORT_ORDER_DESC {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
	}

	// Retrieve products
	var products []models.Product
	offset := (page - 1) * pageSize
	if err := query.Order(orderBy).Limit(int(pageSize)).Offset(int(offset)).Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve products: %w", err)
	}

	// Convert to protobuf
	pbProducts := make([]*pb.Product, len(products))
	for i, product := range products {
		pbProducts[i] = convertModelToProto(&product)
	}

	// Calculate pagination metadata
	totalPages := (totalItems + int64(pageSize) - 1) / int64(pageSize)

	return &pb.ListProductsResponse{
		Products: pbProducts,
		Pagination: &pb.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int32(totalItems),
			TotalPages: int32(totalPages),
		},
	}, nil
}

// Helper functions

func convertProtoStatusToModel(status pb.ProductStatus) models.ProductStatus {
	switch status {
	case pb.ProductStatus_PRODUCT_STATUS_ACTIVE:
		return models.ProductStatusActive
	case pb.ProductStatus_PRODUCT_STATUS_DISCONTINUED:
		return models.ProductStatusDiscontinued
	default:
		return models.ProductStatusDraft
	}
}

func convertModelStatusToProto(status models.ProductStatus) pb.ProductStatus {
	switch status {
	case models.ProductStatusActive:
		return pb.ProductStatus_PRODUCT_STATUS_ACTIVE
	case models.ProductStatusDiscontinued:
		return pb.ProductStatus_PRODUCT_STATUS_DISCONTINUED
	default:
		return pb.ProductStatus_PRODUCT_STATUS_DRAFT
	}
}

func convertAttributesToJSON(attributes map[string]*pb.AttributeValue) (datatypes.JSON, error) {
	if attributes == nil {
		return datatypes.JSON([]byte("{}")), nil
	}

	// Convert protobuf AttributeValue map to simple map for JSON storage
	simpleMap := make(map[string]interface{})
	for key, value := range attributes {
		switch v := value.Value.(type) {
		case *pb.AttributeValue_StringValue:
			simpleMap[key] = v.StringValue
		case *pb.AttributeValue_NumberValue:
			simpleMap[key] = v.NumberValue
		case *pb.AttributeValue_BooleanValue:
			simpleMap[key] = v.BooleanValue
		case *pb.AttributeValue_DateValue:
			simpleMap[key] = v.DateValue.AsTime()
		}
	}

	// Marshal map to JSON bytes
	jsonBytes, err := json.Marshal(simpleMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes to JSON: %w", err)
	}

	return datatypes.JSON(jsonBytes), nil
}

func convertJSONToAttributes(jsonData datatypes.JSON) map[string]*pb.AttributeValue {
	// For MVP, return empty map
	// Full implementation would parse JSONB and convert to protobuf AttributeValue
	return make(map[string]*pb.AttributeValue)
}

// assignCategories assigns categories to a product
func (s *productServiceImpl) assignCategories(ctx context.Context, productID uuid.UUID, categoryIDs []string, orgID uuid.UUID) error {
	for _, categoryIDStr := range categoryIDs {
		categoryID, err := uuid.Parse(categoryIDStr)
		if err != nil {
			return fmt.Errorf("invalid category ID '%s': %w", categoryIDStr, ErrInvalidProduct)
		}

		// Verify category exists and belongs to same organization
		var category models.Category
		if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", categoryID, orgID).First(&category).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("category with ID '%s' not found: %w", categoryID, ErrCategoryNotFound)
			}
			return fmt.Errorf("failed to verify category: %w", err)
		}

		// Create product-category association
		productCategory := &models.ProductCategory{
			ProductID:  productID,
			CategoryID: categoryID,
		}

		if err := s.db.WithContext(ctx).Create(productCategory).Error; err != nil {
			// Ignore duplicate errors (already assigned)
			if !strings.Contains(err.Error(), "duplicate") {
				return fmt.Errorf("failed to assign category: %w", err)
			}
		}
	}

	return nil
}

func convertModelToProto(product *models.Product) *pb.Product {
	// TODO: Load category IDs from product_categories table
	// For now, return empty array
	categoryIDs := []string{}

	// Debug: ensure BasePrice is int64
	basePrice := product.BasePrice
	if basePrice == 0 && product.BasePrice != 0 {
		// This should not happen, but let's ensure it's properly typed
		basePrice = product.BasePrice
	}

	return &pb.Product{
		Id:             product.ID.String(),
		OrganizationId: product.OrganizationID.String(),
		Sku:            product.SKU,
		Name:           product.Name,
		Description:    product.Description,
		BasePrice:      basePrice,
		Status:         convertModelStatusToProto(product.Status),
		Attributes:     convertJSONToAttributes(product.Attributes),
		CategoryIds:    categoryIDs,
		CreatedAt:      timestamppb.New(product.CreatedAt),
		UpdatedAt:      timestamppb.New(product.UpdatedAt),
	}
}
