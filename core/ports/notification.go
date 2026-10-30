package ports

import (
	"context"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

type NotificationRepository interface {
	Create(notification *domain.Notification) error
	CreateBatch(items []*domain.Notification) error
	GetByID(id uuid.UUID) (*domain.Notification, error)
	GetByUserID(userID uuid.UUID, userType string, limit, offset int) ([]domain.Notification, int64, error)
	GetUnreadByUserID(userID uuid.UUID, userType string) ([]domain.Notification, error)
	GetUnreadCount(userID uuid.UUID, userType string) (int64, error)
	MarkAsRead(id uuid.UUID) error
	MarkAllAsRead(userID uuid.UUID, userType string) error
	Delete(id uuid.UUID) error
	DeleteExpired() (int64, error)
	DeleteOldRead(daysOld int) (int64, error)
}

type NotificationService interface {
	NotifyAuctionCreated(ctx context.Context, productID uuid.UUID, productName string) error
	NotifyAuctionActivated(ctx context.Context, productID uuid.UUID, productName string) error
	NotifyAuctionPostponed(ctx context.Context, productID uuid.UUID, productName string, newCloseAt string) error
	NotifyAuctionClosed(ctx context.Context, productID uuid.UUID, productName string) error
	NotifyAuctionCancelled(ctx context.Context, productID uuid.UUID, productName string, reason string) error
	NotifyParticipationConfirmed(ctx context.Context, participantID string, productID uuid.UUID, productName string, feePaid domain.Money, txRef string) error
	NotifyBidPlaced(ctx context.Context, participantID string, productID uuid.UUID, productName string, bidValue domain.Money) error
	NotifyBidUpdated(ctx context.Context, participantID string, productID uuid.UUID, productName string, oldValue, newValue domain.Money) error
	NotifyAuctionWinner(ctx context.Context, winnerID string, productID uuid.UUID, productName string, winningBid domain.Money) error
	NotifyAuctionNoWinner(ctx context.Context, productID uuid.UUID, productName string) error
	NotifyPaymentSuccess(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error
	NotifyPaymentFailed(ctx context.Context, participantID string, productID uuid.UUID, txRef string, reason string) error
	NotifyRefundProcessed(ctx context.Context, participantID string, productID uuid.UUID, amount domain.Money, txRef string) error
	NotifyLegalDocumentPublished(ctx context.Context, docType string) error

	GetUserNotifications(userID uuid.UUID, userType string, limit, offset int) ([]domain.Notification, int64, error)
	GetUnreadNotifications(userID uuid.UUID, userType string) ([]domain.Notification, error)
	GetUnreadCount(ctx context.Context, userID uuid.UUID, userType string) (int64, error)

	MarkAsRead(ctx context.Context, notificationID uuid.UUID) error
	MarkAllAsRead(ctx context.Context, userID uuid.UUID, userType string) error
	DeleteNotification(notificationID uuid.UUID) error
}