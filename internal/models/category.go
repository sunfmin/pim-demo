package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Category represents a product category with hierarchical support
type Category struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index"`
	ParentID       *uuid.UUID `gorm:"type:uuid;index"` // NULL for root categories
	Name           string     `gorm:"not null;size:255"`
	Slug           string     `gorm:"not null;size:255"`
	Description    string     `gorm:"type:text"`
	DisplayOrder   int        `gorm:"not null;default:0;index"`
	IsActive       bool       `gorm:"not null;default:true"`
	CreatedAt      time.Time  `gorm:"not null;default:now()"`
	UpdatedAt      time.Time  `gorm:"not null;default:now()"`

	// Relationships
	Organization Organization `gorm:"foreignKey:OrganizationID"`
	Parent       *Category    `gorm:"foreignKey:ParentID"`
	Children     []Category   `gorm:"foreignKey:ParentID"`
	Products     []Product    `gorm:"many2many:product_categories;"`
}

// BeforeCreate hook to set UUID if not provided
func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for Category
func (Category) TableName() string {
	return "categories"
}

