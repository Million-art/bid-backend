package chapapayment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/google/uuid"
)

var ErrFulfillmentDeferred = errors.New("chapa: participation fulfillment deferred")

type VerifiedSnapshot struct {
	TxRef          string
	ChapaReference string
	APIStatus      string
	DataStatus     string
	Amount         interface{}
	Meta           map[string]interface{}
	CustomerEmail  string
	CustomerFirst  string
}

type Service struct {
	repo            ports.PaymentLedgerRepository
	lubSvc          *lub.Service
	users           ports.ParticipantUserRepository
	notificationSvc ports.NotificationService
	log             *slog.Logger
}

func NewService(
	repo ports.PaymentLedgerRepository,
	lubSvc *lub.Service,
	users ports.ParticipantUserRepository,
	notificationSvc ports.NotificationService,
	log *slog.Logger,
) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, lubSvc: lubSvc, users: users, notificationSvc: notificationSvc, log: log}
}

func (s *Service) ProcessVerified(ctx context.Context, snap *VerifiedSnapshot) error {
	txRef := strings.TrimSpace(snap.TxRef)
	if txRef == "" {
		return fmt.Errorf("chapa: empty tx_ref")
	}

	chapaOK := strings.EqualFold(strings.TrimSpace(snap.APIStatus), "success") &&
		strings.EqualFold(strings.TrimSpace(snap.DataStatus), "success")

	auctionID, userID, pricePaid, parseErr := parseParticipationMeta(snap)
	if parseErr != nil {
		existing, lookupErr := s.repo.GetByTxRef(ctx, txRef)
		if lookupErr == nil && existing != nil &&
			existing.UserID != nil && existing.AuctionID != nil {
			auctionID = *existing.AuctionID
			userID = *existing.UserID
			pricePaid = existing.Amount
			parseErr = nil
			s.log.Info("chapa_meta_recovered_from_pending_row", "tx_ref", txRef)
		}
	}
	if parseErr != nil {
		p := &domain.Payment{
			TxRef:     txRef,
			Status:    domain.PaymentStatusFailed,
			Processed: true,
			Amount:    domain.Zero(),
			LastError: domain.FormatErrorDetail(parseErr),
		}
		if err := s.repo.Upsert(ctx, p); err != nil {
			return fmt.Errorf("payment ledger upsert (meta invalid): %w", err)
		}
		s.log.Warn("chapa_payment_meta_invalid", "tx_ref", txRef, "error", parseErr)
		return nil
	}

	if !chapaOK {
		p := &domain.Payment{
			TxRef:     txRef,
			UserID:    ptrUUID(userID),
			AuctionID: ptrUUID(auctionID),
			Amount:    pricePaid,
			Status:    domain.PaymentStatusFailed,
			Processed: true,
			LastError: fmt.Sprintf("chapa verify status not success (api=%q data=%q)", snap.APIStatus, snap.DataStatus),
		}
		if err := s.repo.Upsert(ctx, p); err != nil {
			return fmt.Errorf("payment ledger upsert (chapa not success): %w", err)
		}
		s.log.Warn("chapa_payment_not_success", "tx_ref", txRef, "api_status", snap.APIStatus, "data_status", snap.DataStatus)
		return nil
	}

	p := &domain.Payment{
		TxRef:          txRef,
		ChapaReference: strings.TrimSpace(snap.ChapaReference),
		UserID:         ptrUUID(userID),
		AuctionID:      ptrUUID(auctionID),
		Amount:         pricePaid,
		Status:         domain.PaymentStatusSuccess,
		Processed:      false,
		LastError:      "",
	}
	if err := s.repo.Upsert(ctx, p); err != nil {
		return fmt.Errorf("payment ledger upsert: %w", err)
	}

	if existingPart, err := s.lubSvc.GetParticipation(auctionID, userID.String()); err == nil && existingPart != nil {
		if err := s.repo.MarkProcessed(ctx, txRef); err != nil {
			s.log.Error("chapa_mark_processed_failed", "tx_ref", txRef, "error", err)
		}
		s.log.Info("chapa_fulfillment_idempotent_participation", "tx_ref", txRef)
		return nil
	}

	_, err := s.lubSvc.RecordParticipation(ctx, auctionID, userID.String(), txRef, pricePaid)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			if err := s.repo.MarkProcessed(ctx, txRef); err != nil {
				s.log.Error("chapa_mark_processed_after_conflict_failed", "tx_ref", txRef, "error", err)
			}
			s.log.Info("chapa_fulfillment_idempotent_record", "tx_ref", txRef)
			return nil
		}
		_ = s.repo.SetFulfillmentFailure(ctx, txRef, err)
		if domain.IsRetryableFulfillmentError(err) {
			s.log.Error("chapa_participation_record_failed", "tx_ref", txRef, "auction_id", auctionID.String(), "user_id", userID.String(), "error", err)
			return ErrFulfillmentDeferred
		} else {
			s.log.Warn("chapa_participation_record_failed_no_retry", "tx_ref", txRef, "auction_id", auctionID.String(), "user_id", userID.String(), "error", err)
			if mErr := s.repo.MarkProcessed(ctx, txRef); mErr != nil {
				s.log.Error("chapa_mark_processed_failed_after_non_retryable", "tx_ref", txRef, "error", mErr)
			}
			return nil
		}
	}

	if err := s.notificationSvc.NotifyParticipationConfirmed(ctx, userID.String(), auctionID, "", pricePaid); err != nil {
		s.log.Error("failed to send participation notification", "tx_ref", txRef, "user_id", userID, "error", err)
	}

	if err := s.repo.MarkProcessed(ctx, txRef); err != nil {
		s.log.Error("chapa_mark_processed_failed", "tx_ref", txRef, "error", err)
		return fmt.Errorf("mark processed: %w", err)
	}
	s.log.Info("chapa_fulfillment_complete", "tx_ref", txRef, "auction_id", auctionID.String(), "user_id", userID.String())
	return nil
}

