package domain

import (
	"time"

	"gorm.io/gorm"
)

// ParticipantUser represents a participant account.
type ParticipantUser struct {
	ID        string         `gorm:"type:uuid;primaryKey;size:36"                        json:"id"`
	Email     string         `gorm:"type:varchar(255);index:idx_participant_email"       json:"email"`
	FullName  string         `gorm:"type:varchar(255);not null;column:full_name"         json:"fullName"`
	Phone     string         `gorm:"type:varchar(50);index:idx_participant_phone"        json:"phone"`
	Status    string         `gorm:"type:varchar(20);default:'active';not null;column:status;index:idx_participant_status" json:"status"` // active, suspended
	CreatedAt time.Time      `gorm:"column:created_at;index:idx_participant_created_at"  json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at"                                   json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index"                                               json:"-"`
}

func (ParticipantUser) TableName() string {
	return "participant_users"
}
