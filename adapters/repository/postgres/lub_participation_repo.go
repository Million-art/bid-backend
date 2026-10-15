package postgres

import (
	"errors"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LUBParticipationRepository implements ports.LUBParticipationRepository.
type LUBParticipationRepository struct {
	db *gorm.DB
}

func NewLUBParticipationRepository(db *gorm.DB) *LUBParticipationRepository {
	return &LUBParticipationRepository{db: db}
}

func (r *LUBParticipationRepository) Create(p *domain.LUBParticipation) error {
	return r.db.Create(p).Error
}

func (r *LUBParticipationRepository) GetByProductAndParticipant(productID uuid.UUID, participantID string) (*domain.LUBParticipation, error) {
	var p domain.LUBParticipation
	err := r.db.Where("product_id = ? AND participant_id = ?", productID, participantID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not found is valid — caller checks nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *LUBParticipationRepository) GetByTransactionID(txID string) (*domain.LUBParticipation, error) {
	var p domain.LUBParticipation
	err := r.db.Where("transaction_id = ?", txID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *LUBParticipationRepository) ListByProduct(productID uuid.UUID, limit, offset int) ([]domain.LUBParticipation, int64, error) {
	var total int64
	if err := r.db.Model(&domain.LUBParticipation{}).Where("product_id = ?", productID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var parts []domain.LUBParticipation
	if err := r.db.Where("product_id = ?", productID).Order("joined_at DESC").Limit(limit).Offset(offset).Find(&parts).Error; err != nil {
		return nil, 0, err
	}
	return parts, total, nil
}

func (r *LUBParticipationRepository) UpdateStatus(id uuid.UUID, status string) error {
	return r.db.Model(&domain.LUBParticipation{}).Where("id = ?", id).Update("status", status).Error
}

func (r *LUBParticipationRepository) CountByProduct(productID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&domain.LUBParticipation{}).Where("product_id = ? AND status = 'active'", productID).Count(&count).Error
	return count, err
}
