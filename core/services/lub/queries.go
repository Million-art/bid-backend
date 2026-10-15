package lub

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// ---------------------- Product queries ----------------------

// GetProductByID returns a product by ID.
func (s *Service) GetProductByID(id uuid.UUID) (*domain.Product, error) {
	return s.productRepo.GetByID(id)
}

// ListProducts returns a paginated list of all products (admin).
func (s *Service) ListProducts(status, search string, page, limit int) ([]domain.Product, int64, error) {
	offset := (page - 1) * limit
	return s.productRepo.List(status, search, limit, offset)
}

// ListPublicProducts returns products visible to participants.
func (s *Service) ListPublicProducts(page, limit int) ([]domain.Product, int64, error) {
	offset := (page - 1) * limit
	return s.productRepo.ListPublic(limit, offset)
}

// ---------------------- Participation queries ----------------------

// GetParticipation returns the participation record for a user on a product.
func (s *Service) GetParticipation(productID uuid.UUID, participantID string) (*domain.LUBParticipation, error) {
	return s.participationRepo.GetByProductAndParticipant(productID, participantID)
}

// ListParticipations returns all participants for a product (admin overview).
func (s *Service) ListParticipations(productID uuid.UUID, page, limit int) ([]domain.LUBParticipation, int64, error) {
	offset := (page - 1) * limit
	return s.participationRepo.ListByProduct(productID, limit, offset)
}

// CountParticipants returns the number of participants for a product.
func (s *Service) CountParticipants(productID uuid.UUID) (int64, error) {
	return s.participationRepo.CountByProduct(productID)
}

// ---------------------- Bid entry queries ----------------------

// GetBidEntry returns a single bid entry by ID.
func (s *Service) GetBidEntry(id uuid.UUID) (*domain.BidEntry, error) {
	return s.bidEntryRepo.GetByID(id)
}

// ListMyBids returns all bid entries a participant has placed on a product.
func (s *Service) ListMyBids(productID uuid.UUID, participantID string) ([]domain.BidEntry, error) {
	return s.bidEntryRepo.ListByProductAndParticipant(productID, participantID)
}

// ListProductBidEntries returns all bid entries for a product (admin).
func (s *Service) ListProductBidEntries(productID uuid.UUID, page, limit int) ([]domain.BidEntry, int64, error) {
	offset := (page - 1) * limit
	return s.bidEntryRepo.ListByProduct(productID, limit, offset)
}
