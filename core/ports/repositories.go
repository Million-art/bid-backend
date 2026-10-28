package ports

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

type AuditLogService interface {
	Log(log *domain.AuditLog, ctx *domain.AuditContext) error
}

type UserRepository interface {
	Create(user *domain.User) error
	GetByID(id uuid.UUID) (*domain.User, error)
	GetByEmail(email string) (*domain.User, error)
	GetByPhone(phone string) (*domain.User, error)
	GetByTelegramUsername(username string) (*domain.User, error)
	GetByTelegramID(tgID int64) (*domain.User, error)
	GetByEmailIncludeDeleted(email string) (*domain.User, error)
	Update(user *domain.User) error
	List(page, limit int) ([]domain.User, int64, error)
	ListByRoles(roles []domain.UserRole, page, limit int) ([]domain.User, int64, error)
	ListByRolesWithFilter(roles []domain.UserRole, status string, search string, page, limit int) ([]domain.User, int64, error)
	GetAdmins() ([]domain.User, error)
	Count(filter map[string]interface{}) (int64, error)
	Delete(id uuid.UUID) error
	ListTelegramIDs() ([]int64, error)
	UpsertTelegramProfile(telegramID int64, username, fullName, phone string) (*domain.User, error)
}

type AuditLogRepository interface {
	Create(log *domain.AuditLog) error
	List(filter map[string]interface{}, limit, offset int) ([]domain.AuditLog, int64, error)
}

type ParticipantUserRepository interface {
	List(limit, offset int, search, status string) ([]domain.ParticipantUser, int64, error)
	ListAllIDs(limit, offset int) ([]string, int64, error)
	GetByID(id string) (*domain.ParticipantUser, error)
	GetByPhone(phone string) (*domain.ParticipantUser, error)
	UpsertByPhone(user *domain.ParticipantUser) error
	Create(user *domain.ParticipantUser) error
	UpdateStatus(id string, status string) error
	Delete(id string) error
	Count(filter map[string]interface{}) (int64, error)
}
