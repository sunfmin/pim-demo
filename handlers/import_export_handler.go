package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/services"
)

const (
	// MaxImportFileSize is the maximum size for CSV import (10 MB)
	MaxImportFileSize = 10 << 20
)

// ImportExportHandler handles HTTP requests for import/export operations
type ImportExportHandler struct {
	importExportService services.ImportExportService
	productService      services.ProductService
}

// NewImportExportHandler creates a new ImportExportHandler
func NewImportExportHandler(importExportService services.ImportExportService, productService services.ProductService) *ImportExportHandler {
	return &ImportExportHandler{
		importExportService: importExportService,
		productService:      productService,
	}
}

// Import handles POST /api/v1/products/import
func (h *ImportExportHandler) Import(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/import")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := ctx.Value("organization_id").(uuid.UUID)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(MaxImportFileSize); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: failed to parse multipart form: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing file in request", services.ErrInvalidProduct), generateRequestID())
		return
	}
	defer file.Close()

	// Validate file type
	if header.Header.Get("Content-Type") != "text/csv" && !isCSVFilename(header.Filename) {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: file must be CSV format", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Get import mode (default to CREATE_OR_UPDATE)
	mode := pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
	modeStr := r.FormValue("mode")
	if modeStr != "" {
		switch modeStr {
		case "create_only":
			mode = pb.ImportMode_IMPORT_MODE_CREATE_ONLY
		case "update_only":
			mode = pb.ImportMode_IMPORT_MODE_UPDATE_ONLY
		case "create_or_update":
			mode = pb.ImportMode_IMPORT_MODE_CREATE_OR_UPDATE
		}
	}

	// Perform import
	result, err := h.importExportService.ImportProducts(ctx, orgID, file, mode)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to response
	resp := &pb.ImportProductsResponse{
		JobId:   uuid.New().String(), // For future async support
		Status:  pb.ImportJobStatus_IMPORT_JOB_STATUS_COMPLETED,
		IsAsync: false,
		Result:  result,
	}

	// Set status to PARTIALLY_COMPLETED if there were errors
	if result.FailedRows > 0 && result.SuccessfulRows > 0 {
		resp.Status = pb.ImportJobStatus_IMPORT_JOB_STATUS_PARTIALLY_COMPLETED
	} else if result.FailedRows > 0 && result.SuccessfulRows == 0 {
		resp.Status = pb.ImportJobStatus_IMPORT_JOB_STATUS_FAILED
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Export handles POST /api/v1/products/export
func (h *ImportExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/export")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := ctx.Value("organization_id").(uuid.UUID)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse request body
	var req pb.ExportProductsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is okay, use default filter
		req.Filter = nil
	}

	// Perform export
	csvReader, err := h.importExportService.ExportProducts(ctx, orgID, req.Filter)
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

// Validate handles POST /api/v1/products/import/validate
func (h *ImportExportHandler) Validate(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/import/validate")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := ctx.Value("organization_id").(uuid.UUID)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(MaxImportFileSize); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: failed to parse multipart form: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing file in request", services.ErrInvalidProduct), generateRequestID())
		return
	}
	defer file.Close()

	// Validate file type
	if header.Header.Get("Content-Type") != "text/csv" && !isCSVFilename(header.Filename) {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: file must be CSV format", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Perform validation
	result, err := h.importExportService.ValidateImport(ctx, orgID, file)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// GetTemplate handles GET /api/v1/products/import/template
func (h *ImportExportHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/products/import/template")
	defer span.Finish()

	// Check if examples should be included
	includeExamples := r.URL.Query().Get("examples") == "true"

	// Generate template
	csvReader, err := h.importExportService.GetImportTemplate(ctx, includeExamples)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}
	defer csvReader.Close()

	// Set response headers for file download
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="product-import-template.csv"`)
	w.Header().Set("Cache-Control", "no-cache")

	// Stream CSV to response
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, csvReader); err != nil {
		ext.Error.Set(span, true)
		span.LogKV("error", "failed to stream CSV template", "err", err.Error())
		return
	}
}

// isCSVFilename checks if filename has .csv extension
func isCSVFilename(filename string) bool {
	return len(filename) > 4 && filename[len(filename)-4:] == ".csv"
}

