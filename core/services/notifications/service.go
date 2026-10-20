package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
)

const (
	legalNotifyBatchSize      = 200
	auctionParticipantPageSize = 200
)

type Service struct {
	notificationRepo    ports.NotificationRepository
	userRepo            ports.UserRepository
	participantUserRepo ports.ParticipantUserRepository
	participationRepo   ports.AuctionParticipationRepository
	cacheRepo           ports.CacheRepository
	emailSvc            ports.EmailService
	smsSvc              ports.SmsService
	log                 *slog.Logger
}

func NewService(
	notificationRepo ports.NotificationRepository,
	userRepo ports.UserRepository,
	participantUserRepo ports.ParticipantUserRepository,
	participationRepo ports.AuctionParticipationRepository,
	cacheRepo ports.CacheRepository,
	emailSvc ports.EmailService,
	smsSvc ports.SmsService,
) *Service {
	return &Service{
		notificationRepo:    notificationRepo,
		userRepo:            userRepo,
		participantUserRepo: participantUserRepo,
		participationRepo:   participationRepo,
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

func (s *Service) createAndInvalidate(ctx context.Context, n *domain.Notification) error {
	if err := s.notificationRepo.Create(n); err != nil {
		return err
	}
	if s.cacheRepo != nil {
		_ = s.cacheRepo.InvalidateUnreadCount(ctx, n.UserID)
	}
	return nil
}

func (s *Service) notifyParticipant(
	ctx context.Context,
	participantID string,
	auctionID uuid.UUID,
	notifType, title, message, priority string,
) error {
	uid, err := uuid.Parse(participantID)
	if err != nil {
		return nil
	}
	aid := auctionID
	n := &domain.Notification{
		ID:          uuid.New(),
		UserID:      uid,
		UserType:    domain.UserTypeParticipant,
		Type:        notifType,
		Title:       title,
		Message:     message,
		ActionURL:   auctionActionURL(auctionID),
		ActionLabel: "View auction",
		AuctionID:   &aid,
		Priority:    priority,
	}
	return s.createAndInvalidate(ctx, n)
}

func (s *Service) notifyAuctionParticipants(
	ctx context.Context,
	auctionID uuid.UUID,
	notifType, title, message, priority string,
) error {
	if s.participationRepo == nil {
		return nil
	}
	offset := 0
	for {
		parts, total, err := s.participationRepo.ListByAuction(auctionID, auctionParticipantPageSize, offset)
		if err != nil {
			return fmt.Errorf("list auction participants: %w", err)
		}
		if len(parts) == 0 {
			break
		}
		batch := make([]*domain.Notification, 0, len(parts))
		for _, p := range parts {
			if p.Status != "" && p.Status != domain.ParticipationStatusActive {
				continue
			}
			uid, err := uuid.Parse(p.ParticipantID)
			if err != nil {
				continue
			}
			aid := auctionID
			batch = append(batch, &domain.Notification{
				ID:          uuid.New(),
				UserID:      uid,
				UserType:    domain.UserTypeParticipant,
				Type:        notifType,
				Title:       title,
				Message:     message,
				ActionURL:   auctionActionURL(auctionID),
				ActionLabel: "View auction",
				AuctionID:   &aid,
				Priority:    priority,
			})
		}
		if err := s.flushBatch(ctx, batch); err != nil {
			return err
		}
		offset += len(parts)
		if int64(offset) >= total {
			break
		}
	}
	return nil
}

func (s *Service) flushBatch(ctx context.Context, batch []*domain.Notification) error {
	if len(batch) == 0 {
		return nil
	}
	if err := s.notificationRepo.CreateBatch(batch); err != nil {
		return err
	}
	for _, n := range batch {
		if s.cacheRepo != nil {
			_ = s.cacheRepo.InvalidateUnreadCount(ctx, n.UserID)
		}
	}
	return nil
}

func (s *Service) NotifyAuctionCreated(ctx context.Context, productID uuid.UUID, productName string) error {
	// Draft auctions are staff-only; no participant fan-out.
	return nil
}

func (s *Service) NotifyAuctionActivated(ctx context.Context, productID uuid.UUID, productName string) error {
	if s.participantUserRepo == nil {
		return nil
	}
	title, message, priority := auctionActivatedCopy(displayName(productName))
	off := 0
	for {
		ids, total, err := s.participantUserRepo.ListAllIDs(legalNotifyBatchSize, off)
		if err != nil {
			return fmt.Errorf("list participants for auction activated: %w", err)
		}
		if len(ids) == 0 {
			break
		}
		batch := make([]*domain.Notification, 0, len(ids))
		aid := productID
		for _, idStr := range ids {
			pid, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			batch = append(batch, &domain.Notification{
				ID:          uuid.New(),
				UserID:      pid,
				UserType:    domain.UserTypeParticipant,
				Type:        domain.NotificationTypeAuctionActivated,
				Title:       title,
				Message:     message,
				ActionURL:   auctionActionURL(productID),
				ActionLabel: "View auction",
				AuctionID:   &aid,
				Priority:    priority,
			})
		}
		if err := s.flushBatch(ctx, batch); err != nil {
			return err
		}
		off += len(ids)
		if int64(off) >= total {
			break
		}
	}
	return nil
}

func (s *Service) NotifyAuctionPostponed(ctx context.Context, productID uuid.UUID, productName string, newCloseAt string) error {
	title, message, priority := auctionPostponedCopy(displayName(productName), newCloseAt)
	return s.notifyAuctionParticipants(ctx, productID, domain.NotificationTypeAuctionPostponed, title, message, priority)
}

func (s *Service) NotifyAuctionClosed(ctx context.Context, productID uuid.UUID, productName string) error {
	title, message, priority := auctionClosedCopy(displayName(productName))
	return s.notifyAuctionParticipants(ctx, productID, domain.NotificationTypeAuctionClosed, title, message, priority)
}

func (s *Service) NotifyAuctionCancelled(ctx context.Context, productID uuid.UUID, productName string, reason string) error {
	title, message, priority := auctionCancelledCopy(displayName(productName), reason)
	return s.notifyAuctionParticipants(ctx, productID, domain.NotificationTypeAuctionCancelled, title, message, priority)
}

func (s *Service) NotifyParticipationConfirmed(ctx context.Context, participantID string, productID uuid.UUID, productName string, feePaid domain.Money) error {
	title, message, priority := participationNotificationCopy(displayName(productName), feePaid)
	return s.notifyParticipant(ctx, participantID, productID, domain.NotificationTypeParticipationConfirmed, title, message, priority)
}

func (s *Service) NotifyBidPlaced(ctx context.Context, participantID string, productID uuid.UUID, productName string, bidValue domain.Money) error {
	title, message, priority := bidPlacedCopy(displayName(productName), bidValue)
	return s.notifyParticipant(ctx, participantID, productID, domain.NotificationTypeBidPlaced, title, message, priority)
}

func (s *Service) NotifyBidUpdated(ctx context.Context, participantID string, productID uuid.UUID, productName string, oldValue, newValue domain.Money) error {
	title, message, priority := bidUpdatedCopy(displayName(productName), oldValue, newValue)
	return s.notifyParticipant(ctx, participantID, productID, domain.NotificationTypeBidUpdated, title, message, priority)
}

func (s *Service) NotifyAuctionWinner(ctx context.Context, winnerID string, productID uuid.UUID, productName string, winningBid domain.Money) error {
	title, message, priority := winnerCopy(displayName(productName), winningBid)
	return s.notifyParticipant(ctx, winnerID, productID, domain.NotificationTypeAuctionWinner, title, message, priority)
}

func (s *Service) NotifyAuctionNoWinner(ctx context.Context, productID uuid.UUID, productName string) error {
	title, message, priority := noWinnerCopy(displayName(productName))
	return s.notifyAuctionParticipants(ctx, productID, domain.NotificationTypeAuctionNoWinner, title, message, priority)
}

func (s *Service) NotifyPaymentSuccess(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error {
	uid, err := uuid.Parse(participantID)
	if err != nil {
		return nil
	}
	title, message, priority := paymentSuccessCopy(amount, txRef)
	aid := productID
	n := &domain.Notification{
		ID:          uuid.New(),
		UserID:      uid,
		UserType:    domain.UserTypeParticipant,
		Type:        domain.NotificationTypePaymentSuccess,
		Title:       title,
		Message:     message,
		ActionURL:   auctionActionURL(productID),
		ActionLabel: "View auction",
		AuctionID:   &aid,
		Priority:    priority,
	}
	return s.createAndInvalidate(ctx, n)
}

func (s *Service) NotifyPaymentFailed(ctx context.Context, participantID string, productID uuid.UUID, txRef string, reason string) error {
	uid, err := uuid.Parse(participantID)
	if err != nil {
		return nil
	}
	title, message, priority := paymentFailedCopy(txRef, reason)
	aid := productID
	n := &domain.Notification{
		ID:        uuid.New(),
		UserID:    uid,
		UserType:  domain.UserTypeParticipant,
		Type:      domain.NotificationTypePaymentFailed,
		Title:     title,
		Message:   message,
		AuctionID: &aid,
		Priority:  priority,
	}
	return s.createAndInvalidate(ctx, n)
}

func (s *Service) NotifyRefundProcessed(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error {
	_ = txRef
	title, message, priority := refundCopy("this auction", amount)
	return s.notifyParticipant(ctx, participantID, productID, domain.NotificationTypeRefundCompleted, title, message, priority)
}

func displayName(productName string) string {
	if productName == "" {
		return "an auction"
	}
	return productName
}
