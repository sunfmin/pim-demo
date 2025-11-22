package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ProductStatus represents the status of a product
type ProductStatus string

const (
	ProductStatusDraft        ProductStatus = "draft"
	ProductStatusActive       ProductStatus = "active"
	ProductStatusDiscontinued ProductStatus = "discontinued"
)

// Product represents a sellable item in the PIM system
type Product struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index"`
	SKU            string         `gorm:"not null;size:50"`
	Name           string         `gorm:"not null;size:500"`
	Description    string         `gorm:"type:text"`
	BasePrice      int64          `gorm:"not null;default:0"` // Price in cents
	Status         ProductStatus  `gorm:"not null;index"`
	Attributes     datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	SearchVector   string         `gorm:"type:tsvector"` // For full-text search
	CreatedAt      time.Time      `gorm:"not null;default:now()"`
	UpdatedAt      time.Time      `gorm:"not null;default:now()"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	// Relationships
	Organization Organization     `gorm:"foreignKey:OrganizationID"`
	Categories   []Category       `gorm:"many2many:product_categories;"`
	Variants     []ProductVariant `gorm:"foreignKey:ParentProductID"`
	Assets       []Asset          `gorm:"foreignKey:ProductID"`
}

// BeforeCreate hook to set UUID and default status if not provided
func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.Status == "" {
		p.Status = ProductStatusDraft
	}
	return nil
}

// TableName specifies the table name for Product
func (Product) TableName() string {
	return "products"
}
