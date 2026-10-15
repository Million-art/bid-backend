package postgres

import (
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RefundRecordRepository struct {
	db *gorm.DB
}

func NewRefundRecordRepository(db *gorm.DB) *RefundRecordRepository {
	return &RefundRecordRepository{db: db}
}

func (r *RefundRecordRepository) Create(record *domain.RefundRecord) error {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return r.db.Create(record).Error
}

func (r *RefundRecordRepository) GetByID(id interface{}) (*domain.RefundRecord, error) {
	var row domain.RefundRecord
	if err := r.db.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *RefundRecordRepository) GetByTxRef(txRef string) (*domain.RefundRecord, error) {
	var row domain.RefundRecord
	if err := r.db.Where("tx_ref = ?", txRef).Order("created_at DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *RefundRecordRepository) UpdateStatus(id interface{}, status, chapaRef, errMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if strings.TrimSpace(chapaRef) != "" {
		updates["chapa_refund_ref"] = strings.TrimSpace(chapaRef)
	}
	updates["error_message"] = errMsg
	return r.db.Model(&domain.RefundRecord{}).Where("id = ?", id).Updates(updates).Error
}

func (r *RefundRecordRepository) ListAll(search string, limit, offset int) ([]domain.RefundRecord, int64, error) {
	q := r.db.Model(&domain.RefundRecord{})
	if s := strings.TrimSpace(search); s != "" {
		term := "%" + s + "%"
		q = q.Where("tx_ref ILIKE ? OR chapa_refund_ref ILIKE ?", term, term)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []domain.RefundRecord
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
