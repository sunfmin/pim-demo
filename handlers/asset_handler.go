package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/yourorg/pim-demo/api/gen/v1"
	"github.com/yourorg/pim-demo/internal/middleware"
	"github.com/yourorg/pim-demo/internal/models"
	"github.com/yourorg/pim-demo/services"
)

// AssetHandler handles HTTP requests for assets
type AssetHandler struct {
	assetService   services.AssetService
	productService services.ProductService
}

// NewAssetHandler creates a new AssetHandler
func NewAssetHandler(assetService services.AssetService, productService services.ProductService) *AssetHandler {
	return &AssetHandler{
		assetService:   assetService,
		productService: productService,
	}
}

// Upload handles POST /api/v1/assets
func (h *AssetHandler) Upload(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/assets")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(110 << 20); err != nil { // 110 MB max
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Extract product_id
	productIDStr := r.FormValue("product_id")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid product ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Extract file
	file, header, err := r.FormFile("file")
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: file is required", services.ErrInvalidProduct), generateRequestID())
		return
	}
	defer file.Close()

	// Extract optional fields
	assetType := r.FormValue("asset_type")
	if assetType == "" {
		assetType = "image" // Default
	}

	altText := r.FormValue("alt_text")
	isPrimary := r.FormValue("is_primary") == "true"

	// Create upload request
	uploadReq := &services.UploadAssetRequest{
		ProductID:   productID,
		File:        file,
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		FileSize:    header.Size,
		AssetType:   assetType,
		AltText:     altText,
		IsPrimary:   isPrimary,
	}

	// Upload asset
	asset, err := h.assetService.Upload(ctx, orgID, uploadReq)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbAsset := modelToProtoAsset(asset)

	resp := &pb.UploadAssetResponse{
		Asset: pbAsset,
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// Get handles GET /api/v1/assets/{id}
func (h *AssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/assets/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract asset ID from URL
	assetIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid asset ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Fetch asset
	asset, err := h.assetService.Get(ctx, orgID, assetID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbAsset := modelToProtoAsset(asset)

	resp := &pb.GetAssetResponse{
		Asset: pbAsset,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Download handles GET /api/v1/assets/{id}/download
func (h *AssetHandler) Download(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/assets/{id}/download")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract asset ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	path = strings.TrimSuffix(path, "/download")
	assetID, err := uuid.Parse(path)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid asset ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Get file
	file, contentType, err := h.assetService.GetFile(ctx, orgID, assetID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}
	defer file.Close()

	// Set headers
	w.Header().Set("Content-Type", contentType)

	// Stream file
	if _, err := io.Copy(w, file); err != nil {
		// Log error but can't send response headers again
		fmt.Printf("Error streaming file: %v\n", err)
	}
}

// Update handles PUT /api/v1/assets/{id}
func (h *AssetHandler) Update(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "PUT /api/v1/assets/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract asset ID from URL
	assetIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid asset ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Parse request body
	var req pb.UpdateAssetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: %v", services.ErrInvalidProduct, err), generateRequestID())
		return
	}

	// Set asset ID from URL
	req.Id = assetID.String()

	// Update asset
	asset, err := h.assetService.Update(ctx, orgID, &req)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbAsset := modelToProtoAsset(asset)

	resp := &pb.UpdateAssetResponse{
		Asset: pbAsset,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/assets/{id}
func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "DELETE /api/v1/assets/{id}")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract asset ID from URL
	assetIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid asset ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Delete asset
	if err := h.assetService.Delete(ctx, orgID, assetID); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	resp := &pb.DeleteAssetResponse{
		Success: true,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// ListByProduct handles GET /api/v1/products/{id}/assets
func (h *AssetHandler) ListByProduct(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "GET /api/v1/products/{id}/assets")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract product ID from URL
	productIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/products/")
	productIDStr = strings.TrimSuffix(productIDStr, "/assets")
	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid product ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Fetch assets
	assets, err := h.assetService.List(ctx, orgID, productID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbAssets := make([]*pb.Asset, len(assets))
	for i, a := range assets {
		pbAssets[i] = modelToProtoAsset(a)
	}

	resp := &pb.ListAssetsResponse{
		Assets: pbAssets,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// SetPrimary handles POST /api/v1/assets/{id}/set-primary
func (h *AssetHandler) SetPrimary(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "POST /api/v1/assets/{id}/set-primary")
	defer span.Finish()

	// Extract organization ID from context
	orgID, ok := middleware.GetOrganizationID(ctx)
	if !ok {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: missing organization ID", services.ErrUnauthorized), generateRequestID())
		return
	}

	// Extract asset ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	path = strings.TrimSuffix(path, "/set-primary")
	assetID, err := uuid.Parse(path)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, fmt.Errorf("%w: invalid asset ID", services.ErrInvalidProduct), generateRequestID())
		return
	}

	// Set primary
	if err := h.assetService.SetPrimary(ctx, orgID, assetID); err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Fetch updated asset
	asset, err := h.assetService.Get(ctx, orgID, assetID)
	if err != nil {
		ext.Error.Set(span, true)
		HandleServiceError(w, err, generateRequestID())
		return
	}

	// Convert to protobuf response
	pbAsset := modelToProtoAsset(asset)

	resp := &pb.SetPrimaryAssetResponse{
		Asset: pbAsset,
	}

	WriteJSON(w, http.StatusOK, resp)
}

// modelToProtoAsset converts a GORM model to protobuf asset
func modelToProtoAsset(asset *models.Asset) *pb.Asset {
	// Map asset type string to enum
	assetTypeEnum := pb.AssetType_ASSET_TYPE_UNSPECIFIED
	switch asset.AssetType {
	case "image":
		assetTypeEnum = pb.AssetType_ASSET_TYPE_IMAGE
	case "video":
		assetTypeEnum = pb.AssetType_ASSET_TYPE_VIDEO
	case "document":
		assetTypeEnum = pb.AssetType_ASSET_TYPE_DOCUMENT
	}

	return &pb.Asset{
		Id:             asset.ID.String(),
		OrganizationId: asset.OrganizationID.String(),
		ProductId:      asset.ProductID.String(),
		FileName:       asset.FileName,
		StoragePath:    asset.StoragePath,
		ContentType:    asset.ContentType,
		FileSize:       asset.FileSize,
		AssetType:      assetTypeEnum,
		IsPrimary:      asset.IsPrimary,
		DisplayOrder:   int32(asset.DisplayOrder),
		AltText:        asset.AltText,
		CreatedAt:      timestamppb.New(asset.CreatedAt),
	}
}
