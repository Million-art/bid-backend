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

func (s *Service) CountParticipants(auctionID uuid.UUID) (int64, error) {
	return s.participationRepo.CountByAuction(auctionID)
}

// ---------------------- Bid queries ----------------------

func (s *Service) GetBid(id uuid.UUID) (*domain.Bid, error) {
	return s.bidRepo.GetByID(id)
}

func (s *Service) ListMyBids(auctionID uuid.UUID, participantID string) ([]domain.Bid, error) {
	return s.bidRepo.ListByAuctionAndParticipant(auctionID, participantID)
}

func (s *Service) ListAuctionBids(auctionID uuid.UUID, page, limit int) ([]domain.Bid, int64, error) {
	offset := (page - 1) * limit
	return s.bidRepo.ListByAuction(auctionID, limit, offset)
}
