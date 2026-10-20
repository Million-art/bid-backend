package lub

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// ---------------------- Auction queries ----------------------

func (s *Service) GetAuctionByID(id uuid.UUID) (*domain.Auction, error) {
	return s.auctionRepo.GetByID(id)
}

func (s *Service) ListAuctions(status, search string, page, limit int) ([]domain.Auction, int64, error) {
	offset := (page - 1) * limit
	return s.auctionRepo.List(status, search, limit, offset)
}

func (s *Service) ListPublicAuctions(page, limit int) ([]domain.Auction, int64, error) {
	offset := (page - 1) * limit
	return s.auctionRepo.ListPublic(limit, offset)
}

// ---------------------- Participation queries ----------------------

func (s *Service) GetParticipation(auctionID uuid.UUID, participantID string) (*domain.AuctionParticipation, error) {
	return s.participationRepo.GetByAuctionAndParticipant(auctionID, participantID)
}

func (s *Service) ListParticipations(auctionID uuid.UUID, page, limit int) ([]domain.AuctionParticipation, int64, error) {
	offset := (page - 1) * limit
	return s.participationRepo.ListByAuction(auctionID, limit, offset)
}

func (s *Service) ListParticipantParticipations(participantID string, page, limit int) ([]domain.AuctionParticipation, int64, error) {
	offset := (page - 1) * limit
	return s.participationRepo.ListByParticipant(participantID, limit, offset)
}

func (s *Service) CountParticipants(auctionID uuid.UUID) (int64, error) {
	return s.participationRepo.CountByAuction(auctionID)
}

// ---------------------- Bid queries ----------------------

func (s *Service) GetBid(id uuid.UUID) (*domain.Bid, error) {
	b, err := s.bidRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	b.PopulateUpdateState()
	return b, nil
}

func (s *Service) ListMyBids(auctionID uuid.UUID, participantID string) ([]domain.Bid, error) {
	bids, err := s.bidRepo.ListByAuctionAndParticipant(auctionID, participantID)
	if err != nil {
		return nil, err
	}
	for i := range bids {
		bids[i].PopulateUpdateState()
	}
	return bids, nil
}

func (s *Service) ListAuctionBids(auctionID uuid.UUID, page, limit int) ([]domain.Bid, int64, error) {
	offset := (page - 1) * limit
	bids, total, err := s.bidRepo.ListByAuction(auctionID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for i := range bids {
		bids[i].PopulateUpdateState()
	}
	return bids, total, nil
}

func (s *Service) GetAuctionAudit(auctionID uuid.UUID) (*domain.AuctionAudit, error) {
	a, err := s.auctionRepo.GetByID(auctionID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, domain.NewAppError(404, "Auction not found", nil)
	}

	if a.Status != domain.AuctionStatusExecuted && a.Status != domain.AuctionStatusClosed {
		return nil, domain.NewAppError(403, "Audit log is published only after the auction round has completed", nil)
	}

	bids, total, err := s.bidRepo.ListByAuction(auctionID, 10000, 0)
	if err != nil {
		return nil, err
	}

	type valGroup struct {
		bidValue domain.Money
		count    int
		isWinner bool
	}
	var groups []valGroup
	groupMap := make(map[string]int)

	for _, b := range bids {
		valStr := b.BidValue.String()
		idx, exists := groupMap[valStr]
		if !exists {
			groupMap[valStr] = len(groups)
			groups = append(groups, valGroup{
				bidValue: b.BidValue,
				count:    1,
				isWinner: b.IsWinner,
			})
		} else {
			groups[idx].count++
			if b.IsWinner {
				groups[idx].isWinner = true
			}
		}
	}

	uniqueCount := 0
	duplicateCount := 0
	entries := make([]domain.AuctionAuditEntry, len(groups))

	for i, g := range groups {
		isUnique := g.count == 1
		if isUnique {
			uniqueCount++
		} else {
			duplicateCount += g.count
		}
		isWinner := g.isWinner
		if a.WinnerBidValue != nil && g.bidValue.Equal(*a.WinnerBidValue) {
			isWinner = true
		}
		entries[i] = domain.AuctionAuditEntry{
			BidValue: g.bidValue,
			Count:    g.count,
			IsUnique: isUnique,
			IsWinner: isWinner,
		}
	}

	audit := &domain.AuctionAudit{
		AuctionID:           a.ID,
		Title:               a.Title,
		Status:              a.Status,
		WinnerBidValue:      a.WinnerBidValue,
		WinnerName:          a.WinnerName,
		TotalBids:           total,
		UniqueBidsCount:     uniqueCount,
		DuplicatedBidsCount: duplicateCount,
		MinBidValue:         a.MinBidValue,
		MaxBidValue:         a.MaxBidValue,
		Entries:             entries,
	}

	return audit, nil
}
