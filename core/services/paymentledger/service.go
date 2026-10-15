package paymentledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/google/uuid"
)

type Service struct {
	ledger     ports.PaymentLedgerRepository
	refunds    ports.RefundRecordRepository
	chapa      ports.PaymentProvider
	lubSvc     *lub.Service
	audit      *services.AuditLogService
}

func NewService(
	ledger ports.PaymentLedgerRepository,
	refunds ports.RefundRecordRepository,
	chapa ports.PaymentProvider,
	lubSvc *lub.Service,
	audit *services.AuditLogService,
) *Service {
	return &Service{
		ledger:  ledger,
		refunds: refunds,
		chapa:   chapa,
		lubSvc:  lubSvc,
		audit:   audit,
	}
}

func (s *Service) List(ctx context.Context, f ports.PaymentListFilter) ([]domain.PaymentStaffView, int64, error) {
	return s.ledger.ListStaff(ctx, f)
}

func (s *Service) Refund(ctx context.Context, txRef string, actorID uuid.UUID, reason string) error {
	txRef = strings.TrimSpace(txRef)
	if txRef == "" {
		return domain.ErrInvalidInput
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Staff refund  participation fulfillment failed"
	}

	p, err := s.ledger.GetByTxRef(ctx, txRef)
	if err != nil {
		return err
	}
	if p.Status == domain.PaymentStatusRefunded {
		return domain.ErrConflict
	}
	if p.Processed {
		return domain.NewAppError(422, "payment is already fulfilled; refund is only for unprocessed rows", nil)
	}
	if p.Status != domain.PaymentStatusSuccess && p.Status != domain.PaymentStatusFailed {
		return domain.NewAppError(422, "only successful or failed Chapa payments can be refunded", nil)
	}
	if p.UserID == nil || p.ProductID == nil {
		return domain.NewAppError(422, "payment row is missing user or product metadata", nil)
	}

	snap, err := verifyChapaPayment(s.chapa, txRef)
	if err != nil {
		return err
	}
	if snap.Reference != "" {
		_ = s.ledger.SetChapaReference(ctx, txRef, snap.Reference)
	}

	if existing, rErr := s.refunds.GetByTxRef(txRef); rErr == nil && existing != nil {
		switch existing.Status {
		case domain.RefundStatusPending, domain.RefundStatusProcessing, domain.RefundStatusSuccess:
			return domain.NewAppError(409, fmt.Sprintf("refund already in progress or completed (status=%s)", existing.Status), nil)
		}
	}

	part, _ := s.lubSvc.GetParticipation(*p.ProductID, p.UserID.String())
	if part != nil && part.Status == domain.LUBParticipationStatusActive {
		return s.initiateParticipationRefund(ctx, p, part, actorID, reason, snap)
	}

	return s.refundPaymentOnly(ctx, p, actorID, reason, snap)
}

func (s *Service) initiateParticipationRefund(ctx context.Context, p *domain.Payment, part *domain.LUBParticipation, actorID uuid.UUID, reason string, snap *ports.ChapaTransactionSnapshot) error {
	record := &domain.RefundRecord{
		ID:            uuid.New(),
		ProductID:     part.ProductID,
		ParticipationID: &part.ID,
		ParticipantID: part.ParticipantID,
		Amount:        p.Amount,
		TxRef:         p.TxRef,
		Status:        domain.RefundStatusPending,
		ProcessedBy:   &actorID,
	}
	if err := s.refunds.Create(record); err != nil {
		return err
	}

	refPath := ports.PickChapaRefundPathRef(snap, p.TxRef)
	if refPath == strings.TrimSpace(p.TxRef) && strings.TrimSpace(p.ChapaReference) != "" {
		refPath = strings.TrimSpace(p.ChapaReference)
	}
	chapaRef, err := s.chapa.InitiateRefund(refPath, p.Amount.Float64(), reason, record.ID.String())
	if err != nil {
		_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusFailed, "", err.Error())
		return fmt.Errorf("chapa refund call failed: %w", err)
	}

	_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusProcessing, chapaRef, "")

	chapaStatus := ""
	if st, verr := s.chapa.VerifyRefund(chapaRef); verr != nil {
		_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusProcessing, chapaRef,
			"refund accepted; verify pending: "+verr.Error())
	} else {
		chapaStatus = st
	}

	recordStatus, markPayment := chapaRefundOutcome(chapaStatus)
	if chapaStatus != "" {
		_ = s.refunds.UpdateStatus(record.ID, recordStatus, chapaRef, "")
	}

	if recordStatus == domain.RefundStatusReversed {
		return domain.NewAppError(422, "Chapa reversed the refund (funds returned to your Chapa balance)", nil)
	}

	if markPayment {
		if err := s.ledger.MarkRefunded(ctx, p.TxRef); err != nil {
			return err
		}
		if err := s.lubSvc.RefundParticipation(ctx, part.ID); err != nil {
			return fmt.Errorf("failed to mark participation refunded: %w", err)
		}
	}

	if s.audit != nil {
		action := "payment.ledger.refund_processing"
		if markPayment {
			action = "payment.ledger.refunded"
		}
		s.audit.Log(&domain.AuditLog{
			UserID:   actorID,
			Action:   action,
			Entity:   "payment",
			EntityID: p.TxRef,
			Metadata: map[string]interface{}{
				"tx_ref":           p.TxRef,
				"product_id":       p.ProductID.String(),
				"participant_id":   p.UserID.String(),
				"participation_id": part.ID.String(),
				"amount":           p.Amount.String(),
				"chapa_refund_ref": chapaRef,
				"chapa_status":     chapaStatus,
				"refund_record_id": record.ID.String(),
				"refund_status":    recordStatus,
			},
		}, nil)
	}
	return nil
}

