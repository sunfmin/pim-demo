package services

import (
	"context"
	"io"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// ExportService defines the interface for bulk product export operations
type ExportService interface {
	// ExportProducts exports products to CSV format
	ExportProducts(ctx context.Context, orgID uuid.UUID, filter *pb.ProductsFilter) (io.ReadCloser, error)
}
