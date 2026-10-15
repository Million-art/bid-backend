package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string
type UserStatus string

const (
	RoleSuperAdmin  UserRole = "super_admin"
	RoleAdmin       UserRole = "admin"
	RoleCompliance  UserRole = "compliance"
	RoleAgent       UserRole = "agent"
	RoleParticipant UserRole = "participant"

	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
	StatusPending   UserStatus = "pending"
)

type User struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FullName string    `gorm:"not null" json:"fullName"`
	Email    string    `gorm:"uniqueIndex;not null" json:"email"`
	Password string    `gorm:"column:password_hash;not null" json:"-"` // Never return password
	Phone    string    `json:"phone"`

	TelegramID       *int64   `gorm:"uniqueIndex" json:"telegramId,omitempty"`
	TelegramUsername string   `gorm:"type:varchar(100)" json:"telegramUsername,omitempty"`
	JoinedVia        string   `gorm:"type:varchar(30);default:'web'" json:"joinedVia"`

	Role      UserRole       `gorm:"type:varchar(20);not null" json:"role"`
	Status    UserStatus     `gorm:"type:varchar(20);default:'active'" json:"status"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	}
