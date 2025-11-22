package handlers

import (
	"errors"
	"net/http"

	"github.com/yourorg/pim-demo/services"
)

// ErrorCode represents an HTTP error response structure
type ErrorCode struct {
	Code       string // Machine-readable error code
	Message    string // Human-readable error message
	StatusCode int    // HTTP status code
	ServiceErr error  // Corresponding service-layer sentinel error
}

// HTTP error code definitions
var (
	// Product errors (400, 404, 409)
	ErrCodeProductNotFound = ErrorCode{
		Code:       "PRODUCT_NOT_FOUND",
		Message:    "The requested product was not found",
		StatusCode: http.StatusNotFound,
		ServiceErr: services.ErrProductNotFound,
	}

	ErrCodeDuplicateSKU = ErrorCode{
		Code:       "DUPLICATE_SKU",
		Message:    "A product with this SKU already exists",
		StatusCode: http.StatusConflict,
		ServiceErr: services.ErrDuplicateSKU,
	}

	ErrCodeInvalidProductData = ErrorCode{
		Code:       "INVALID_PRODUCT_DATA",
		Message:    "The product data is invalid",
		StatusCode: http.StatusBadRequest,
		ServiceErr: services.ErrInvalidProduct,
	}

	ErrCodeProductInUse = ErrorCode{
		Code:       "PRODUCT_IN_USE",
		Message:    "Product has dependencies and cannot be deleted",
		StatusCode: http.StatusConflict,
		ServiceErr: services.ErrProductHasDependencies,
	}

	// Category errors (404)
	ErrCodeCategoryNotFound = ErrorCode{
		Code:       "CATEGORY_NOT_FOUND",
		Message:    "The requested category was not found",
		StatusCode: http.StatusNotFound,
		ServiceErr: services.ErrCategoryNotFound,
	}

	// Variant errors (400, 404)
	ErrCodeVariantNotFound = ErrorCode{
		Code:       "VARIANT_NOT_FOUND",
		Message:    "The requested product variant was not found",
		StatusCode: http.StatusNotFound,
		ServiceErr: services.ErrVariantNotFound,
	}

	ErrCodeInvalidVariantData = ErrorCode{
		Code:       "INVALID_VARIANT_DATA",
		Message:    "The variant data is invalid",
		StatusCode: http.StatusBadRequest,
		ServiceErr: services.ErrInvalidVariantData,
	}

	// Asset errors (400, 404, 413)
	ErrCodeAssetNotFound = ErrorCode{
		Code:       "ASSET_NOT_FOUND",
		Message:    "The requested asset was not found",
		StatusCode: http.StatusNotFound,
		ServiceErr: services.ErrAssetNotFound,
	}

	ErrCodeInvalidFileFormat = ErrorCode{
		Code:       "INVALID_FILE_FORMAT",
		Message:    "The uploaded file format is not supported",
		StatusCode: http.StatusBadRequest,
		ServiceErr: services.ErrInvalidAssetFormat,
	}

	ErrCodeFileTooLarge = ErrorCode{
		Code:       "FILE_TOO_LARGE",
		Message:    "The uploaded file exceeds the maximum size limit",
		StatusCode: http.StatusRequestEntityTooLarge,
		ServiceErr: services.ErrAssetTooLarge,
	}

	ErrCodeAssetLimitExceeded = ErrorCode{
		Code:       "ASSET_LIMIT_EXCEEDED",
		Message:    "The maximum number of assets per product has been reached",
		StatusCode: http.StatusBadRequest,
		ServiceErr: services.ErrAssetLimitExceeded,
	}

	// Import/Export errors (400)
	ErrCodeImportFailed = ErrorCode{
		Code:       "IMPORT_FAILED",
		Message:    "The import operation failed",
		StatusCode: http.StatusBadRequest,
		ServiceErr: services.ErrImportFailed,
	}

	// Authentication/Authorization errors (401, 403)
	ErrCodeUnauthorized = ErrorCode{
		Code:       "UNAUTHORIZED",
		Message:    "Authentication is required to access this resource",
		StatusCode: http.StatusUnauthorized,
		ServiceErr: services.ErrUnauthorized,
	}

	ErrCodeForbidden = ErrorCode{
		Code:       "FORBIDDEN",
		Message:    "You do not have permission to access this resource",
		StatusCode: http.StatusForbidden,
		ServiceErr: services.ErrOrganizationMismatch,
	}

	// Generic errors (400, 500)
	ErrCodeInvalidRequest = ErrorCode{
		Code:       "INVALID_REQUEST",
		Message:    "The request is invalid",
		StatusCode: http.StatusBadRequest,
		ServiceErr: nil, // Generic validation error
	}

	ErrCodeInternalError = ErrorCode{
		Code:       "INTERNAL_ERROR",
		Message:    "An internal server error occurred",
		StatusCode: http.StatusInternalServerError,
		ServiceErr: nil, // Generic unexpected error
	}
)

// errorCodeMap maps service errors to HTTP error codes
var errorCodeMap = map[error]ErrorCode{
	services.ErrProductNotFound:        ErrCodeProductNotFound,
	services.ErrDuplicateSKU:           ErrCodeDuplicateSKU,
	services.ErrInvalidProduct:         ErrCodeInvalidProductData,
	services.ErrProductHasDependencies: ErrCodeProductInUse,
	services.ErrCategoryNotFound:       ErrCodeCategoryNotFound,
	services.ErrVariantNotFound:        ErrCodeVariantNotFound,
	services.ErrInvalidVariantData:     ErrCodeInvalidVariantData,
	services.ErrAssetNotFound:          ErrCodeAssetNotFound,
	services.ErrInvalidAssetFormat:     ErrCodeInvalidFileFormat,
	services.ErrAssetTooLarge:          ErrCodeFileTooLarge,
	services.ErrAssetLimitExceeded:     ErrCodeAssetLimitExceeded,
	services.ErrImportFailed:           ErrCodeImportFailed,
	services.ErrUnauthorized:           ErrCodeUnauthorized,
	services.ErrOrganizationMismatch:   ErrCodeForbidden,
}

// MapServiceError maps a service error to an HTTP error code
func MapServiceError(err error) ErrorCode {
	// Check for direct match
	for serviceErr, errorCode := range errorCodeMap {
		if errors.Is(err, serviceErr) {
			return errorCode
		}
	}

	// Default to internal server error for unknown errors
	return ErrCodeInternalError
}
