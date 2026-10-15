package lub

import (
	"context"
	"fmt"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// DrawResult contains the outcome of executing a LUB draw.
type DrawResult struct {
	WinnerEntryID     uuid.UUID
	WinnerParticipant string
	WinnerBidValue    domain.Money
	HasWinner         bool // false when every bid value was placed by >1 participant
}

// ExecuteDraw finds the lowest unique bid and marks it as the winner.
// This is the canonical draw algorithm:
//
//	SELECT lowest bid_value placed by exactly one participant_id.
//
// If no unique bid exists, the product is left in "closed" status and
// HasWinner=false is returned so the admin can handle it (e.g. extend close_at).
//
// trigger is a short string such as "manual" or "scheduled" for audit purposes.
func (s *Service) ExecuteDraw(ctx context.Context, productID uuid.UUID, trigger string, actorID uuid.UUID) (*DrawResult, error) {
	p, err := s.productRepo.GetByID(productID)
	if err != nil {
		return nil, err
	}

	// Guard: only close/active/postponed products can be drawn
	switch p.Status {
	case domain.ProductStatusActive, domain.ProductStatusPostponed, domain.ProductStatusClosed:
		// allowed
	case domain.ProductStatusExecuted:
		return nil, domain.NewAppError(409, "Draw has already been executed for this product", nil)
	case domain.ProductStatusCancelled:
		return nil, domain.NewAppError(422, "Cannot execute draw on a cancelled product", nil)
	default:
		return nil, domain.NewAppError(422, fmt.Sprintf("Product status %q does not allow draw execution", p.Status), nil)
	}

	// Transition to "closed" to block new bids immediately
	if p.Status != domain.ProductStatusClosed {
		if err := s.productRepo.UpdateStatus(productID, domain.ProductStatusClosed); err != nil {
			return nil, err
		}
	}

	// Find winning entry (lowest unique bid_value)
	winningEntry, err := s.bidEntryRepo.FindWinner(productID)
	if err != nil {
		return nil, fmt.Errorf("draw: find winner: %w", err)
	}

	result := &DrawResult{}

	if winningEntry == nil {
		// No unique bid — log and return without executing
		s.logAudit(actorID, "lub.draw.no_winner",
			"lub_product", productID.String(),
			fmt.Sprintf("trigger=%s, no unique bid found", trigger))
		return result, nil
	}

	// Mark the winner on bid_entries and compute IsUnique flags for all
	if err := s.bidEntryRepo.MarkWinner(productID, winningEntry.ID); err != nil {
		return nil, fmt.Errorf("draw: mark winner: %w", err)
	}

	// Update the product with winner info and status=executed
	now := time.Now().UTC()
	p.WinnerID = &winningEntry.ParticipantID
	p.WinnerBidValue = &winningEntry.BidValue
	p.ExecutedAt = &now
	p.Status = domain.ProductStatusExecuted

	// Fetch participation to get winner name/email/phone
	if part, err := s.participationRepo.GetByProductAndParticipant(productID, winningEntry.ParticipantID); err == nil && part != nil {
		p.WinnerName = part.ParticipantName
		p.WinnerEmail = part.ParticipantEmail
		p.WinnerPhone = part.ParticipantPhone
	}

	if err := s.productRepo.Update(p); err != nil {
		return nil, fmt.Errorf("draw: save product winner: %w", err)
	}

	result.HasWinner = true
	result.WinnerEntryID = winningEntry.ID
	result.WinnerParticipant = winningEntry.ParticipantID
	result.WinnerBidValue = winningEntry.BidValue

	s.logAudit(actorID, "lub.draw.executed",
		"lub_product", productID.String(),
		fmt.Sprintf("trigger=%s winner=%s bid_value=%.2f",
			trigger, winningEntry.ParticipantID, winningEntry.BidValue.Float64()))

	return result, nil
}

// CloseProduct transitions an active/postponed product to "closed" without executing the draw.
// Useful to stop accepting bids before manually running ExecuteDraw.
func (s *Service) CloseProduct(ctx context.Context, productID uuid.UUID, actorID uuid.UUID) error {
	p, err := s.productRepo.GetByID(productID)
	if err != nil {
		return err
	}
	if p.Status != domain.ProductStatusActive && p.Status != domain.ProductStatusPostponed {
		return domain.NewAppError(422, "Only active or postponed products can be closed", nil)
	}
	if err := s.productRepo.UpdateStatus(productID, domain.ProductStatusClosed); err != nil {
		return err
	}
	s.logAudit(actorID, "lub.product.closed", "lub_product", productID.String(), p.Name)
	return nil
}

// ProcessDueProducts is called by the scheduler to auto-close products whose
// CloseAt has passed and (optionally) immediately execute the draw.
func (s *Service) ProcessDueProducts(ctx context.Context) error {
	products, err := s.productRepo.ListDueForClose()
	if err != nil {
		return err
	}
	for _, p := range products {
		pid := p.ID
		if _, err := s.ExecuteDraw(ctx, pid, "scheduled", uuid.Nil); err != nil {
			// Log but continue processing other products
			s.logAudit(uuid.Nil, "lub.draw.auto_error",
				"lub_product", pid.String(), err.Error())
		}
	}
	return nil
}
