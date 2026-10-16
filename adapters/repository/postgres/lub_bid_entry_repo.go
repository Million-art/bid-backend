package postgres

import (
	"errors"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BidRepository implements ports.BidRepository.
type BidRepository struct {
	db *gorm.DB
}

func NewBidRepository(db *gorm.DB) *BidRepository {
	return &BidRepository{db: db}
}

func (r *BidRepository) Create(b *domain.Bid) error {
	return r.db.Create(b).Error
}

func (r *BidRepository) GetByID(id uuid.UUID) (*domain.Bid, error) {
	var b domain.Bid
	err := r.db.First(&b, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *BidRepository) Update(b *domain.Bid) error {
	return r.db.Save(b).Error
}

func (r *BidRepository) GetByAuctionAndParticipantAndValue(auctionID uuid.UUID, participantID string, bidValue domain.Money) (*domain.Bid, error) {
	var b domain.Bid
	err := r.db.Where("auction_id = ? AND participant_id = ? AND bid_value = ?", auctionID, participantID, bidValue).First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *BidRepository) ListByAuctionAndParticipant(auctionID uuid.UUID, participantID string) ([]domain.Bid, error) {
	var bids []domain.Bid
	err := r.db.Where("auction_id = ? AND participant_id = ?", auctionID, participantID).
		Order("placed_at ASC").Find(&bids).Error
	return bids, err
}

func (r *BidRepository) ListByAuction(auctionID uuid.UUID, limit, offset int) ([]domain.Bid, int64, error) {
	var total int64
	if err := r.db.Model(&domain.Bid{}).Where("auction_id = ?", auctionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var bids []domain.Bid
	if err := r.db.Where("auction_id = ?", auctionID).
		Order("bid_value ASC, placed_at ASC").
		Limit(limit).Offset(offset).Find(&bids).Error; err != nil {
		return nil, 0, err
	}
	return bids, total, nil
}

// FindWinner returns the bid with the lowest bid_value placed by exactly one participant.
func (r *BidRepository) FindWinner(auctionID uuid.UUID) (*domain.Bid, error) {
	type winnerRow struct {
		ID            uuid.UUID
		ParticipantID string
		BidValue      domain.Money
	}
	var row winnerRow
	err := r.db.Raw(`
		SELECT id, participant_id, bid_value
		FROM auction_bids
		WHERE auction_id = ?
		  AND id IN (
		    SELECT MAX(id)
		    FROM auction_bids
		    WHERE auction_id = ?
		    GROUP BY bid_value
		    HAVING COUNT(DISTINCT participant_id) = 1
		  )
		ORDER BY bid_value ASC
		LIMIT 1
	`, auctionID, auctionID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	return r.GetByID(row.ID)
}

// MarkWinner sets is_winner=true on the winning bid, and sets is_unique=true
// on all bids whose bid_value was placed by exactly one participant.
func (r *BidRepository) MarkWinner(auctionID uuid.UUID, winnerBidID uuid.UUID) error {
	err := r.db.Exec(`
		UPDATE auction_bids
		SET is_unique = TRUE
		WHERE auction_id = ?
		  AND bid_value IN (
		      SELECT bid_value
		      FROM auction_bids
		      WHERE auction_id = ?
		      GROUP BY bid_value
		      HAVING COUNT(DISTINCT participant_id) = 1
		  )
	`, auctionID, auctionID).Error
	if err != nil {
		return err
	}
	return r.db.Model(&domain.Bid{}).
		Where("id = ?", winnerBidID).
		Update("is_winner", true).Error
}

func (r *BidRepository) DeleteBid(id uuid.UUID) error {
	return r.db.Delete(&domain.Bid{}, "id = ?", id).Error
}
