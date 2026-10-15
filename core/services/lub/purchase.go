package lub

import (
	"context"
	"fmt"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// ---------------------- Product management (admin) ----------------------

// CreateProduct creates a new LUB product in draft status.
func (s *Service) CreateProduct(ctx context.Context, p *domain.Product, createdBy uuid.UUID) error {
	p.Status = domain.ProductStatusDraft
	p.CreatedBy = createdBy
	p.PostponeCount = 0
	if p.MinBidValue.IsZero() {
		p.MinBidValue = domain.NewMoney(0.01)
	}
	if err := s.productRepo.Create(p); err != nil {
		return err
	}
	s.logAudit(createdBy, "lub.product.created", "lub_product", p.ID.String(), p.Name)
	return nil
}

// UpdateProduct updates mutable fields of a product (name, description, fees, dates).
// Only draft products can be fully edited; active products can only have CloseAt extended (postponed).
func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, updates *domain.Product, actorID uuid.UUID) error {
	p, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}
	if p.Status == domain.ProductStatusExecuted || p.Status == domain.ProductStatusCancelled {
		return domain.NewAppError(422, "Cannot edit a product that is already executed or cancelled", nil)
	}
	// Apply allowed updates
	if updates.Name != "" {
		p.Name = updates.Name
	}
	if updates.Description != "" {
		p.Description = updates.Description
	}
	if updates.ImageURL != "" {
		p.ImageURL = updates.ImageURL
	}
	if updates.VideoURL != "" {
		p.VideoURL = updates.VideoURL
	}
	if updates.EntranceFee.IsPositive() && p.Status == domain.ProductStatusDraft {
		p.EntranceFee = updates.EntranceFee
	}
	if updates.MinBidValue.IsPositive() && p.Status == domain.ProductStatusDraft {
		p.MinBidValue = updates.MinBidValue
	}
	if updates.MaxBidValue != nil && p.Status == domain.ProductStatusDraft {
		p.MaxBidValue = updates.MaxBidValue
	}
	if updates.StartAt != nil {
		p.StartAt = updates.StartAt
	}
	if updates.CloseAt != nil {
		p.CloseAt = updates.CloseAt
	}
	if err := s.productRepo.Update(p); err != nil {
		return err
	}
	s.logAudit(actorID, "lub.product.updated", "lub_product", id.String(), p.Name)
	return nil
}

// ActivateProduct transitions a product from draft → active.
func (s *Service) ActivateProduct(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	p, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}
	if p.Status != domain.ProductStatusDraft {
		return domain.NewAppError(422, "Only draft products can be activated", nil)
	}
	if !p.EntranceFee.IsPositive() {
		return domain.NewAppError(422, "Entrance fee must be set before activating", nil)
	}
	if p.CloseAt == nil {
		return domain.NewAppError(422, "CloseAt must be set before activating", nil)
	}
	if err := s.productRepo.UpdateStatus(id, domain.ProductStatusActive); err != nil {
		return err
	}
	s.logAudit(actorID, "lub.product.activated", "lub_product", id.String(), p.Name)
	return nil
}

// PostponeProduct extends the CloseAt of an active or postponed product.
func (s *Service) PostponeProduct(ctx context.Context, id uuid.UUID, newCloseAt time.Time, actorID uuid.UUID) error {
	p, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}
	if p.Status != domain.ProductStatusActive && p.Status != domain.ProductStatusPostponed {
		return domain.NewAppError(422, "Only active or postponed products can be postponed", nil)
	}
	now := time.Now().UTC()
	nc := newCloseAt.UTC()
	if !nc.After(now) {
		return domain.NewAppError(422, "New close time must be in the future", nil)
	}
	if p.CloseAt != nil && !nc.After(*p.CloseAt) {
		return domain.NewAppError(422, "New close time must be later than the current close time", nil)
	}
	p.CloseAt = &nc
	p.PostponeCount++
	pp := now
	p.PostponedAt = &pp
	p.Status = domain.ProductStatusPostponed
	if err := s.productRepo.Update(p); err != nil {
		return err
	}
	s.logAudit(actorID, "lub.product.postponed", "lub_product", id.String(),
		fmt.Sprintf("new_close_at=%s", nc.Format(time.RFC3339)))
	return nil
}

// CancelProduct cancels an active/draft product. Admin-only.
func (s *Service) CancelProduct(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	p, err := s.productRepo.GetByID(id)
	if err != nil {
		return err
	}
	if p.Status == domain.ProductStatusExecuted {
		return domain.NewAppError(422, "Cannot cancel an already executed product", nil)
	}
	if p.Status == domain.ProductStatusCancelled {
		return domain.NewAppError(422, "Product is already cancelled", nil)
	}
	if err := s.productRepo.UpdateStatus(id, domain.ProductStatusCancelled); err != nil {
		return err
	}
	s.logAudit(actorID, "lub.product.cancelled", "lub_product", id.String(), p.Name)
	return nil
}

// ---------------------- Participation (entrance fee) ----------------------

