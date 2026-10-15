package domain

import (
	"time"

	"github.com/google/uuid"
)

// LUBParticipation records that a participant has paid the entrance fee
// for a specific product. This grants them the right to place bid entries.
// There is at most one participation record per (product_id, participant_id).
type LUBParticipation struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProductID     uuid.UUID `gorm:"type:uuid;not null;index"                       json:"productId"`
	ParticipantID string    `gorm:"type:text;not null;index"                       json:"participantId"` // external participant_users.id

	// Participant snapshot at purchase time (denormalised for audit/display)
	ParticipantName  string `gorm:"type:varchar(255)" json:"participantName,omitempty"`
	ParticipantEmail string `gorm:"type:varchar(255)" json:"participantEmail,omitempty"`
	ParticipantPhone string `gorm:"type:varchar(50)"  json:"participantPhone,omitempty"`

	// Payment reference (Chapa tx_ref)
	TransactionID string `gorm:"type:varchar(255);uniqueIndex" json:"transactionId"`
	FeePaid       Money  `gorm:"type:decimal(12,2);not null"   json:"feePaid"`

	// Status: active | refunded
	Status string `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`

	JoinedAt  time.Time `gorm:"column:joined_at;autoCreateTime" json:"joinedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (LUBParticipation) TableName() string { return "lub_participations" }

const (
	LUBParticipationStatusActive   = "active"
	LUBParticipationStatusRefunded = "refunded"
)
