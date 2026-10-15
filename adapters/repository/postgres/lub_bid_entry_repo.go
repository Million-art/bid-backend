package postgres

import (
	"errors"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LUBBidEntryRepository implements ports.LUBBidEntryRepository.
type LUBBidEntryRepository struct {
	db *gorm.DB
}

func NewLUBBidEntryRepository(db *gorm.DB) *LUBBidEntryRepository {
	return &LUBBidEntryRepository{db: db}
}

func (r *LUBBidEntryRepository) Create(e *domain.BidEntry) error {
	return r.db.Create(e).Error
}

func (r *LUBBidEntryRepository) GetByID(id uuid.UUID) (*domain.BidEntry, error) {
	var e domain.BidEntry
	err := r.db.First(&e, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *LUBBidEntryRepository) Update(e *domain.BidEntry) error {
	return r.db.Save(e).Error
}

func (r *LUBBidEntryRepository) GetByProductAndParticipantAndValue(productID uuid.UUID, participantID string, bidValue domain.Money) (*domain.BidEntry, error) {
	var e domain.BidEntry
	err := r.db.Where("product_id = ? AND participant_id = ? AND bid_value = ?", productID, participantID, bidValue).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *LUBBidEntryRepository) ListByProductAndParticipant(productID uuid.UUID, participantID string) ([]domain.BidEntry, error) {
	var entries []domain.BidEntry
	err := r.db.Where("product_id = ? AND participant_id = ?", productID, participantID).
		Order("placed_at ASC").Find(&entries).Error
	return entries, err
}

func (r *LUBBidEntryRepository) ListByProduct(productID uuid.UUID, limit, offset int) ([]domain.BidEntry, int64, error) {
	var total int64
	if err := r.db.Model(&domain.BidEntry{}).Where("product_id = ?", productID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []domain.BidEntry
	if err := r.db.Where("product_id = ?", productID).
		Order("bid_value ASC, placed_at ASC").
		Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// FindWinner returns the entry with the lowest bid_value placed by exactly one participant.
// The SQL logic:
//
//	SELECT bid_value, MAX(id) as id, MAX(participant_id) as participant_id
//	FROM lub_bid_entries
//	WHERE product_id = $1
//	GROUP BY bid_value
//	HAVING COUNT(DISTINCT participant_id) = 1
//	ORDER BY bid_value ASC
//	LIMIT 1
//
// We use COUNT(DISTINCT participant_id) = 1 to find values placed by exactly one person.
func (r *LUBBidEntryRepository) FindWinner(productID uuid.UUID) (*domain.BidEntry, error) {
	type winnerRow struct {
		ID            uuid.UUID
		ParticipantID string
		BidValue      domain.Money
	}
	var row winnerRow
	err := r.db.Raw(`
		SELECT id, participant_id, bid_value
		FROM lub_bid_entries
		WHERE product_id = ?
		  AND id IN (
		    SELECT MAX(id)
		    FROM lub_bid_entries
		    WHERE product_id = ?
		    GROUP BY bid_value
		    HAVING COUNT(DISTINCT participant_id) = 1
		  )
		ORDER BY bid_value ASC
		LIMIT 1
	`, productID, productID).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		return nil, nil // no unique bid
	}
	entry, err := r.GetByID(row.ID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// MarkWinner sets is_winner=true on the winning entry, and sets is_unique=true
// on all entries whose bid_value was placed by exactly one participant.
func (r *LUBBidEntryRepository) MarkWinner(productID uuid.UUID, winnerEntryID uuid.UUID) error {
	// 1. Mark all unique values (bid_value used by exactly 1 distinct participant)
	err := r.db.Exec(`
		UPDATE lub_bid_entries
		SET is_unique = TRUE
		WHERE product_id = ?
		  AND bid_value IN (
		      SELECT bid_value
		      FROM lub_bid_entries
		      WHERE product_id = ?
		      GROUP BY bid_value
		      HAVING COUNT(DISTINCT participant_id) = 1
		  )
	`, productID, productID).Error
	if err != nil {
		return err
	}
	// 2. Mark the winning entry
	return r.db.Model(&domain.BidEntry{}).
		Where("id = ?", winnerEntryID).
		Update("is_winner", true).Error
}

func (r *LUBBidEntryRepository) DeleteEntry(id uuid.UUID) error {
	return r.db.Delete(&domain.BidEntry{}, "id = ?", id).Error
}
