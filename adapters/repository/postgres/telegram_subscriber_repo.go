package postgres

import (
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TelegramSubscriberRepo struct {
	db *gorm.DB
}

func NewTelegramSubscriberRepository(db *gorm.DB) *TelegramSubscriberRepo {
	return &TelegramSubscriberRepo{db: db}
}

func (r *TelegramSubscriberRepo) Upsert(sub *domain.TelegramSubscriber) error {
	if sub == nil || sub.TelegramID == 0 {
		return nil
	}
	now := time.Now().UTC()
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = now
	}
	sub.UpdatedAt = now
	sub.TelegramUsername = strings.TrimPrefix(strings.TrimSpace(sub.TelegramUsername), "@")
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "telegram_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"telegram_username", "full_name", "updated_at",
		}),
	}).Create(sub).Error
}

func (r *TelegramSubscriberRepo) ListTelegramIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Model(&domain.TelegramSubscriber{}).
		Order("telegram_id ASC").
		Pluck("telegram_id", &ids).Error
	return ids, err
}

func (r *TelegramSubscriberRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&domain.TelegramSubscriber{}).Count(&n).Error
	return n, err
}
