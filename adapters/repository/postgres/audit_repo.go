package postgres

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"gorm.io/gorm"
)

type AuditLogRepo struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(log *domain.AuditLog) error {
	return r.db.Create(log).Error
}

func (r *AuditLogRepo) List(filter map[string]interface{}, limit, offset int) ([]domain.AuditLog, int64, error) {
	var logs []domain.AuditLog
	var total int64

	query := r.db.Model(&domain.AuditLog{}).Preload("User")

	allowedFilters := map[string]bool{
		"action":    true,
		"entity":    true,
		"entity_id": true,
		"user_id":   true,
	}

	for k, v := range filter {
		if v != "" && allowedFilters[k] {
			query = query.Where(k+" = ?", v)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
