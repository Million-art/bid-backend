package ports

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// LUBProductRepository manages LUB auction products.
type LUBProductRepository interface {
	Create(p *domain.Product) error
	GetByID(id uuid.UUID) (*domain.Product, error)
	Update(p *domain.Product) error
	UpdateStatus(id uuid.UUID, status domain.ProductStatus) error
	List(status string, search string, limit, offset int) ([]domain.Product, int64, error)
	ListPublic(limit, offset int) ([]domain.Product, int64, error)
	// ListDueForClose returns active products whose close_at has passed (for the scheduler).
	ListDueForClose() ([]domain.Product, error)
	Delete(id uuid.UUID) error
}

// LUBParticipationRepository manages entrance-fee payment records.
type LUBParticipationRepository interface {
	// Create inserts a new participation record (idempotent on transaction_id).
	Create(p *domain.LUBParticipation) error
	// GetByProductAndParticipant fetches the single participation record for a user+product.
	GetByProductAndParticipant(productID uuid.UUID, participantID string) (*domain.LUBParticipation, error)
	// GetByTransactionID fetches by Chapa tx_ref (idempotency check).
	GetByTransactionID(txID string) (*domain.LUBParticipation, error)
	// ListByProduct returns all participations for an admin overview.
	ListByProduct(productID uuid.UUID, limit, offset int) ([]domain.LUBParticipation, int64, error)
	// UpdateStatus changes the status (e.g. active → refunded).
	UpdateStatus(id uuid.UUID, status string) error
	// CountByProduct returns the number of participants for a product.
	CountByProduct(productID uuid.UUID) (int64, error)
}

// LUBBidEntryRepository manages the bid values participants place.
type LUBBidEntryRepository interface {
	// Create inserts a new bid entry. Returns ErrConflict if (product_id, participant_id, bid_value) already exists.
	Create(e *domain.BidEntry) error
	// GetByID returns a single bid entry.
	GetByID(id uuid.UUID) (*domain.BidEntry, error)
	// Update saves changes to a bid entry (e.g. new bid_value).
	Update(e *domain.BidEntry) error
	// GetByProductAndParticipantAndValue fetches a specific (product, participant, value) triple.
	GetByProductAndParticipantAndValue(productID uuid.UUID, participantID string, bidValue domain.Money) (*domain.BidEntry, error)
	// ListByProductAndParticipant returns all bid entries for one user on one product.
	ListByProductAndParticipant(productID uuid.UUID, participantID string) ([]domain.BidEntry, error)
	// ListByProduct returns all bid entries for a product (used at draw time and admin overview).
	ListByProduct(productID uuid.UUID, limit, offset int) ([]domain.BidEntry, int64, error)
	// FindWinner returns the entry with the lowest bid_value placed by exactly one participant.
	// Returns (nil, nil) when no unique bid exists (all values were duplicated).
	FindWinner(productID uuid.UUID) (*domain.BidEntry, error)
	// MarkWinner marks IsUnique=true and IsWinner=true on the winning entry and
	// sets IsUnique=true on all other entries with a unique value.
	MarkWinner(productID uuid.UUID, winnerEntryID uuid.UUID) error
	// DeleteEntry removes a bid entry (admin correction only).
	DeleteEntry(id uuid.UUID) error
}
