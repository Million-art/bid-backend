package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
)

// WebSocketBroadcaster defines interface for sending real-time messages
type WebSocketBroadcaster interface {
	BroadcastToUser(userID string, message interface{})
}

const legalNotifyBatchSize = 200

type Service struct {
	notificationRepo    ports.NotificationRepository
	userRepo            ports.UserRepository
	participantUserRepo ports.ParticipantUserRepository
	cacheRepo           ports.CacheRepository
	wsBroadcaster       WebSocketBroadcaster
	emailSvc            ports.EmailService
	smsSvc              ports.SmsService
	log                 *slog.Logger
}

func NewService(
	notificationRepo ports.NotificationRepository,
	userRepo ports.UserRepository,
	participantUserRepo ports.ParticipantUserRepository,
	cacheRepo ports.CacheRepository,
	wsBroadcaster WebSocketBroadcaster,
	emailSvc ports.EmailService,
	smsSvc ports.SmsService,
) *Service {
	return &Service{
		notificationRepo:    notificationRepo,
		userRepo:            userRepo,
		participantUserRepo: participantUserRepo,
		cacheRepo:           cacheRepo,
		wsBroadcaster:       wsBroadcaster,
		emailSvc:            emailSvc,
		smsSvc:              smsSvc,
		log:                 slog.Default(),
	}
}

// BroadcastToUser sends notification via WebSocket for real-time delivery
func (s *Service) BroadcastToUser(
	userID uuid.UUID,
	userType string,
	notification *domain.Notification,
) error {
	if s.wsBroadcaster == nil {
		return nil // WebSocket not configured
	}

	// Send to WebSocket broadcaster
	message := map[string]interface{}{
		"type": "notification",
		"data": notification,
	}

	s.wsBroadcaster.BroadcastToUser(userID.String(), message)
	return nil
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

// MarkWinnerConfirmationAsRead marks a confirmation request as read
func (s *Service) MarkWinnerConfirmationAsRead(confirmationID uuid.UUID) error {
	return s.notificationRepo.MarkByConfirmationID(confirmationID)
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
		ID:       uuid.New(),
		UserID:   uid,
		UserType: domain.UserTypeParticipant,
		Type:     "participation_confirmed",
		Title:    title,
		Message:  message,
		Priority: priority,
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

// NotifyTicketPurchased sends SMS notification when a ticket is purchased.
func (s *Service) NotifyTicketPurchased(ctx context.Context, participantID uuid.UUID, bidID uuid.UUID, ticketNumbers []int, quantity int) error {
	ticketStr := formatTicketNumbers(ticketNumbers)
	if ticketStr == "" {
		ticketStr = fmt.Sprintf("%d ticket(s)", quantity)
	}

	// "Thanks for your purchase! You bought %s. Check your account for details."
	msg := fmt.Sprintf("ስለገዙን እናመሰግናለን! %s ቲኬት ገዝተዋል። ዝርዝሩን ለማየት አካውንትዎን ይግቡ።", ticketStr)

	participant, err := s.participantUserRepo.GetByID(participantID.String())
	if err != nil || participant == nil {
		s.log.Warn("failed to get participant for ticket purchase notification", "participant_id", participantID, "error", err)
		return err
	}

	if participant.Phone == "" {
		s.log.Warn("participant has no phone number for SMS notification", "participant_id", participantID)
		return nil
	}

	return s.smsSvc.SendSms(participant.Phone, msg)
}

// formatTicketNumbers formats a slice of ticket numbers into a readable string.
// Examples: [1,2,3] -> "1-3", [1,5,7,8,9] -> "1, 5-9", [1,3,5] -> "1, 3, 5"
func formatTicketNumbers(numbers []int) string {
	if len(numbers) == 0 {
		return ""
	}

	// Sort the numbers
	sorted := make([]int, len(numbers))
	copy(sorted, numbers)
	sort.Ints(sorted)

	// Build ranges
	var result []string
	start := sorted[0]
	end := sorted[0]

	for i := 1; i < len(sorted); i++ {
		if sorted[i] == end+1 {
			// Consecutive number, extend the range
			end = sorted[i]
		} else {
			// Gap detected, close the current range
			if start == end {
				result = append(result, fmt.Sprintf("%d", start))
			} else {
				result = append(result, fmt.Sprintf("%d-%d", start, end))
			}
			start = sorted[i]
			end = sorted[i]
		}
	}

	// Close the final range
	if start == end {
		result = append(result, fmt.Sprintf("%d", start))
	} else {
		result = append(result, fmt.Sprintf("%d-%d", start, end))
	}

	return strings.Join(result, ", ")
}
