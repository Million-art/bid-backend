package postgres

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepo) GetByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	if err := r.db.Take(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByEmail(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Take(&user, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByPhone(phone string) (*domain.User, error) {
	var user domain.User
	clean := strings.TrimSpace(phone)
	if clean == "" {
		return nil, domain.ErrNotFound
	}

	digits := strings.ReplaceAll(strings.ReplaceAll(clean, "+", ""), " ", "")
	if strings.HasPrefix(digits, "0") {
		digits = "251" + digits[1:]
	}

	withPlus := "+" + digits
	withoutPlus := digits
	localFormat := clean
	if strings.HasPrefix(digits, "251") {
		localFormat = "0" + digits[3:]
	}

	if err := r.db.Where("phone = ? OR phone = ? OR phone = ? OR phone = ?", clean, withPlus, withoutPlus, localFormat).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByTelegramUsername(username string) (*domain.User, error) {
	var user domain.User
	clean := strings.TrimPrefix(strings.TrimSpace(username), "@")
	if clean == "" {
		return nil, domain.ErrNotFound
	}
	if err := r.db.Where("LOWER(telegram_username) = LOWER(?)", clean).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByTelegramID(tgID int64) (*domain.User, error) {
	var user domain.User
	if tgID <= 0 {
		return nil, domain.ErrNotFound
	}
	if err := r.db.Where("telegram_id = ?", tgID).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByEmailIncludeDeleted(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Unscoped().Take(&user, "email = ?", email).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Update(user *domain.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepo) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.User{}, id).Error
}

func (r *UserRepo) GetAdmins() ([]domain.User, error) {
	var users []domain.User
	err := r.db.Where("role IN ?", []domain.UserRole{domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleCompliance}).Find(&users).Error
	return users, err
}

func (r *UserRepo) ListTelegramIDs() ([]int64, error) {
	var ids []int64
	err := r.db.Model(&domain.User{}).
		Where("telegram_id IS NOT NULL AND telegram_id > 0").
		Order("telegram_id ASC").
		Pluck("telegram_id", &ids).Error
	return ids, err
}

// UpsertTelegramProfile updates an existing Telegram-linked user, or creates a
// participant account the same way Telegram Login Widget signup does.
func (r *UserRepo) UpsertTelegramProfile(telegramID int64, username, fullName, phone string) (*domain.User, error) {
	if telegramID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	fullName = strings.TrimSpace(fullName)
	phone = strings.TrimSpace(phone)

	user, err := r.GetByTelegramID(telegramID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	if user != nil {
		changed := false
		if username != "" && user.TelegramUsername != username {
			user.TelegramUsername = username
			changed = true
		}
		if fullName != "" && (user.FullName == "" || strings.HasPrefix(user.FullName, "Telegram User")) {
			user.FullName = fullName
			changed = true
		}
		if phone != "" && strings.TrimSpace(user.Phone) == "" {
			user.Phone = phone
			changed = true
		}
		if changed {
			if err := r.Update(user); err != nil {
				return nil, err
			}
		}
		return user, nil
	}

	if fullName == "" {
		if username != "" {
			fullName = username
		} else {
			fullName = "Telegram User " + strconv.FormatInt(telegramID, 10)
		}
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte("TELEGRAM_ONLY_"+strconv.FormatInt(telegramID, 10)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	tgID := telegramID
	user = &domain.User{
		ID:               uuid.New(),
		FullName:         fullName,
		Email:            "tg_" + strconv.FormatInt(telegramID, 10) + "@carramarket.com",
		Password:         string(hashed),
		Phone:            phone,
		TelegramID:       &tgID,
		TelegramUsername: username,
		JoinedVia:        "telegram",
		Role:             domain.RoleParticipant,
		Status:           domain.StatusActive,
	}
	if err := r.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserCountByMonth returns new user registrations per month for the last N months.
func (r *UserRepo) GetUserCountByMonth(lastMonths int) ([]domain.MonthlyCount, error) {
	var result []domain.MonthlyCount
	cutoff := time.Now().AddDate(0, -lastMonths, 0)
	err := r.db.Model(&domain.User{}).
		Select("to_char(created_at, 'YYYY-MM') as month, count(*) as count").
		Where("created_at >= ?", cutoff).
		Group("to_char(created_at, 'YYYY-MM')").
		Order("month").
		Scan(&result).Error
	return result, err
}
