package postgres

import (
	"errors"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"gorm.io/gorm"
)

// ParticipantUserRepo reads from participant_users (client app signups).
type ParticipantUserRepo struct {
	db *gorm.DB
}

func NewParticipantUserRepository(db *gorm.DB) *ParticipantUserRepo {
	return &ParticipantUserRepo{db: db}
}

func (r *ParticipantUserRepo) ListAllIDs(limit, offset int) ([]string, int64, error) {
	base := r.db.Model(&domain.ParticipantUser{})
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ids []string
	err := base.Select("id").Order("created_at DESC").Limit(limit).Offset(offset).Scan(&ids).Error
	return ids, total, err
}

func (r *ParticipantUserRepo) List(limit, offset int, search, status string) ([]domain.ParticipantUser, int64, error) {
	base := r.db.Model(&domain.ParticipantUser{})
	if search != "" {
		pattern := "%" + search + "%"
		base = base.Where("full_name ILIKE ? OR email ILIKE ? OR phone ILIKE ?", pattern, pattern, pattern)
	}
	if status != "" && (status == "active" || status == "suspended") {
		base = base.Where("status = ?", status)
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []domain.ParticipantUser
	err := base.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

func (r *ParticipantUserRepo) GetByID(id string) (*domain.ParticipantUser, error) {
	var u domain.ParticipantUser
	if err := r.db.Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *ParticipantUserRepo) GetByPhone(phone string) (*domain.ParticipantUser, error) {
	var u domain.ParticipantUser
	if err := r.db.Where("phone = ?", phone).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *ParticipantUserRepo) Create(user *domain.ParticipantUser) error {
	return r.db.Create(user).Error
}

func (r *ParticipantUserRepo) UpsertByPhone(user *domain.ParticipantUser) error {
	var existing domain.ParticipantUser
	err := r.db.Where("phone = ?", user.Phone).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Not found, create it
			return r.db.Create(user).Error
		}
		return err // other DB error
	}

	// Found existing user. Assign the found ID back to the pointer
	// so the caller can use it for foreign keys.
	user.ID = existing.ID

	// Only update fields if necessary.
	// For POS, we just want to ensure the user exists. We can update FullName if we want.
	updates := map[string]interface{}{
		"full_name":  user.FullName,
		"updated_at": user.UpdatedAt,
	}
	return r.db.Model(&domain.ParticipantUser{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func (r *ParticipantUserRepo) Count(filter map[string]interface{}) (int64, error) {
	var total int64
	query := r.db.Model(&domain.ParticipantUser{})

	if status, ok := filter["status"]; ok {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *ParticipantUserRepo) UpdateStatus(id string, status string) error {
	return r.db.Model(&domain.ParticipantUser{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ParticipantUserRepo) Delete(id string) error {
	return r.db.Delete(&domain.ParticipantUser{}, "id = ?", id).Error
}

func (r *ParticipantUserRepo) GetCountByMonth(lastMonths int) ([]domain.MonthlyCount, error) {
	if lastMonths <= 0 {
		lastMonths = 12
	}
	cutoff := time.Now().AddDate(0, -lastMonths, 0)
	var result []domain.MonthlyCount
	err := r.db.Model(&domain.ParticipantUser{}).
		Select("to_char(created_at, 'YYYY-MM') as month, count(*) as count").
		Where("created_at >= ?", cutoff).
		Group("to_char(created_at, 'YYYY-MM')").
		Order("month").
		Scan(&result).Error
	return result, err
}
