package postgres

import (
	"errors"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuctionParticipationRepository implements ports.AuctionParticipationRepository.
type AuctionParticipationRepository struct {
	db *gorm.DB
}

func NewAuctionParticipationRepository(db *gorm.DB) *AuctionParticipationRepository {
	return &AuctionParticipationRepository{db: db}
}

func (r *AuctionParticipationRepository) Create(p *domain.AuctionParticipation) error {
	return r.db.Create(p).Error
}

func (r *AuctionParticipationRepository) GetByAuctionAndParticipant(auctionID uuid.UUID, participantID string) (*domain.AuctionParticipation, error) {
	var p domain.AuctionParticipation
	err := r.db.Where("auction_id = ? AND participant_id = ?", auctionID, participantID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *AuctionParticipationRepository) GetByTransactionID(txID string) (*domain.AuctionParticipation, error) {
	var p domain.AuctionParticipation
	err := r.db.Where("transaction_id = ?", txID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *AuctionParticipationRepository) ListByAuction(auctionID uuid.UUID, limit, offset int) ([]domain.AuctionParticipation, int64, error) {
	var total int64
	if err := r.db.Model(&domain.AuctionParticipation{}).Where("auction_id = ?", auctionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var parts []domain.AuctionParticipation
	if err := r.db.Where("auction_id = ?", auctionID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&parts).Error; err != nil {
		return nil, 0, err
	}
	return parts, total, nil
}

func (r *AuctionParticipationRepository) UpdateStatus(id uuid.UUID, status string) error {
	return r.db.Model(&domain.AuctionParticipation{}).Where("id = ?", id).Update("status", status).Error
}

func (r *AuctionParticipationRepository) CountByAuction(auctionID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&domain.AuctionParticipation{}).Where("auction_id = ? AND status = 'active'", auctionID).Count(&count).Error
	return count, err
}
