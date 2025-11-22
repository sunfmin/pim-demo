package services

import (
	"context"
	"io"

	"github.com/google/uuid"
	pb "github.com/yourorg/pim-demo/api/gen/v1"
)

// ImportService defines the interface for bulk product import operations
type ImportService interface {
	// ImportProducts imports products from CSV data
	ImportProducts(ctx context.Context, orgID uuid.UUID, csvReader io.Reader, mode pb.ImportMode) (*pb.ImportResult, error)

	// ValidateImport validates CSV data without importing
	ValidateImport(ctx context.Context, orgID uuid.UUID, csvReader io.Reader) (*pb.ValidateImportResponse, error)

	// GetImportTemplate generates a CSV template for imports
	GetImportTemplate(ctx context.Context, includeExamples bool) (io.ReadCloser, error)
}
