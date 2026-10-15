package domain

import (
	"time"

	"github.com/google/uuid"
)

// BidEntry is one bid value that a participant places on a LUB product.
// A participant can hold multiple BidEntry rows for the same product
// (each with a different bid_value). They may update an existing entry's
// value before the product closes.
//
// Uniqueness rule: (product_id, participant_id, bid_value) must be unique —
// the same participant cannot place the exact same value twice.
type BidEntry struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProductID     uuid.UUID `gorm:"type:uuid;not null;index"                       json:"productId"`
	ParticipantID string    `gorm:"type:text;not null;index"                       json:"participantId"` // external participant_users.id

	// The bid value chosen by the participant (e.g. 1.20, 5.00).
	// Always exactly 2 decimal places (DECIMAL(10,2)).
	BidValue Money `gorm:"type:decimal(10,2);not null" json:"bidValue"`

	// Computed / set at draw time
	IsUnique bool `gorm:"default:false" json:"isUnique"` // true if exactly one participant placed this value
	IsWinner bool `gorm:"default:false" json:"isWinner"` // true for the single winning entry

	PlacedAt  time.Time `gorm:"column:placed_at;autoCreateTime" json:"placedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (BidEntry) TableName() string { return "lub_bid_entries" }
