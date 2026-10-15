package domain

import (
	"time"

	"gorm.io/gorm"
)

// ParticipantUser mirrors participant_users table (client-backend signups).
type ParticipantUser struct {
	ID               string         `gorm:"type:uuid;primaryKey;size:36" json:"id"`
	Email            string         `gorm:"type:varchar(255);index:idx_participant_email" json:"email"`
	FullName         string         `gorm:"type:varchar(255);not null;column:full_name" json:"fullName"`
	Phone            string         `gorm:"type:varchar(50);index:idx_participant_phone" json:"phone"`
	AvailableBalance Money          `gorm:"type:decimal(12,2);default:0;not null;column:available_balance" json:"availableBalance"`
	PendingBalance   Money          `gorm:"type:decimal(12,2);default:0;not null;column:pending_balance" json:"pendingBalance"`
	Location         string         `gorm:"type:varchar(255)" json:"location"`
	AuthProvider     string         `gorm:"type:varchar(50);not null;default:password;column:auth_provider" json:"authProvider"`
	Status           string         `gorm:"type:varchar(20);default:active;not null;column:status" json:"status"` // active, suspended
	CreatedAt        time.Time      `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        time.Time      `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ParticipantUser) TableName() string {
	return "participant_users"
}
