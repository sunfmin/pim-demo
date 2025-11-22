package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ProductVariant represents a specific configuration of a parent product
// Example: A t-shirt product may have variants for different sizes and colors
type ProductVariant struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrganizationID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	ParentProductID   uuid.UUID      `gorm:"type:uuid;not null;index"`
	VariantSKU        string         `gorm:"not null;size:50"`
	VariantAttributes datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	PriceAdjustment   int64          `gorm:"not null;default:0"` // Price difference from parent in cents (can be negative)
	InventoryQuantity int32          `gorm:"not null;default:0"`
	IsActive          bool           `gorm:"not null;default:true"`
	CreatedAt         time.Time      `gorm:"not null;default:now()"`
	UpdatedAt         time.Time      `gorm:"not null;default:now()"`

	// Relationships
	Organization  Organization `gorm:"foreignKey:OrganizationID"`
	ParentProduct Product      `gorm:"foreignKey:ParentProductID"`
}

// TableName specifies the table name for ProductVariant
func (ProductVariant) TableName() string {
	return "product_variants"
}
