package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID         uuid.UUID              `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID              `gorm:"type:uuid;not null;index" json:"userId"`
	ActorRole  string                 `gorm:"type:varchar(32)" json:"actorRole"`
	ActorEmail string                 `gorm:"type:varchar(255)" json:"actorEmail"`
	Action     string                 `gorm:"not null" json:"action"` // e.g., "APPROVE_AGENT", "CREATE_bid"
	Entity     string                 `gorm:"not null" json:"entity"` // e.g., "agent", "bid", "user"
	EntityID   string                 `gorm:"not null" json:"entityId"`
	OldValue   string                 `gorm:"type:text" json:"oldValue"`
	NewValue   string                 `gorm:"type:text" json:"newValue"`
	IPAddress  string                 `json:"ipAddress"`
	UserAgent  string                 `json:"userAgent"`
	Metadata   map[string]interface{} `gorm:"serializer:json" json:"metadata"`
	CreatedAt  time.Time              `json:"createdAt"`
	DeletedAt  gorm.DeletedAt         `gorm:"index" json:"-"`

	// Relationships
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// AuditContext represents the request origin context for logging
type AuditContext struct {
	IPAddress  string
	UserAgent  string
	ActorRole  string
	ActorEmail string
}
