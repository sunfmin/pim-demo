package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductCategory represents the many-to-many relationship between products and categories
type ProductCategory struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID  uuid.UUID `gorm:"type:uuid;not null;index"`
	CategoryID uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt  time.Time `gorm:"not null;default:now()"`

	// Relationships
	Product  Product  `gorm:"foreignKey:ProductID"`
	Category Category `gorm:"foreignKey:CategoryID"`
}

// BeforeCreate hook to set UUID if not provided
func (pc *ProductCategory) BeforeCreate(tx *gorm.DB) error {
	if pc.ID == uuid.Nil {
		pc.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for ProductCategory
func (ProductCategory) TableName() string {
	return "product_categories"
}
