package postgres

import (
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LUBProductRepository implements ports.LUBProductRepository.
type LUBProductRepository struct {
	db *gorm.DB
}

func NewLUBProductRepository(db *gorm.DB) *LUBProductRepository {
	return &LUBProductRepository{db: db}
}

func (r *LUBProductRepository) Create(p *domain.Product) error {
	return r.db.Create(p).Error
}

func (r *LUBProductRepository) GetByID(id uuid.UUID) (*domain.Product, error) {
	var p domain.Product
	err := r.db.First(&p, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *LUBProductRepository) Update(p *domain.Product) error {
	return r.db.Save(p).Error
}

func (r *LUBProductRepository) UpdateStatus(id uuid.UUID, status domain.ProductStatus) error {
	return r.db.Model(&domain.Product{}).Where("id = ?", id).
		Update("status", status).Error
}

func (r *LUBProductRepository) List(status, search string, limit, offset int) ([]domain.Product, int64, error) {
	q := r.db.Model(&domain.Product{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if strings.TrimSpace(search) != "" {
		q = q.Where("name ILIKE ?", "%"+strings.TrimSpace(search)+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var products []domain.Product
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *LUBProductRepository) ListPublic(limit, offset int) ([]domain.Product, int64, error) {
	q := r.db.Model(&domain.Product{}).
		Where("status IN ?", []string{
			string(domain.ProductStatusActive),
			string(domain.ProductStatusPostponed),
			string(domain.ProductStatusClosed),
			string(domain.ProductStatusExecuted),
		})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var products []domain.Product
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *LUBProductRepository) ListDueForClose() ([]domain.Product, error) {
	var products []domain.Product
	now := time.Now().UTC()
	err := r.db.
		Where("status IN ? AND close_at IS NOT NULL AND close_at <= ?",
			[]string{string(domain.ProductStatusActive), string(domain.ProductStatusPostponed)},
			now).
		Find(&products).Error
	return products, err
}

func (r *LUBProductRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Product{}, "id = ?", id).Error
}
