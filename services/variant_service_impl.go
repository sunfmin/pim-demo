package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// variantServiceImpl implements the VariantService interface
type variantServiceImpl struct {
	db *gorm.DB
}

// NewVariantService creates a new instance of VariantService
func NewVariantService(db *gorm.DB) VariantService {
	return &variantServiceImpl{db: db}
}

// Create creates a new product variant
func (s *variantServiceImpl) Create(ctx context.Context, orgID uuid.UUID, req *pb.CreateVariantRequest) (*models.ProductVariant, error) {
	// Validate parent product exists and belongs to organization
	parentProductID, err := uuid.Parse(req.ParentProductId)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parent product ID", ErrInvalidProduct)
	}

	var parentProduct models.Product
	if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", parentProductID, orgID).First(&parentProduct).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: parent product not found", ErrProductNotFound)
		}
		return nil, fmt.Errorf("failed to fetch parent product: %w", err)
	}

	// Validate variant SKU is unique
	var existingCount int64
	s.db.WithContext(ctx).Model(&models.ProductVariant{}).
		Where("variant_sku = ? AND organization_id = ?", req.VariantSku, orgID).
		Count(&existingCount)
	if existingCount > 0 {
		return nil, fmt.Errorf("%w: variant SKU %s already exists", ErrDuplicateSKU, req.VariantSku)
	}

	// Validate variant attributes are not empty
	if len(req.VariantAttributes) == 0 {
		return nil, fmt.Errorf("%w: variant attributes cannot be empty", ErrInvalidVariantData)
	}

	// Convert protobuf attributes to JSONB
	variantAttrsMap := make(map[string]interface{})
	for key, attr := range req.VariantAttributes {
		switch v := attr.Value.(type) {
		case *pb.AttributeValue_StringValue:
			variantAttrsMap[key] = v.StringValue
		case *pb.AttributeValue_NumberValue:
			variantAttrsMap[key] = v.NumberValue
		}
	}

	variantAttrsJSON, err := json.Marshal(variantAttrsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal variant attributes: %w", err)
	}

	// Create variant
	variant := &models.ProductVariant{
		OrganizationID:    orgID,
		ParentProductID:   parentProductID,
		VariantSKU:        req.VariantSku,
		VariantAttributes: datatypes.JSON(variantAttrsJSON),
		PriceAdjustment:   req.PriceAdjustment,
		InventoryQuantity: req.InventoryQuantity,
		IsActive:          req.IsActive,
	}

	if err := s.db.WithContext(ctx).Create(variant).Error; err != nil {
		return nil, fmt.Errorf("failed to create variant: %w", err)
	}

	return variant, nil
}

// Get retrieves a variant by ID
func (s *variantServiceImpl) Get(ctx context.Context, orgID, variantID uuid.UUID) (*models.ProductVariant, error) {
	var variant models.ProductVariant
	err := s.db.WithContext(ctx).
		Preload("ParentProduct").
		Where("id = ? AND organization_id = ?", variantID, orgID).
		First(&variant).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: variant not found", ErrVariantNotFound)
		}
		return nil, fmt.Errorf("failed to fetch variant: %w", err)
	}

	return &variant, nil
}

// Update updates an existing variant
func (s *variantServiceImpl) Update(ctx context.Context, orgID uuid.UUID, req *pb.UpdateVariantRequest) (*models.ProductVariant, error) {
	variantID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid variant ID", ErrInvalidProduct)
	}

	// Fetch existing variant
	variant, err := s.Get(ctx, orgID, variantID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	updates := make(map[string]interface{})

	if req.VariantAttributes != nil && len(req.VariantAttributes) > 0 {
		variantAttrsMap := make(map[string]interface{})
		for key, attr := range req.VariantAttributes {
			switch v := attr.Value.(type) {
			case *pb.AttributeValue_StringValue:
				variantAttrsMap[key] = v.StringValue
			case *pb.AttributeValue_NumberValue:
				variantAttrsMap[key] = v.NumberValue
			}
		}
		variantAttrsJSON, err := json.Marshal(variantAttrsMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal variant attributes: %w", err)
		}
		updates["variant_attributes"] = datatypes.JSON(variantAttrsJSON)
	}

	if req.PriceAdjustment != 0 || req.PriceAdjustment != variant.PriceAdjustment {
		updates["price_adjustment"] = req.PriceAdjustment
	}

	if req.InventoryQuantity != 0 || req.InventoryQuantity != variant.InventoryQuantity {
		updates["inventory_quantity"] = req.InventoryQuantity
	}

	if req.IsActive != variant.IsActive {
		updates["is_active"] = req.IsActive
	}

	// Apply updates
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(variant).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update variant: %w", err)
		}
	}

	// Reload variant
	return s.Get(ctx, orgID, variantID)
}

// Delete soft-deletes a variant
func (s *variantServiceImpl) Delete(ctx context.Context, orgID, variantID uuid.UUID) error {
	// Verify variant exists and belongs to organization
	variant, err := s.Get(ctx, orgID, variantID)
	if err != nil {
		return err
	}

	// Hard delete (no soft delete for variants)
	if err := s.db.WithContext(ctx).Delete(variant).Error; err != nil {
		return fmt.Errorf("failed to delete variant: %w", err)
	}

	return nil
}

