package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// assetServiceImpl implements the AssetService interface
type assetServiceImpl struct {
	db         *gorm.DB
	storageDir string
}

// NewAssetService creates a new instance of AssetService
func NewAssetService(db *gorm.DB, storageDir string) AssetService {
	return &assetServiceImpl{
		db:         db,
		storageDir: storageDir,
	}
}

// Supported MIME types and file extensions
var (
	supportedImageTypes = map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}

	supportedVideoTypes = map[string]bool{
		"video/mp4":  true,
		"video/webm": true,
	}

	supportedDocumentTypes = map[string]bool{
		"application/pdf": true,
	}

	maxImageSize    int64 = 10 * 1024 * 1024  // 10 MB
	maxVideoSize    int64 = 100 * 1024 * 1024 // 100 MB
	maxDocumentSize int64 = 10 * 1024 * 1024  // 10 MB
	maxAssetsPerProduct     = 50
)

// Upload uploads a new asset file and creates database record
func (s *assetServiceImpl) Upload(ctx context.Context, orgID uuid.UUID, req *UploadAssetRequest) (*models.Asset, error) {
	// Validate product exists
	var product models.Product
	if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", req.ProductID, orgID).First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: product not found", ErrProductNotFound)
		}
		return nil, fmt.Errorf("failed to fetch product: %w", err)
	}

	// Check asset count limit
	var assetCount int64
	s.db.WithContext(ctx).Model(&models.Asset{}).
		Where("product_id = ? AND organization_id = ?", req.ProductID, orgID).
		Count(&assetCount)
	if assetCount >= int64(maxAssetsPerProduct) {
		return nil, fmt.Errorf("%w: maximum %d assets per product", ErrInvalidProduct, maxAssetsPerProduct)
	}

	// Validate file type
	if err := validateFileType(req.ContentType, req.AssetType); err != nil {
		return nil, err
	}

	// Validate file size
	if err := validateFileSize(req.FileSize, req.AssetType); err != nil {
		return nil, err
	}

	// Generate storage path
	assetID := uuid.New()
	ext := filepath.Ext(req.FileName)
	storagePath := generateStoragePath(s.storageDir, orgID, req.ProductID, assetID, ext)

	// Ensure directory exists
	dir := filepath.Dir(storagePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Save file to storage
	file, err := os.Create(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, req.File); err != nil {
		os.Remove(storagePath) // Clean up on failure
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Handle primary image
	if req.IsPrimary {
		// Unset current primary
		s.db.WithContext(ctx).Model(&models.Asset{}).
			Where("product_id = ? AND organization_id = ? AND is_primary = ?", req.ProductID, orgID, true).
			Update("is_primary", false)
	}

	// Create asset record
	asset := &models.Asset{
		ID:             assetID,
		OrganizationID: orgID,
		ProductID:      req.ProductID,
		FileName:       req.FileName,
		StoragePath:    storagePath,
		ContentType:    req.ContentType,
		FileSize:       req.FileSize,
		AssetType:      req.AssetType,
		IsPrimary:      req.IsPrimary,
		DisplayOrder:   int(assetCount), // Append to end
		AltText:        req.AltText,
	}

	if err := s.db.WithContext(ctx).Create(asset).Error; err != nil {
		os.Remove(storagePath) // Clean up on failure
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}

	return asset, nil
}

// Get retrieves an asset by ID
func (s *assetServiceImpl) Get(ctx context.Context, orgID, assetID uuid.UUID) (*models.Asset, error) {
	var asset models.Asset
	err := s.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", assetID, orgID).
		First(&asset).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("%w: asset not found", ErrAssetNotFound)
		}
		return nil, fmt.Errorf("failed to fetch asset: %w", err)
	}

	return &asset, nil
}

// Update updates an existing asset metadata
func (s *assetServiceImpl) Update(ctx context.Context, orgID uuid.UUID, req *pb.UpdateAssetRequest) (*models.Asset, error) {
	assetID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid asset ID", ErrInvalidProduct)
	}

	// Fetch existing asset
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	updates := make(map[string]interface{})

	if req.AltText != "" {
		updates["alt_text"] = req.AltText
	}

	if req.DisplayOrder != 0 {
		updates["display_order"] = req.DisplayOrder
	}

	// Apply updates
	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(asset).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update asset: %w", err)
		}
	}

	// Reload asset
	return s.Get(ctx, orgID, assetID)
}