func (s *Service) tryFulfillPayment(ctx context.Context, p *domain.Payment) bool {
	if p.UserID == nil || p.AuctionID == nil {
		s.log.Warn("chapa_retry_skip_incomplete", "tx_ref", p.TxRef)
		return false
	}
	txRef := p.TxRef
	if existingPart, err := s.lubSvc.GetParticipation(*p.AuctionID, p.UserID.String()); err == nil && existingPart != nil {
		if err := s.repo.MarkProcessed(ctx, txRef); err != nil {
			s.log.Error("chapa_retry_mark_processed_failed", "tx_ref", txRef, "error", err)
			return false
		}
		return true
	}
	_, err := s.lubSvc.RecordParticipation(ctx, *p.AuctionID, p.UserID.String(), txRef, p.Amount)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			_ = s.repo.MarkProcessed(ctx, txRef)
			return true
		}
		_ = s.repo.SetFulfillmentFailure(ctx, txRef, err)
		if domain.IsRetryableFulfillmentError(err) {
			s.log.Error("chapa_retry_participation_failed", "tx_ref", txRef, "error", err)
			return false
		} else {
			s.log.Warn("chapa_retry_participation_failed_no_retry", "tx_ref", txRef, "error", err)
			_ = s.repo.MarkProcessed(ctx, txRef)
			return true
		}
	}
	if err := s.repo.MarkProcessed(ctx, txRef); err != nil {
		s.log.Error("chapa_retry_mark_processed_failed", "tx_ref", txRef, "error", err)
		return false
	}
	s.log.Info("chapa_retry_fulfilled", "tx_ref", txRef)
	return true
}

func (s *Service) RetryPaymentByTxRef(ctx context.Context, txRef string) (fulfilled bool, err error) {
	txRef = strings.TrimSpace(txRef)
	if txRef == "" {
		return false, nil
	}
	p, err := s.repo.GetByTxRef(ctx, txRef)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if p.Status != domain.PaymentStatusSuccess || p.Processed {
		return false, nil
	}
	return s.tryFulfillPayment(ctx, p), nil
}

func (s *Service) CreatePendingPayment(ctx context.Context, txRef string, userID, auctionID uuid.UUID, amount domain.Money) error {
	p := &domain.Payment{
		TxRef:     txRef,
		UserID:    ptrUUID(userID),
		AuctionID: ptrUUID(auctionID),
		Amount:    amount,
		Status:    domain.PaymentStatusPending,
		Processed: false,
	}
	if err := s.repo.Upsert(ctx, p); err != nil {
		return fmt.Errorf("chapa: create pending payment: %w", err)
	}
	s.log.Info("chapa_pending_payment_created", "tx_ref", txRef, "user_id", userID, "auction_id", auctionID)
	return nil
}

func (s *Service) RetryUnprocessedPayments(ctx context.Context, limit int) (int, error) {
	rows, err := s.repo.ListSuccessUnprocessed(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for i := range rows {
		if s.tryFulfillPayment(ctx, &rows[i]) {
			done++
		}
	}
	return done, nil
}

func (s *Service) ListSuccessUnprocessed(ctx context.Context, limit int) ([]domain.Payment, error) {
	return s.repo.ListSuccessUnprocessed(ctx, limit)
}

func parseParticipationMeta(snap *VerifiedSnapshot) (auctionID, userID uuid.UUID, price domain.Money, err error) {
	meta := snap.Meta
	if meta == nil {
		return uuid.Nil, uuid.Nil, domain.Zero(), fmt.Errorf("chapa webhook missing meta: %w", domain.ErrInvalidInput)
	}
	as, _ := meta["auction_id"].(string)
	if as == "" {
		as, _ = meta["product_id"].(string)
	}
	us, _ := meta["user_id"].(string)
	auctionID, _ = uuid.Parse(strings.TrimSpace(as))
	userID, _ = uuid.Parse(strings.TrimSpace(us))
	if auctionID == uuid.Nil || userID == uuid.Nil {
		return uuid.Nil, uuid.Nil, domain.Zero(), fmt.Errorf("chapa webhook missing auction_id or user_id: %w", domain.ErrInvalidInput)
	}
	switch a := snap.Amount.(type) {
	case string:
		price, err = domain.NewMoneyFromString(strings.TrimSpace(a))
		if err != nil {
			return uuid.Nil, uuid.Nil, domain.Zero(), err
		}
	case float64:
		price = domain.NewMoney(a)
	default:
		price = domain.Zero()
	}
	return auctionID, userID, price, nil
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
