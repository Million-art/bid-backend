package lub

import (
	"github.com/bid-hub/admin-backend/core/ports"
)

// Service is the LUB (Lowest Unique Bid) business-logic layer.
// It is dependency-injected with repository interfaces and optional
// infrastructure adapters (audit, notifications, email, SMS).
type Service struct {
	productRepo       ports.LUBProductRepository
	participationRepo ports.LUBParticipationRepository
	bidEntryRepo      ports.LUBBidEntryRepository
	audit             ports.AuditLogService
	notificationSvc   LUBNotificationProvider // optional
}

// LUBNotificationProvider is a small interface for the notifications we need.
// It is satisfied by the existing notifications.Service — only the methods
// used here are listed to avoid tight coupling.
type LUBNotificationProvider interface {
	// Implementations are free to no-op any method.
}

// NewService creates a new LUB service with required and optional dependencies.
func NewService(
	productRepo ports.LUBProductRepository,
	participationRepo ports.LUBParticipationRepository,
	bidEntryRepo ports.LUBBidEntryRepository,
	audit ports.AuditLogService,
	notificationSvc LUBNotificationProvider,
) *Service {
	return &Service{
		productRepo:       productRepo,
		participationRepo: participationRepo,
		bidEntryRepo:      bidEntryRepo,
		audit:             audit,
		notificationSvc:   notificationSvc,
	}
}
