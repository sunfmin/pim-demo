package models

import (
	"time"

	"github.com/google/uuid"
)

// Asset represents a digital file associated with a product
type Asset struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index"`
	ProductID      uuid.UUID `gorm:"type:uuid;not null;index"`
	FileName       string    `gorm:"not null;size:255"`
	StoragePath    string    `gorm:"uniqueIndex;not null;size:1000"`
	ContentType    string    `gorm:"not null;size:100"`
	FileSize       int64     `gorm:"not null"`
	AssetType      string    `gorm:"not null"` // image, video, document
	IsPrimary      bool      `gorm:"not null;default:false;index"`
	DisplayOrder   int       `gorm:"not null;default:0;index"`
	AltText        string    `gorm:"size:500"`
	CreatedAt      time.Time `gorm:"not null;default:now()"`

	// Relationships
	Organization Organization `gorm:"foreignKey:OrganizationID"`
	Product      Product      `gorm:"foreignKey:ProductID"`
}

// TableName specifies the table name for Asset
func (Asset) TableName() string {
	return "assets"
}

