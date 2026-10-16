package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuctionStatus represents the lifecycle state of a Lowest Unique Bid auction.
type AuctionStatus string

const (
	AuctionStatusDraft     AuctionStatus = "draft"
	AuctionStatusActive    AuctionStatus = "active"
	AuctionStatusPostponed AuctionStatus = "postponed" // CloseAt extended by admin
	AuctionStatusClosed    AuctionStatus = "closed"    // Bidding over, resolving winner
	AuctionStatusExecuted  AuctionStatus = "executed"  // Winner determined
	AuctionStatusCancelled AuctionStatus = "cancelled"
)

// Auction is a Lowest Unique Bid (LUB) auction item.
// Participants pay an entrance fee to unlock bidding.
// The winner is the participant whose bid_value is the lowest value
// placed by exactly one person.
type Auction struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title       string        `gorm:"type:varchar(255);not null"                     json:"title"`
	Description string        `gorm:"type:text"                                      json:"description"`
	ImageURL    string        `gorm:"type:text"                                      json:"imageUrl,omitempty"`
	VideoURL    string        `gorm:"type:text"                                      json:"videoUrl,omitempty"`
	Status      AuctionStatus `gorm:"type:auction_status;not null;default:'draft';index:idx_auctions_status;index:idx_auctions_due_close,priority:1" json:"status"`

	// Financials & Rules
	EntranceFee Money  `gorm:"type:decimal(12,2);not null"              json:"entranceFee"`
	MinBidValue Money  `gorm:"type:decimal(10,2);not null;default:0.01" json:"minBidValue"`
	MaxBidValue *Money `gorm:"type:decimal(10,2)"                       json:"maxBidValue,omitempty"`

	// Schedule
	StartAt    *time.Time `gorm:"column:start_at"                                                          json:"startAt,omitempty"`
	CloseAt    *time.Time `gorm:"column:close_at;index:idx_auctions_due_close,priority:2"                  json:"closeAt,omitempty"`
	ExecutedAt *time.Time `gorm:"column:executed_at"                                                       json:"executedAt,omitempty"`

	// Winner Reference (set after ExecuteDraw)
	WinnerBidID         *uuid.UUID `gorm:"type:uuid;index:idx_auctions_winner_bid"         json:"winnerBidId,omitempty"`
	WinnerParticipantID *string    `gorm:"type:uuid;index:idx_auctions_winner_participant" json:"winnerParticipantId,omitempty"`
	WinnerBidValue      *Money     `gorm:"type:decimal(10,2)"                              json:"winnerBidValue,omitempty"`

	CreatedBy uuid.UUID `gorm:"type:uuid;not null;index:idx_auctions_created_by" json:"createdBy"`
	CreatedAt time.Time `gorm:"index:idx_auctions_created_at"                    json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relations
	Creator *User `gorm:"foreignKey:CreatedBy;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
}

func (Auction) TableName() string { return "auctions" }
