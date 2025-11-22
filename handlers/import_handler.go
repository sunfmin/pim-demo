package handlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/services"
)

const (
	// MaxImportFileSize is the maximum size for CSV import (10 MB)
	MaxImportFileSize = 10 << 20
)

// ImportHandler handles HTTP requests for import operations
type ImportHandler struct {
	importService services.ImportService
}

// NewImportHandler creates a new ImportHandler
func NewImportHandler(importService services.ImportService) *ImportHandler {
	return &ImportHandler{
		importService: importService,
	}
}

// Import handles POST /api/v1/products/import
func (h *ImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/import")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
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
	result, err := h.importService.ImportProducts(ctx, orgID, file, mode)
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

// Validate handles POST /api/v1/products/import/validate
func (h *ImportHandler) Validate(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/products/import/validate")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
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
	result, err := h.importService.ValidateImport(ctx, orgID, file)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// GetTemplate handles GET /api/v1/products/import/template
func (h *ImportHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/products/import/template")
	defer span.Finish()

	// Check if examples should be included
	includeExamples := r.URL.Query().Get("examples") == "true"

	// Generate template
	csvReader, err := h.importService.GetImportTemplate(ctx, includeExamples)
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
