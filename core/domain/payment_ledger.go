package domain

import (
	"time"

	"github.com/google/uuid"
)

func (Payment) TableName() string {
	return "payments"
}

type Payment struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TxRef          string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"txRef"`
	ChapaReference string     `gorm:"type:varchar(255)" json:"chapaReference,omitempty"`
	UserID         *uuid.UUID `gorm:"type:uuid;index" json:"userId,omitempty"`
	ProductID      *uuid.UUID `gorm:"type:uuid;index" json:"productId,omitempty"`
	ParticipationID *uuid.UUID `gorm:"type:uuid;index" json:"participationId,omitempty"`
	Amount         Money      `gorm:"type:decimal(12,2);not null" json:"amount"`
	Status         string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Processed      bool       `gorm:"not null;default:false" json:"processed"`
	RetryDisabled  bool       `gorm:"not null;default:false" json:"retryDisabled"`
	ReferredBy     *uuid.UUID `gorm:"type:uuid" json:"referredBy,omitempty"`
	LastError      string     `gorm:"type:text" json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

const (
	PaymentStatusPending  = "pending"
	PaymentStatusSuccess  = "success"
	PaymentStatusFailed   = "failed"
	PaymentStatusRefunded = "refunded"
)

type PaymentStaffView struct {
	Payment
	ParticipantName  string `json:"participantName,omitempty"`
	ParticipantEmail string `json:"participantEmail,omitempty"`
	ProductName      string `json:"productName,omitempty"`
	HasParticipation bool   `json:"hasParticipation"`
	RefundStatus     string `json:"refundStatus,omitempty"`
}

type WalletTransaction struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletType       string    `gorm:"type:varchar(20);not null;index" json:"walletType"` // participant, platform
	WalletID         string    `gorm:"type:varchar(36);not null;index" json:"walletId"`
	TransactionType  string    `gorm:"type:varchar(50);not null" json:"transactionType"`
	Amount           Money     `gorm:"type:decimal(12,2);not null" json:"amount"`
	ReferenceID      *uuid.UUID `gorm:"type:uuid" json:"referenceId,omitempty"`
	Description      string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type MonthlyCount struct {
	Month string `json:"month"`
	Count int64  `json:"count"`
}

type MonthlyAmount struct {
	Month  string `json:"month"`
	Amount Money  `json:"amount"`
}

const (
	RefundStatusPending    = "pending"
	RefundStatusProcessing = "processing"
	RefundStatusSuccess    = "success"
	RefundStatusFailed     = "failed"
	RefundStatusReversed   = "reversed"
)

type RefundRecord struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProductID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"productId"`
	ParticipationID *uuid.UUID `gorm:"type:uuid;index" json:"participationId,omitempty"`
	ParticipantID   string     `gorm:"type:varchar(36);not null;index" json:"participantId"`
	Amount          Money      `gorm:"type:decimal(12,2);not null" json:"amount"`
	TxRef           string     `gorm:"type:varchar(255);not null;index" json:"txRef"`
	ChapaRefundRef  string     `gorm:"type:varchar(255)" json:"chapaRefundRef,omitempty"`
	Status          string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	ErrorMessage    string     `gorm:"type:text" json:"errorMessage,omitempty"`
	ProcessedBy     *uuid.UUID `gorm:"type:uuid" json:"processedBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (RefundRecord) TableName() string {
	return "refund_records"
}