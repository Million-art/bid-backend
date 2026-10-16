package notifications

import (
	"context"
	"log/slog"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
)

const legalNotifyBatchSize = 200

type Service struct {
	notificationRepo    ports.NotificationRepository
	userRepo            ports.UserRepository
	participantUserRepo ports.ParticipantUserRepository
	cacheRepo           ports.CacheRepository
	emailSvc            ports.EmailService
	smsSvc              ports.SmsService
	log                 *slog.Logger
}

func NewService(
	notificationRepo ports.NotificationRepository,
	userRepo ports.UserRepository,
	participantUserRepo ports.ParticipantUserRepository,
	cacheRepo ports.CacheRepository,
	emailSvc ports.EmailService,
	smsSvc ports.SmsService,
) *Service {
	return &Service{
		notificationRepo:    notificationRepo,
		userRepo:            userRepo,
		participantUserRepo: participantUserRepo,
		cacheRepo:           cacheRepo,
		emailSvc:            emailSvc,
		smsSvc:              smsSvc,
		log:                 slog.Default(),
	}
}

// GetUserNotifications retrieves paginated notifications for a user
func (s *Service) GetUserNotifications(
	userID uuid.UUID,
	userType string,
	limit, offset int,
) ([]domain.Notification, int64, error) {
	return s.notificationRepo.GetByUserID(userID, userType, limit, offset)
}

// GetUnreadNotifications retrieves unread notifications for a user
func (s *Service) GetUnreadNotifications(
	userID uuid.UUID,
	userType string,
) ([]domain.Notification, error) {
	return s.notificationRepo.GetUnreadByUserID(userID, userType)
}

// GetUnreadCount returns count of unread notifications with caching support
func (s *Service) GetUnreadCount(
	ctx context.Context,
	userID uuid.UUID,
	userType string,
) (int64, error) {
	if s.cacheRepo != nil {
		count, found, err := s.cacheRepo.GetUnreadCount(ctx, userID)
		if err == nil && found {
			return count, nil
		}
	}

	count, err := s.notificationRepo.GetUnreadCount(userID, userType)
	if err != nil {
		return 0, err
	}

	if s.cacheRepo != nil {
		_ = s.cacheRepo.SetUnreadCount(ctx, userID, count)
	}

	return count, nil
}

// MarkAsRead marks a notification as read and invalidates cache
func (s *Service) MarkAsRead(ctx context.Context, notificationID uuid.UUID) error {
	notification, err := s.notificationRepo.GetByID(notificationID)
	if err == nil && notification != nil && s.cacheRepo != nil {
		_ = s.cacheRepo.InvalidateUnreadCount(ctx, notification.UserID)
	}
	return s.notificationRepo.MarkAsRead(notificationID)
}

// MarkAllAsRead marks all notifications as read for a user and invalidates cache
func (s *Service) MarkAllAsRead(ctx context.Context, userID uuid.UUID, userType string) error {
	if s.cacheRepo != nil {
		_ = s.cacheRepo.InvalidateUnreadCount(ctx, userID)
	}
	return s.notificationRepo.MarkAllAsRead(userID, userType)
}

// DeleteNotification deletes a notification
func (s *Service) DeleteNotification(notificationID uuid.UUID) error {
	return s.notificationRepo.Delete(notificationID)
}

func (s *Service) NotifyAuctionCreated(ctx context.Context, productID uuid.UUID, productName string) error {
	return nil
}

func (s *Service) NotifyAuctionActivated(ctx context.Context, productID uuid.UUID, productName string) error {
	return nil
}

func (s *Service) NotifyAuctionPostponed(ctx context.Context, productID uuid.UUID, productName string, newCloseAt string) error {
	return nil
}

func (s *Service) NotifyAuctionClosed(ctx context.Context, productID uuid.UUID, productName string) error {
	return nil
}

func (s *Service) NotifyAuctionCancelled(ctx context.Context, productID uuid.UUID, productName string, reason string) error {
	return nil
}

func (s *Service) NotifyParticipationConfirmed(ctx context.Context, participantID string, productID uuid.UUID, productName string, feePaid domain.Money) error {
	uid, err := uuid.Parse(participantID)
	if err != nil {
		return nil
	}
	title, message, priority := participationNotificationCopy(productName, feePaid)
	n := &domain.Notification{
		ID:        uuid.New(),
		UserID:    uid,
		UserType:  domain.UserTypeParticipant,
		Type:      domain.NotificationTypeParticipationConfirmed,
		Title:     title,
		Message:   message,
		AuctionID: &productID,
		Priority:  priority,
	}
	return s.notificationRepo.Create(n)
}

func (s *Service) NotifyBidPlaced(ctx context.Context, participantID string, productID uuid.UUID, productName string, bidValue domain.Money) error {
	return nil
}

func (s *Service) NotifyBidUpdated(ctx context.Context, participantID string, productID uuid.UUID, productName string, oldValue, newValue domain.Money) error {
	return nil
}

func (s *Service) NotifyAuctionWinner(ctx context.Context, winnerID string, productID uuid.UUID, productName string, winningBid domain.Money) error {
	return nil
}

func (s *Service) NotifyAuctionNoWinner(ctx context.Context, productID uuid.UUID, productName string) error {
	return nil
}

func (s *Service) NotifyPaymentSuccess(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error {
	return nil
}

func (s *Service) NotifyPaymentFailed(ctx context.Context, participantID string, productID uuid.UUID, txRef string, reason string) error {
	return nil
}

func (s *Service) NotifyRefundProcessed(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error {
	return nil
}
