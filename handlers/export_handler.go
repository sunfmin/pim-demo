package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/services"
)

// ExportHandler handles HTTP requests for export operations
type ExportHandler struct {
	exportService services.ExportService
}

// NewExportHandler creates a new ExportHandler
func NewExportHandler(exportService services.ExportService) *ExportHandler {
	return &ExportHandler{
		exportService: exportService,
	}
}

// Export handles POST /api/v1/products/export
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/export")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse request body
	var req pb.ExportProductsRequest
	if err := ReadJSON(r, &req); err != nil {
		// Empty body is okay, use default filter
		req.Filter = nil
	}

	// Perform export
	csvReader, err := h.exportService.ExportProducts(ctx, orgID, req.Filter)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}
	defer csvReader.Close()

	// Set response headers for file download
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="products-export-%d.csv"`, timestamppb.Now().AsTime().Unix()))
	w.Header().Set("Cache-Control", "no-cache")

	// Stream CSV to response
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, csvReader); err != nil {
		ext.Error.Set(span, true)
		span.LogKV("error", "failed to stream CSV", "err", err.Error())
		// Can't send error response after headers are written
		return
	}
}
