package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/yourorg/pim-demo/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateTestOrganization creates a test organization in the database
func CreateTestOrganization(t *testing.T, db *gorm.DB, overrides map[string]interface{}) *models.Organization {
	t.Helper()

	org := &models.Organization{
		Name:     "Test Organization",
		Slug:     "test-org-" + uuid.New().String()[:8],
		Settings: datatypes.JSON([]byte("{}")),
	}

	// Apply overrides
	if name, ok := overrides["name"].(string); ok {
		org.Name = name
	}
	if slug, ok := overrides["slug"].(string); ok {
		org.Slug = slug
	}
	if settings, ok := overrides["settings"].(datatypes.JSON); ok {
		org.Settings = settings
	}

	if err := db.Create(org).Error; err != nil {
		t.Fatalf("failed to create test organization: %v", err)
	}

	return org
}

// CreateTestProduct creates a test product in the database
func CreateTestProduct(t *testing.T, db *gorm.DB, orgID uuid.UUID, overrides map[string]interface{}) *models.Product {
	t.Helper()

	product := &models.Product{
		OrganizationID: orgID,
		SKU:            "TEST-SKU-" + uuid.New().String()[:8],
		Name:           "Test Product",
		Description:    "Test product description",
		BasePrice:      1999, // $19.99
		Status:         models.ProductStatusDraft,
		Attributes:     datatypes.JSON([]byte("{}")),
	}

	// Apply overrides
	if sku, ok := overrides["sku"].(string); ok {
		product.SKU = sku
	}
	if name, ok := overrides["name"].(string); ok {
		product.Name = name
	}
	if description, ok := overrides["description"].(string); ok {
		product.Description = description
	}
	if basePrice, ok := overrides["base_price"].(int64); ok {
		product.BasePrice = basePrice
	}
	if status, ok := overrides["status"].(models.ProductStatus); ok {
		product.Status = status
	}
	if attributes, ok := overrides["attributes"].(datatypes.JSON); ok {
		product.Attributes = attributes
	}

	if err := db.Create(product).Error; err != nil {
		t.Fatalf("failed to create test product: %v", err)
	}

	return product
}

// CreateTestProductBatch creates multiple test products for performance testing
func CreateTestProductBatch(t *testing.T, db *gorm.DB, orgID uuid.UUID, count int) []*models.Product {
	t.Helper()

	products := make([]*models.Product, count)
	for i := 0; i < count; i++ {
		products[i] = CreateTestProduct(t, db, orgID, map[string]interface{}{
			"sku":  uuid.New().String(),
			"name": "Bulk Product " + uuid.New().String()[:8],
		})
	}

	return products
}

// CreateTestCategory creates a test category in the database
func CreateTestCategory(t *testing.T, db *gorm.DB, orgID uuid.UUID, overrides map[string]interface{}) *models.Category {
	t.Helper()

	category := &models.Category{
		OrganizationID: orgID,
		Name:           "Test Category",
		Slug:           "test-category-" + uuid.New().String()[:8],
		Description:    "Test category description",
		DisplayOrder:   0,
		IsActive:       true,
	}

	// Apply overrides
	if name, ok := overrides["name"].(string); ok {
		category.Name = name
	}
	if slug, ok := overrides["slug"].(string); ok {
		category.Slug = slug
	}
	if description, ok := overrides["description"].(string); ok {
		category.Description = description
	}
	if displayOrder, ok := overrides["display_order"].(int); ok {
		category.DisplayOrder = displayOrder
	}
	if isActive, ok := overrides["is_active"].(bool); ok {
		category.IsActive = isActive
	}
	if parentID, ok := overrides["parent_id"].(*uuid.UUID); ok {
		category.ParentID = parentID
	}

	if err := db.Create(category).Error; err != nil {
		t.Fatalf("failed to create test category: %v", err)
	}

	return category
}

// CreateTestCategoryTree creates a hierarchical category structure for testing
func CreateTestCategoryTree(t *testing.T, db *gorm.DB, orgID uuid.UUID) (*models.Category, *models.Category, *models.Category) {
	t.Helper()

	// Create root category
	root := CreateTestCategory(t, db, orgID, map[string]interface{}{
		"name": "Electronics",
		"slug": "electronics",
	})

	// Create child category
	child := CreateTestCategory(t, db, orgID, map[string]interface{}{
		"name":      "Computers",
		"slug":      "computers",
		"parent_id": &root.ID,
	})

	// Create grandchild category
	grandchild := CreateTestCategory(t, db, orgID, map[string]interface{}{
		"name":      "Laptops",
		"slug":      "laptops",
		"parent_id": &child.ID,
	})

	return root, child, grandchild
}

