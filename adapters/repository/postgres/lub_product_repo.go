package postgres

import (
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuctionRepository implements ports.AuctionRepository.
type AuctionRepository struct {
	db *gorm.DB
}

func NewAuctionRepository(db *gorm.DB) *AuctionRepository {
	return &AuctionRepository{db: db}
}

func (r *AuctionRepository) Create(a *domain.Auction) error {
	return r.db.Create(a).Error
}

func (r *AuctionRepository) GetByID(id uuid.UUID) (*domain.Auction, error) {
	var a domain.Auction
	err := r.db.First(&a, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	slice := []domain.Auction{a}
	r.populateWinnerNames(slice)
	a = slice[0]
	return &a, nil
}

func (r *AuctionRepository) Update(a *domain.Auction) error {
	return r.db.Save(a).Error
}

func (r *AuctionRepository) UpdateStatus(id uuid.UUID, status domain.AuctionStatus) error {
	return r.db.Model(&domain.Auction{}).Where("id = ?", id).
		Update("status", status).Error
}

func (r *AuctionRepository) List(status, search string, limit, offset int) ([]domain.Auction, int64, error) {
	q := r.db.Model(&domain.Auction{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if strings.TrimSpace(search) != "" {
		q = q.Where("title ILIKE ?", "%"+strings.TrimSpace(search)+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var auctions []domain.Auction
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&auctions).Error; err != nil {
		return nil, 0, err
	}
	r.populateWinnerNames(auctions)
	return auctions, total, nil
}

func (r *AuctionRepository) ListPublic(limit, offset int) ([]domain.Auction, int64, error) {
	q := r.db.Model(&domain.Auction{}).
		Where("status IN ?", []string{
			string(domain.AuctionStatusActive),
			string(domain.AuctionStatusPostponed),
			string(domain.AuctionStatusClosed),
			string(domain.AuctionStatusExecuted),
		})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var auctions []domain.Auction
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&auctions).Error; err != nil {
		return nil, 0, err
	}
	r.populateWinnerNames(auctions)
	return auctions, total, nil
}

func (r *AuctionRepository) populateWinnerNames(auctions []domain.Auction) {
	if len(auctions) == 0 {
		return
	}
	idSet := make(map[string]struct{})
	for _, a := range auctions {
		if a.WinnerParticipantID != nil {
			pid := strings.TrimSpace(*a.WinnerParticipantID)
			if pid != "" {
				idSet[pid] = struct{}{}
			}
		}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	nameByID := make(map[string]string, len(ids))

	type idNameRow struct {
		ID       string `gorm:"column:id"`
		FullName string `gorm:"column:full_name"`
	}

	var userRows []idNameRow
	if err := r.db.Table("users").Select("id, full_name").Where("id IN ?", ids).Scan(&userRows).Error; err == nil {
		for _, row := range userRows {
			if n := strings.TrimSpace(row.FullName); n != "" {
				nameByID[strings.ToLower(strings.TrimSpace(row.ID))] = n
			}
		}
	}

	var partRows []idNameRow
	if err := r.db.Table("participant_users").Select("id, full_name").Where("id IN ?", ids).Scan(&partRows).Error; err == nil {
		for _, row := range partRows {
			key := strings.ToLower(strings.TrimSpace(row.ID))
			if _, exists := nameByID[key]; !exists {
				if n := strings.TrimSpace(row.FullName); n != "" {
					nameByID[key] = n
				}
			}
		}
	}

	for i := range auctions {
		if auctions[i].WinnerParticipantID != nil {
			key := strings.ToLower(strings.TrimSpace(*auctions[i].WinnerParticipantID))
			if name, ok := nameByID[key]; ok {
				auctions[i].WinnerName = name
			}
		}
	}
}

func (r *AuctionRepository) ListDueForClose() ([]domain.Auction, error) {
	var auctions []domain.Auction
	now := time.Now().UTC()
	err := r.db.
		Where("status IN ? AND close_at IS NOT NULL AND close_at <= ?",
			[]string{string(domain.AuctionStatusActive), string(domain.AuctionStatusPostponed)},
			now).
		Find(&auctions).Error
	return auctions, err
}

func (r *AuctionRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Auction{}, "id = ?", id).Error
}