// Delete removes an asset from database and filesystem
func (s *assetServiceImpl) Delete(ctx context.Context, orgID, assetID uuid.UUID) error {
	// Fetch asset
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return err
	}

	// Delete from database
	if err := s.db.WithContext(ctx).Delete(asset).Error; err != nil {
		return fmt.Errorf("failed to delete asset: %w", err)
	}

	// Delete file from storage
	if err := os.Remove(asset.StoragePath); err != nil && !os.IsNotExist(err) {
		// Log error but don't fail - file might already be gone
		fmt.Printf("Warning: failed to delete file %s: %v\n", asset.StoragePath, err)
	}

	return nil
}

// List retrieves all assets for a product
func (s *assetServiceImpl) List(ctx context.Context, orgID, productID uuid.UUID) ([]*models.Asset, error) {
	var assets []*models.Asset
	err := s.db.WithContext(ctx).
		Where("product_id = ? AND organization_id = ?", productID, orgID).
		Order("is_primary DESC, display_order ASC, created_at ASC").
		Find(&assets).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch assets: %w", err)
	}

	return assets, nil
}

// SetPrimary sets an asset as the primary image for a product
func (s *assetServiceImpl) SetPrimary(ctx context.Context, orgID, assetID uuid.UUID) error {
	// Fetch asset
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return err
	}

	// Transaction to unset old primary and set new
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Unset current primary
		if err := tx.Model(&models.Asset{}).
			Where("product_id = ? AND organization_id = ? AND is_primary = ?", asset.ProductID, orgID, true).
			Update("is_primary", false).Error; err != nil {
			return fmt.Errorf("failed to unset current primary: %w", err)
		}

		// Set new primary
		if err := tx.Model(asset).Update("is_primary", true).Error; err != nil {
			return fmt.Errorf("failed to set new primary: %w", err)
		}

		return nil
	})
}

// GetFile retrieves the file content of an asset
func (s *assetServiceImpl) GetFile(ctx context.Context, orgID, assetID uuid.UUID) (io.ReadCloser, string, error) {
	// Fetch asset
	asset, err := s.Get(ctx, orgID, assetID)
	if err != nil {
		return nil, "", err
	}

	// Open file
	file, err := os.Open(asset.StoragePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("%w: file not found in storage", ErrAssetNotFound)
		}
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}

	return file, asset.ContentType, nil
}

// validateFileType validates the file MIME type
func validateFileType(contentType, assetType string) error {
	contentType = strings.ToLower(strings.TrimSpace(contentType))

	switch assetType {
	case "image":
		if !supportedImageTypes[contentType] {
			return fmt.Errorf("%w: unsupported image type %s", ErrInvalidAssetFormat, contentType)
		}
	case "video":
		if !supportedVideoTypes[contentType] {
			return fmt.Errorf("%w: unsupported video type %s", ErrInvalidAssetFormat, contentType)
		}
	case "document":
		if !supportedDocumentTypes[contentType] {
			return fmt.Errorf("%w: unsupported document type %s", ErrInvalidAssetFormat, contentType)
		}
	default:
		return fmt.Errorf("%w: invalid asset type %s", ErrInvalidProduct, assetType)
	}

	return nil
}

// validateFileSize validates the file size based on asset type
func validateFileSize(fileSize int64, assetType string) error {
	var maxSize int64

	switch assetType {
	case "image":
		maxSize = maxImageSize
	case "video":
		maxSize = maxVideoSize
	case "document":
		maxSize = maxDocumentSize
	default:
		return fmt.Errorf("%w: invalid asset type", ErrInvalidProduct)
	}

	if fileSize > maxSize {
		return fmt.Errorf("%w: file size %d exceeds maximum %d bytes", ErrAssetTooLarge, fileSize, maxSize)
	}

	return nil
}

// generateStoragePath generates a unique storage path for an asset
func generateStoragePath(baseDir string, orgID, productID, assetID uuid.UUID, ext string) string {
	return filepath.Join(baseDir, orgID.String(), productID.String(), fmt.Sprintf("%s%s", assetID.String(), ext))
}