// CreateTestVariant creates a test product variant in the database
func CreateTestVariant(t *testing.T, db *gorm.DB, parentProductID uuid.UUID, orgID uuid.UUID, overrides map[string]interface{}) *models.ProductVariant {
	t.Helper()

	variant := &models.ProductVariant{
		OrganizationID:    orgID,
		ParentProductID:   parentProductID,
		VariantSKU:        "TEST-VARIANT-" + uuid.New().String()[:8],
		VariantAttributes: datatypes.JSON([]byte("{}")),
		PriceAdjustment:   0,
		InventoryQuantity: 100,
		IsActive:          true,
	}

	// Apply overrides
	if variantSKU, ok := overrides["variant_sku"].(string); ok {
		variant.VariantSKU = variantSKU
	}
	if variantAttributes, ok := overrides["variant_attributes"].(datatypes.JSON); ok {
		variant.VariantAttributes = variantAttributes
	}
	if priceAdjustment, ok := overrides["price_adjustment"].(int64); ok {
		variant.PriceAdjustment = priceAdjustment
	}
	if inventoryQuantity, ok := overrides["inventory_quantity"].(int32); ok {
		variant.InventoryQuantity = inventoryQuantity
	}
	if isActive, ok := overrides["is_active"].(bool); ok {
		variant.IsActive = isActive
	}

	if err := db.Create(variant).Error; err != nil {
		t.Fatalf("failed to create test variant: %v", err)
	}

	return variant
}

// CreateTestVariantSet creates multiple variants for a parent product
func CreateTestVariantSet(t *testing.T, db *gorm.DB, parentProductID uuid.UUID, orgID uuid.UUID, sizes []string, colors []string) []*models.ProductVariant {
	t.Helper()

	variants := make([]*models.ProductVariant, 0)
	for _, size := range sizes {
		for _, color := range colors {
			attributesJSON, _ := json.Marshal(map[string]string{
				"size":  size,
				"color": color,
			})
			variant := CreateTestVariant(t, db, parentProductID, orgID, map[string]interface{}{
				"variant_sku":        fmt.Sprintf("VARIANT-%s-%s-%s", size, color, uuid.New().String()[:4]),
				"variant_attributes": datatypes.JSON(attributesJSON),
			})
			variants = append(variants, variant)
		}
	}

	return variants
}

// CreateTestAsset creates a test asset in the database and filesystem
func CreateTestAsset(t *testing.T, db *gorm.DB, productID uuid.UUID, orgID uuid.UUID, storageDir string, overrides map[string]interface{}) *models.Asset {
	t.Helper()

	// Generate unique storage path
	assetID := uuid.New()
	fileName := "test-asset.jpg"
	if fn, ok := overrides["file_name"].(string); ok {
		fileName = fn
	}

	storagePath := fmt.Sprintf("%s/%s/%s/%s", storageDir, orgID.String(), productID.String(), fileName)

	// Ensure directory exists
	os.MkdirAll(fmt.Sprintf("%s/%s/%s", storageDir, orgID.String(), productID.String()), 0755)

	// Create fake file
	os.WriteFile(storagePath, []byte("fake-file-content"), 0644)

	asset := &models.Asset{
		ID:             assetID,
		OrganizationID: orgID,
		ProductID:      productID,
		FileName:       fileName,
		StoragePath:    storagePath,
		ContentType:    "image/jpeg",
		FileSize:       100,
		AssetType:      "image",
		IsPrimary:      false,
		DisplayOrder:   0,
		AltText:        "",
	}

	// Apply overrides
	if contentType, ok := overrides["content_type"].(string); ok {
		asset.ContentType = contentType
	}
	if fileSize, ok := overrides["file_size"].(int64); ok {
		asset.FileSize = fileSize
	}
	if assetType, ok := overrides["asset_type"].(string); ok {
		asset.AssetType = assetType
	}
	if isPrimary, ok := overrides["is_primary"].(bool); ok {
		asset.IsPrimary = isPrimary
	}
	if displayOrder, ok := overrides["display_order"].(int); ok {
		asset.DisplayOrder = displayOrder
	}
	if altText, ok := overrides["alt_text"].(string); ok {
		asset.AltText = altText
	}

	if err := db.Create(asset).Error; err != nil {
		t.Fatalf("failed to create test asset: %v", err)
	}

	return asset
}

// CreateTestImageFile creates a temporary test image file
func CreateTestImageFile(t *testing.T) (*os.File, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test-image-*.jpg")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Write fake JPEG header
	tmpFile.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}

	return tmpFile, cleanup
}