func (s *Service) refundPaymentOnly(ctx context.Context, p *domain.Payment, actorID uuid.UUID, reason string, snap *ports.ChapaTransactionSnapshot) error {
	record := &domain.RefundRecord{
		ID:            uuid.New(),
		ProductID:     *p.ProductID,
		ParticipantID: p.UserID.String(),
		Amount:        p.Amount,
		TxRef:         p.TxRef,
		Status:        domain.RefundStatusPending,
		ProcessedBy:   &actorID,
	}
	if err := s.refunds.Create(record); err != nil {
		return err
	}

	refPath := ports.PickChapaRefundPathRef(snap, p.TxRef)
	if refPath == strings.TrimSpace(p.TxRef) && strings.TrimSpace(p.ChapaReference) != "" {
		refPath = strings.TrimSpace(p.ChapaReference)
	}
	chapaRef, err := s.chapa.InitiateRefund(refPath, p.Amount.Float64(), reason, record.ID.String())
	if err != nil {
		_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusFailed, "", err.Error())
		return fmt.Errorf("chapa refund call failed: %w", err)
	}

	_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusProcessing, chapaRef, "")

	chapaStatus := ""
	if st, verr := s.chapa.VerifyRefund(chapaRef); verr != nil {
		_ = s.refunds.UpdateStatus(record.ID, domain.RefundStatusProcessing, chapaRef,
			"refund accepted; verify pending: "+verr.Error())
	} else {
		chapaStatus = st
	}

	recordStatus, markPayment := chapaRefundOutcome(chapaStatus)
	if chapaStatus != "" {
		_ = s.refunds.UpdateStatus(record.ID, recordStatus, chapaRef, "")
	}

	if recordStatus == domain.RefundStatusReversed {
		return domain.NewAppError(422, "Chapa reversed the refund (funds returned to your Chapa balance)", nil)
	}

	if markPayment {
		if err := s.ledger.MarkRefunded(ctx, p.TxRef); err != nil {
			return err
		}
	}

	if s.audit != nil {
		action := "payment.ledger.refund_processing"
		if markPayment {
			action = "payment.ledger.refunded"
		}
		s.audit.Log(&domain.AuditLog{
			UserID:   actorID,
			Action:   action,
			Entity:   "payment",
			EntityID: p.TxRef,
			Metadata: map[string]interface{}{
				"tx_ref":         p.TxRef,
				"product_id":     p.ProductID.String(),
				"participant_id": p.UserID.String(),
				"amount":         p.Amount.String(),
				"chapa_refund_ref": chapaRef,
				"chapa_status":     chapaStatus,
				"refund_record_id": record.ID.String(),
				"refund_status":    recordStatus,
			},
		}, nil)
	}
	return nil
}

func RefundEligible(p *domain.PaymentStaffView) bool {
	if p == nil {
		return false
	}
	if p.Status == domain.PaymentStatusRefunded {
		return false
	}
	if p.Processed {
		return false
	}
	if p.Status != domain.PaymentStatusSuccess {
		return false
	}
	switch p.RefundStatus {
	case domain.RefundStatusPending, domain.RefundStatusProcessing, domain.RefundStatusSuccess:
		return false
	}
	return true
}