package services

import (
	"context"
	"io"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/models"
)

// AssetService defines the interface for asset operations
type AssetService interface {
	// Upload uploads a new asset file and creates database record
	Upload(ctx context.Context, orgID uuid.UUID, req *UploadAssetRequest) (*models.Asset, error)

	// Get retrieves an asset by ID
	Get(ctx context.Context, orgID, assetID uuid.UUID) (*models.Asset, error)

	// Update updates an existing asset metadata
	Update(ctx context.Context, orgID uuid.UUID, req *pb.UpdateAssetRequest) (*models.Asset, error)

	// Delete removes an asset from database and filesystem
	Delete(ctx context.Context, orgID, assetID uuid.UUID) error

	// List retrieves all assets for a product
	List(ctx context.Context, orgID, productID uuid.UUID) ([]*models.Asset, error)

	// SetPrimary sets an asset as the primary image for a product
	SetPrimary(ctx context.Context, orgID, assetID uuid.UUID) error

	// GetFile retrieves the file content of an asset
	GetFile(ctx context.Context, orgID, assetID uuid.UUID) (io.ReadCloser, string, error)
}

// UploadAssetRequest contains data for uploading an asset
type UploadAssetRequest struct {
	ProductID   uuid.UUID
	File        io.Reader
	FileName    string
	ContentType string
	FileSize    int64
	AssetType   string
	AltText     string
	IsPrimary   bool
}

