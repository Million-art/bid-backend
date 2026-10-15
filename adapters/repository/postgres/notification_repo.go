package postgres

import (
	"errors"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) CreateBatch(items []*domain.Notification) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.CreateInBatches(items, 100).Error
}

func (r *NotificationRepo) Create(notification *domain.Notification) error {
	// PostgreSQL jsonb rejects empty string; use "{}" when Metadata is empty
	if notification.Metadata == "" {
		notification.Metadata = "{}"
	}
	return r.db.Create(notification).Error
}

func (r *NotificationRepo) GetByID(id uuid.UUID) (*domain.Notification, error) {
	var notification domain.Notification
	err := r.db.First(&notification, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &notification, nil
}

func (r *NotificationRepo) GetByUserID(userID uuid.UUID, userType string, limit, offset int) ([]domain.Notification, int64, error) {
	var notifications []domain.Notification
	var total int64

	query := r.db.Model(&domain.Notification{}).Where("user_id = ? AND user_type = ?", userID, userType)

	// Exclude expired
	query = query.Where("expires_at IS NULL OR expires_at > ?", time.Now())

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Limit(limit).Offset(offset).Order("created_at desc").Find(&notifications).Error
	return notifications, total, err
}

func (r *NotificationRepo) GetUnreadByUserID(userID uuid.UUID, userType string) ([]domain.Notification, error) {
	var notifications []domain.Notification
	err := r.db.Where("user_id = ? AND user_type = ? AND is_read = ?", userID, userType, false).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("created_at desc").
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepo) GetUnreadCount(userID uuid.UUID, userType string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND user_type = ? AND is_read = ?", userID, userType, false).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Count(&count).Error
	return count, err
}

func (r *NotificationRepo) MarkAsRead(id uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.Notification{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *NotificationRepo) MarkAllAsRead(userID uuid.UUID, userType string) error {
	now := time.Now()
	return r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND user_type = ? AND is_read = ?", userID, userType, false).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *NotificationRepo) MarkByConfirmationID(confirmationID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.Notification{}).Where("confirmation_id = ?", confirmationID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *NotificationRepo) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Notification{}, id).Error
}

func (r *NotificationRepo) DeleteExpired() (int64, error) {
	result := r.db.Where("expires_at < ?", time.Now()).Delete(&domain.Notification{})
	return result.RowsAffected, result.Error
}

func (r *NotificationRepo) DeleteOldRead(daysOld int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -daysOld)
	result := r.db.Where("is_read = ? AND read_at < ?", true, cutoff).Delete(&domain.Notification{})
	return result.RowsAffected, result.Error
}
