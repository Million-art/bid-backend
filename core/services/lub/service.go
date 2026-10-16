package lub

import (
	"github.com/bid-hub/admin-backend/core/ports"
)

// Service is the Lowest Unique Bid (LUB) auction business-logic layer.
type Service struct {
	auctionRepo       ports.AuctionRepository
	participationRepo ports.AuctionParticipationRepository
	bidRepo           ports.BidRepository
	audit             ports.AuditLogService
	notificationSvc   LUBNotificationProvider
}

type LUBNotificationProvider interface {
}

func NewService(
	auctionRepo ports.AuctionRepository,
	participationRepo ports.AuctionParticipationRepository,
	bidRepo ports.BidRepository,
	audit ports.AuditLogService,
	notificationSvc LUBNotificationProvider,
) *Service {
	return &Service{
		auctionRepo:       auctionRepo,
		participationRepo: participationRepo,
		bidRepo:           bidRepo,
		audit:             audit,
		notificationSvc:   notificationSvc,
	}
}
