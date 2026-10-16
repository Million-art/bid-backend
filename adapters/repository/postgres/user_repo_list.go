package postgres

import (
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"gorm.io/gorm"
)

func (r *UserRepo) List(page, limit int) ([]domain.User, int64, error) {
	return r.listQuery(r.db.Model(&domain.User{}), page, limit)
}

func (r *UserRepo) ListByRoles(roles []domain.UserRole, page, limit int) ([]domain.User, int64, error) {
	return r.ListByRolesWithFilter(roles, "", "", page, limit)
}

func (r *UserRepo) ListByRolesWithFilter(roles []domain.UserRole, status string, search string, page, limit int) ([]domain.User, int64, error) {
	base := r.db.Model(&domain.User{})
	if len(roles) > 0 {
		roleStrs := make([]string, len(roles))
		for i, ro := range roles {
			roleStrs[i] = string(ro)
		}
		base = base.Where("role IN ?", roleStrs)
	}
	if status != "" && (status == string(domain.StatusActive) || status == string(domain.StatusSuspended) || status == string(domain.StatusPending)) {
		base = base.Where("status = ?", status)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		pattern := "%" + search + "%"
		base = base.Where("full_name ILIKE ? OR email ILIKE ?", pattern, pattern)
	}
	return r.listQuery(base, page, limit)
}

func (r *UserRepo) listQuery(base *gorm.DB, page, limit int) ([]domain.User, int64, error) {
	var users []domain.User
	var total int64
	offset := (page - 1) * limit

	// Run Count on a session clone so it doesn't alter base and break the subsequent Find (GORM Count can clear clauses).
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := base.
		Limit(limit).Offset(offset).
		Order("created_at desc").
		Find(&users).Error

	return users, total, err
}

func (r *UserRepo) Count(filter map[string]interface{}) (int64, error) {
	var total int64
	query := r.db.Model(&domain.User{})

	if role, ok := filter["role"]; ok {
		query = query.Where("role = ?", role)
	}
	if status, ok := filter["status"]; ok {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
