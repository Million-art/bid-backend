package domain

import (
	"time"

	"github.com/google/uuid"
)

// Bid represents a single secret bid placed by an authorized participant on an auction.
// Uniqueness rule: (auction_id, participant_id, bid_value) must be unique.
type Bid struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"                                                                                      json:"id"`
	AuctionID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_bid_auction_participant_value,priority:1;index:idx_bid_auction_value,priority:1"                   json:"auctionId"`
	ParticipantID string    `gorm:"type:uuid;not null;uniqueIndex:ux_bid_auction_participant_value,priority:2;index:idx_bid_participant"                                json:"participantId"`
	BidValue      Money     `gorm:"type:decimal(10,2);not null;uniqueIndex:ux_bid_auction_participant_value,priority:3;index:idx_bid_auction_value,priority:2"          json:"bidValue"`
	IsUnique      bool      `gorm:"not null;default:false"                                                                                                              json:"isUnique"`
	IsWinner      bool      `gorm:"not null;default:false;index:idx_bid_winner"                                                                                         json:"isWinner"`
	PlacedAt      time.Time `gorm:"column:placed_at;autoCreateTime"                                                                                                     json:"placedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`

	// Relations
	Auction *Auction `gorm:"foreignKey:AuctionID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

func (Bid) TableName() string { return "auction_bids" }
