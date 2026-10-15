package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProductStatus represents the lifecycle state of a LUB product/auction.
type ProductStatus string

const (
	ProductStatusDraft     ProductStatus = "draft"
	ProductStatusActive    ProductStatus = "active"
	ProductStatusPostponed ProductStatus = "postponed" // CloseAt extended by admin
	ProductStatusClosed    ProductStatus = "closed"    // Bidding over, resolving winner
	ProductStatusExecuted  ProductStatus = "executed"  // Winner determined
	ProductStatusCancelled ProductStatus = "cancelled"
)

// Product is a LUB (Lowest Unique Bid) auction item.
// Participants pay an entrance fee to be allowed to place bid values.
// The winner is the participant whose bid_value is the lowest value
// placed by exactly one person.
type Product struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string        `gorm:"type:varchar(255);not null"                     json:"name"`
	Description string        `gorm:"type:text"                                      json:"description"`
	ImageURL    string        `gorm:"type:text"                                      json:"imageUrl,omitempty"`
	VideoURL    string        `gorm:"type:text"                                      json:"videoUrl,omitempty"`
	Status      ProductStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`

	// Entrance fee that every participant must pay once to be allowed to place bids.
	EntranceFee Money `gorm:"type:decimal(12,2);not null" json:"entranceFee"`

	// Optional bid value bounds. MinBidValue defaults to 0.01 if not set.
	MinBidValue Money  `gorm:"type:decimal(10,2);not null;default:0.01" json:"minBidValue"`
	MaxBidValue *Money `gorm:"type:decimal(10,2)"                      json:"maxBidValue,omitempty"`

	// Lifecycle timestamps
	StartAt  *time.Time `gorm:"column:start_at"  json:"startAt,omitempty"`
	CloseAt  *time.Time `gorm:"column:close_at"  json:"closeAt,omitempty"`
	ExecutedAt *time.Time `gorm:"column:executed_at" json:"executedAt,omitempty"`

	// Postpone tracking
	PostponeCount int        `gorm:"default:0"              json:"postponeCount"`
	PostponedAt   *time.Time `gorm:"column:postponed_at"    json:"postponedAt,omitempty"`

	// Suspension (staff)
	SuspendedReason  string     `gorm:"type:text"               json:"suspendedReason,omitempty"`
	SuspendedAt      *time.Time `gorm:"column:suspended_at"     json:"suspendedAt,omitempty"`
	ResumeFromStatus ProductStatus `gorm:"type:varchar(20)"     json:"resumeFromStatus,omitempty"`

	// Winner info (set after ExecuteDraw)
	WinnerID       *string    `gorm:"type:text;index"         json:"winnerId,omitempty"`       // participant_user ID (external, string)
	WinnerBidValue *Money     `gorm:"type:decimal(10,2)"      json:"winnerBidValue,omitempty"` // the winning bid amount
	WinnerName     string     `gorm:"type:varchar(255)"       json:"winnerName,omitempty"`
	WinnerEmail    string     `gorm:"type:varchar(255)"       json:"winnerEmail,omitempty"`
	WinnerPhone    string     `gorm:"type:varchar(50)"        json:"winnerPhone,omitempty"`

	// Commission / financials (platform revenue from entrance fees)
	CommissionRate       Money `gorm:"type:decimal(5,4);default:0.10" json:"commissionRate"` // e.g. 0.10 = 10%
	TotalEntranceFeeRevenue Money `gorm:"type:decimal(12,2);default:0" json:"totalEntranceFeeRevenue"`

	// Rejection
	RejectionReason string `gorm:"type:text" json:"rejectionReason,omitempty"`

	// Who created this product (admin user)
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"createdBy"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Product) TableName() string { return "lub_products" }