// RecordParticipation records that a participant paid the entrance fee.
// Idempotent: if a participation already exists for (productID, participantID), returns it.
// transactionID is the Chapa tx_ref (used for idempotency).
func (s *Service) RecordParticipation(
	ctx context.Context,
	productID uuid.UUID,
	participantID string,
	participantName, participantEmail, participantPhone string,
	transactionID string,
	feePaid domain.Money,
) (*domain.LUBParticipation, error) {
	// Idempotency: check existing by tx_ref
	if transactionID != "" {
		if existing, _ := s.participationRepo.GetByTransactionID(transactionID); existing != nil {
			return existing, nil
		}
	}

	// Check if product is accepting participants
	p, err := s.productRepo.GetByID(productID)
	if err != nil {
		return nil, err
	}
	if p.Status != domain.ProductStatusActive && p.Status != domain.ProductStatusPostponed {
		return nil, domain.NewAppError(422, "Product is not currently accepting participants", nil)
	}
	if time.Now().UTC().After(*p.CloseAt) {
		return nil, domain.NewAppError(422, "Bidding period has closed for this product", nil)
	}

	// Check if user already has an active participation
	if existing, _ := s.participationRepo.GetByProductAndParticipant(productID, participantID); existing != nil {
		if existing.Status == domain.LUBParticipationStatusActive {
			return existing, nil // Already participating — idempotent
		}
	}

	part := &domain.LUBParticipation{
		ID:               uuid.New(),
		ProductID:        productID,
		ParticipantID:    participantID,
		ParticipantName:  participantName,
		ParticipantEmail: participantEmail,
		ParticipantPhone: participantPhone,
		TransactionID:    transactionID,
		FeePaid:          feePaid,
		Status:           domain.LUBParticipationStatusActive,
	}
	if err := s.participationRepo.Create(part); err != nil {
		return nil, err
	}
	return part, nil
}

// RefundParticipation marks a participation record as refunded.
func (s *Service) RefundParticipation(ctx context.Context, participationID uuid.UUID) error {
	return s.participationRepo.UpdateStatus(participationID, domain.LUBParticipationStatusRefunded)
}

// ---------------------- Bid entry management ----------------------

// PlaceBid places a new bid entry for a participant.
// The participant must have paid the entrance fee first.
// Returns ErrConflict if the same (product, participant, value) already exists.
func (s *Service) PlaceBid(
	ctx context.Context,
	productID uuid.UUID,
	participantID string,
	bidValue domain.Money,
) (*domain.BidEntry, error) {
	// Validate product state
	p, err := s.productRepo.GetByID(productID)
	if err != nil {
		return nil, err
	}
	if err := s.validateBiddingOpen(p); err != nil {
		return nil, err
	}
	if err := s.validateBidValue(p, bidValue); err != nil {
		return nil, err
	}

	// Must have paid entrance fee
	if err := s.requireParticipation(productID, participantID); err != nil {
		return nil, err
	}

	entry := &domain.BidEntry{
		ID:            uuid.New(),
		ProductID:     productID,
		ParticipantID: participantID,
		BidValue:      bidValue,
	}
	if err := s.bidEntryRepo.Create(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// UpdateBid changes the bid_value on an existing BidEntry.
// The old (product, participant, value) triple is freed; the new value must not
// already exist for this (product, participant) pair.
func (s *Service) UpdateBid(
	ctx context.Context,
	entryID uuid.UUID,
	participantID string,
	newBidValue domain.Money,
) (*domain.BidEntry, error) {
	entry, err := s.bidEntryRepo.GetByID(entryID)
	if err != nil {
		return nil, err
	}
	if entry.ParticipantID != participantID {
		return nil, domain.ErrForbidden
	}

	// Validate product state
	p, err := s.productRepo.GetByID(entry.ProductID)
	if err != nil {
		return nil, err
	}
	if err := s.validateBiddingOpen(p); err != nil {
		return nil, err
	}
	if err := s.validateBidValue(p, newBidValue); err != nil {
		return nil, err
	}

	// Check the new value is not already taken by this participant
	if existing, _ := s.bidEntryRepo.GetByProductAndParticipantAndValue(entry.ProductID, participantID, newBidValue); existing != nil {
		if existing.ID != entryID {
			return nil, domain.NewAppError(409, "You already have a bid entry with that value", nil)
		}
	}

	entry.BidValue = newBidValue
	if err := s.bidEntryRepo.Update(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// ---------------------- Helpers ----------------------

func (s *Service) validateBiddingOpen(p *domain.Product) error {
	if p.Status != domain.ProductStatusActive && p.Status != domain.ProductStatusPostponed {
		return domain.NewAppError(422, "This product is not currently accepting bids", nil)
	}
	if p.CloseAt != nil && time.Now().UTC().After(*p.CloseAt) {
		return domain.NewAppError(422, "The bidding period for this product has closed", nil)
	}
	return nil
}

func (s *Service) validateBidValue(p *domain.Product, v domain.Money) error {
	if v.LessThan(p.MinBidValue) {
		return domain.NewAppError(422, fmt.Sprintf("Bid value must be at least %.2f", p.MinBidValue.Float64()), nil)
	}
	if p.MaxBidValue != nil && p.MaxBidValue.LessThan(v) {
		return domain.NewAppError(422, fmt.Sprintf("Bid value must not exceed %.2f", p.MaxBidValue.Float64()), nil)
	}
	return nil
}

func (s *Service) requireParticipation(productID uuid.UUID, participantID string) error {
	part, err := s.participationRepo.GetByProductAndParticipant(productID, participantID)
	if err != nil {
		return err
	}
	if part == nil || part.Status != domain.LUBParticipationStatusActive {
		return domain.NewAppError(402, "You must pay the entrance fee before placing a bid", nil)
	}
	return nil
}

func (s *Service) logAudit(actorID uuid.UUID, action, entity, entityID, value string) {
	if s.audit == nil {
		return
	}
	s.audit.Log(&domain.AuditLog{
		UserID:   actorID,
		Action:   action,
		Entity:   entity,
		EntityID: entityID,
		NewValue: value,
	}, nil)
}
