package ports

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// AuctionRepository manages Lowest Unique Bid auctions.
type AuctionRepository interface {
	Create(a *domain.Auction) error
	GetByID(id uuid.UUID) (*domain.Auction, error)
	Update(a *domain.Auction) error
	UpdateStatus(id uuid.UUID, status domain.AuctionStatus) error
	List(status string, search string, limit, offset int) ([]domain.Auction, int64, error)
	ListPublic(limit, offset int) ([]domain.Auction, int64, error)
	// ListDueForClose returns active auctions whose close_at has passed (for the scheduler).
	ListDueForClose() ([]domain.Auction, error)
	Delete(id uuid.UUID) error
}

// AuctionParticipationRepository manages entrance-fee payment records.
type AuctionParticipationRepository interface {
	Create(p *domain.AuctionParticipation) error
	GetByAuctionAndParticipant(auctionID uuid.UUID, participantID string) (*domain.AuctionParticipation, error)
	GetByTransactionID(txID string) (*domain.AuctionParticipation, error)
	ListByAuction(auctionID uuid.UUID, limit, offset int) ([]domain.AuctionParticipation, int64, error)
	ListByParticipant(participantID string, limit, offset int) ([]domain.AuctionParticipation, int64, error)
	UpdateStatus(id uuid.UUID, status string) error
	CountByAuction(auctionID uuid.UUID) (int64, error)
}

// BidRepository manages the secret bid values participants place.
type BidRepository interface {
	Create(b *domain.Bid) error
	GetByID(id uuid.UUID) (*domain.Bid, error)
	Update(b *domain.Bid) error
	GetByAuctionAndParticipantAndValue(auctionID uuid.UUID, participantID string, bidValue domain.Money) (*domain.Bid, error)
	ListByAuctionAndParticipant(auctionID uuid.UUID, participantID string) ([]domain.Bid, error)
	ListByAuction(auctionID uuid.UUID, limit, offset int) ([]domain.Bid, int64, error)
	FindWinner(auctionID uuid.UUID) (*domain.Bid, error)
	MarkWinner(auctionID uuid.UUID, winnerBidID uuid.UUID) error
	DeleteBid(id uuid.UUID) error
}
