package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Organization represents a multi-tenant container for all PIM data
type Organization struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name      string         `gorm:"not null;size:255"`
	Slug      string         `gorm:"uniqueIndex;not null;size:63"`
	Settings  datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time      `gorm:"not null;default:now()"`
	UpdatedAt time.Time      `gorm:"not null;default:now()"`
}

// BeforeCreate hook to set UUID if not provided
func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for Organization
func (Organization) TableName() string {
	return "organizations"
}
