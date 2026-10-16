package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification represents an in-app notification
type Notification struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"userId"`
	UserType    string    `gorm:"type:varchar(20);not null" json:"userType"` // participant, agent, admin
	Type        string    `gorm:"type:varchar(50);not null" json:"type"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Message     string    `gorm:"type:text;not null" json:"message"`
	ActionURL   string    `gorm:"type:text" json:"actionUrl,omitempty"`
	ActionLabel string    `gorm:"type:varchar(50)" json:"actionLabel,omitempty"`

	// Related entities
	BidID          *uuid.UUID `gorm:"type:uuid;column:bid_id" json:"bidId,omitempty"`
	ConfirmationID *uuid.UUID `gorm:"type:uuid" json:"confirmationId,omitempty"`
	PayoutID       *uuid.UUID `gorm:"type:uuid" json:"payoutId,omitempty"`

	// Status
	IsRead bool       `gorm:"default:false" json:"isRead"`
	ReadAt *time.Time `json:"readAt,omitempty"`

	// Metadata
	Priority string `gorm:"type:varchar(20);default:'normal'" json:"priority"` // low, normal, high, urgent
	Metadata string `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	ExpiresAt *time.Time     `json:"expiresAt,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// NotificationPreferences stores user notification preferences
type NotificationPreferences struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID   uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
	UserType string    `gorm:"type:varchar(20);not null" json:"userType"`

	// Channel preferences
	EmailEnabled bool `gorm:"default:true" json:"emailEnabled"`
	InAppEnabled bool `gorm:"default:true" json:"inAppEnabled"`
	PushEnabled  bool `gorm:"default:false" json:"pushEnabled"`

	// Event preferences
	WinnerConfirmation bool `gorm:"default:true" json:"winnerConfirmation"`
	PayoutUpdates      bool `gorm:"default:true" json:"payoutUpdates"`
	BidUpdates         bool `gorm:"default:true;column:bid_updates" json:"bidUpdates"`
	Marketing          bool `gorm:"default:false" json:"marketing"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Notification types
const (
	NotificationTypeWinnerConfirmationRequest = "winner_confirmation_request"
	NotificationTypePayoutReady               = "payout_ready"
	NotificationTypePayoutRequested           = "payout_requested"
	NotificationTypePayoutCompleted           = "payout_completed"
	NotificationTypePayoutRejected            = "payout_rejected"
	NotificationTypePrizeNotDelivered         = "prize_not_delivered"
	NotificationTypeWinnerCompensated         = "winner_compensated"
	NotificationTypebidCompleted              = "bid_completed"
	NotificationTypebidDrawn                  = "bid_drawn"
	NotificationTypebidDrawPostponed          = "bid_draw_postponed"
	NotificationTypebidDrawReminder           = "bid_draw_reminder"
	NotificationTypebidCancelled              = "bid_cancelled"
	NotificationTypebidAutoRejected           = "bid_auto_rejected"
	NotificationTypeConfirmationReminder      = "confirmation_reminder"

	// bid approval workflow
	NotificationTypebidSubmitted     = "bid_submitted"
	NotificationTypebidApproved      = "bid_approved"
	NotificationTypebidRejected      = "bid_rejected"
	NotificationTypebidNeedsChanges  = "bid_needs_changes"
	NotificationTypebidStatusChanged = "bid_status_changed"

	// KYC workflow
	NotificationTypeKYCNeedsReview = "kyc_needs_review"
	NotificationTypeKYCApproved    = "kyc_approved"
	NotificationTypeKYCRejected    = "kyc_rejected"

	NotificationTypeLegalDocumentUpdated = "legal_document_updated"

	// H-5: Enhanced features
	NotificationTypePriceIncreased          = "price_increased"
	NotificationTypeRefundChoice            = "refund_choice"
	NotificationTypeRefundCompleted         = "refund_completed"
	NotificationTypeReferralBountyReceived  = "referral_bounty_received"
	NotificationTypeReferralBountyFinalized = "referral_bounty_finalized"
)

// User types
const (
	UserTypeParticipant = "participant"
	UserTypeAgent       = "agent"
	UserTypeAdmin       = "admin"
)

// Notification priorities
const (
	NotificationPriorityLow    = "low"
	NotificationPriorityNormal = "normal"
	NotificationPriorityHigh   = "high"
	NotificationPriorityUrgent = "urgent"
)