// List retrieves all variants with optional filters
func (s *variantServiceImpl) List(ctx context.Context, orgID uuid.UUID, filters *pb.ListVariantsRequest) ([]*models.ProductVariant, int64, error) {
	query := s.db.WithContext(ctx).
		Preload("ParentProduct").
		Where("organization_id = ?", orgID)

	// Apply filters
	if filters != nil {
		if filters.ParentProductId != "" {
			parentProductID, err := uuid.Parse(filters.ParentProductId)
			if err == nil {
				query = query.Where("parent_product_id = ?", parentProductID)
			}
		}

		if filters.Filter != nil {
			query = query.Where("is_active = ?", filters.Filter.IsActive)
		}
	}

	// Count total
	var total int64
	if err := query.Model(&models.ProductVariant{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count variants: %w", err)
	}

	// Apply pagination
	if filters != nil && filters.Pagination != nil {
		offset := (filters.Pagination.Page - 1) * filters.Pagination.PageSize
		query = query.Offset(int(offset)).Limit(int(filters.Pagination.PageSize))
	}

	// Fetch variants
	var variants []*models.ProductVariant
	if err := query.Find(&variants).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch variants: %w", err)
	}

	return variants, total, nil
}

// ListByProduct retrieves all variants for a specific product
func (s *variantServiceImpl) ListByProduct(ctx context.Context, orgID, productID uuid.UUID) ([]*models.ProductVariant, error) {
	var variants []*models.ProductVariant
	err := s.db.WithContext(ctx).
		Where("parent_product_id = ? AND organization_id = ?", productID, orgID).
		Order("created_at ASC").
		Find(&variants).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch variants for product: %w", err)
	}

	return variants, nil
}

// Generate generates variants from attribute combinations (cartesian product)
func (s *variantServiceImpl) Generate(ctx context.Context, orgID uuid.UUID, req *pb.GenerateVariantsRequest) ([]*models.ProductVariant, error) {
	// Validate parent product exists
	parentProductID, err := uuid.Parse(req.ParentProductId)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid parent product ID", ErrInvalidProduct)
	}

	var parentProduct models.Product
	if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", parentProductID, orgID).First(&parentProduct).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: parent product not found", ErrProductNotFound)
		}
		return nil, fmt.Errorf("failed to fetch parent product: %w", err)
	}

	// Generate all combinations (cartesian product)
	combinations := generateCombinations(req.Attributes)

	variants := make([]*models.ProductVariant, 0, len(combinations))

	// Create a variant for each combination
	for _, combo := range combinations {
		// Generate SKU from pattern
		variantSKU := generateSKUFromPattern(req.SkuPattern, parentProduct.SKU, combo)

		// Check if SKU already exists
		var existingCount int64
		s.db.WithContext(ctx).Model(&models.ProductVariant{}).
			Where("variant_sku = ? AND organization_id = ?", variantSKU, orgID).
			Count(&existingCount)
		if existingCount > 0 {
			// Skip if variant with this SKU already exists
			continue
		}

		// Marshal attributes to JSON
		variantAttrsJSON, err := json.Marshal(combo)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal variant attributes: %w", err)
		}

		variant := &models.ProductVariant{
			OrganizationID:    orgID,
			ParentProductID:   parentProductID,
			VariantSKU:        variantSKU,
			VariantAttributes: datatypes.JSON(variantAttrsJSON),
			PriceAdjustment:   req.DefaultPriceAdjustment,
			InventoryQuantity: req.DefaultInventory,
			IsActive:          true,
		}

		if err := s.db.WithContext(ctx).Create(variant).Error; err != nil {
			return nil, fmt.Errorf("failed to create variant %s: %w", variantSKU, err)
		}

		variants = append(variants, variant)
	}

	return variants, nil
}

// generateCombinations generates all possible combinations of attribute values (cartesian product)
func generateCombinations(attributes []*pb.VariantAttributeDefinition) []map[string]string {
	if len(attributes) == 0 {
		return []map[string]string{}
	}

	// Extract attribute keys and values
	keys := make([]string, 0, len(attributes))
	valuesList := make([][]string, 0, len(attributes))

	for _, attr := range attributes {
		keys = append(keys, attr.AttributeName)
		valuesList = append(valuesList, attr.PossibleValues)
	}

	// Generate cartesian product
	return cartesianProduct(keys, valuesList, 0, make(map[string]string))
}

// cartesianProduct recursively generates all combinations
func cartesianProduct(keys []string, valuesList [][]string, index int, current map[string]string) []map[string]string {
	if index == len(keys) {
		// Base case: all attributes assigned
		result := make(map[string]string)
		for k, v := range current {
			result[k] = v
		}
		return []map[string]string{result}
	}

	var results []map[string]string
	key := keys[index]
	values := valuesList[index]

	for _, value := range values {
		current[key] = value
		combinations := cartesianProduct(keys, valuesList, index+1, current)
		results = append(results, combinations...)
	}

	return results
}

// generateSKUFromPattern generates a SKU from a pattern template
// Pattern example: "{parent_sku}-{size}-{color}"
// Result example: "TSHIRT-001-M-Red"
func generateSKUFromPattern(pattern, parentSKU string, attributes map[string]string) string {
	sku := pattern

	// Replace {parent_sku} placeholder
	sku = strings.ReplaceAll(sku, "{parent_sku}", parentSKU)

	// Replace attribute placeholders
	for key, value := range attributes {
		placeholder := fmt.Sprintf("{%s}", key)
		sku = strings.ReplaceAll(sku, placeholder, value)
	}

	return sku
}
