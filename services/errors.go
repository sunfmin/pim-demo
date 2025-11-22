package services

import "errors"

// Sentinel errors for service layer
// These errors are used internally and mapped to HTTP error codes by handlers

var (
	// Product errors
	ErrProductNotFound       = errors.New("product not found")
	ErrDuplicateSKU          = errors.New("SKU already exists")
	ErrInvalidProduct        = errors.New("invalid product data")
	ErrProductHasDependencies = errors.New("product has dependencies and cannot be deleted")

	// Category errors
	ErrCategoryNotFound = errors.New("category not found")

	// Variant errors
	ErrVariantNotFound = errors.New("product variant not found")

	// Asset errors
	ErrAssetNotFound       = errors.New("asset not found")
	ErrInvalidAssetFormat  = errors.New("invalid asset format")
	ErrAssetTooLarge       = errors.New("asset file size exceeds limit")

	// Import/Export errors
	ErrImportFailed = errors.New("import operation failed")

	// Authentication/Authorization errors
	ErrUnauthorized        = errors.New("unauthorized access")
	ErrOrganizationMismatch = errors.New("resource belongs to different organization")
)

