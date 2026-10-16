package http

import (
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AuctionHandler handles participant-facing auction endpoints.
type AuctionHandler struct {
	svc *lub.Service
}

func NewAuctionHandler(svc *lub.Service) *AuctionHandler {
	return &AuctionHandler{svc: svc}
}

type RecordParticipationRequest struct {
	AuctionID     string  `json:"auctionId"     validate:"omitempty,uuid"`
	ProductID     string  `json:"productId"     validate:"omitempty,uuid"`
	ParticipantID string  `json:"participantId"`
	TransactionID string  `json:"transactionId"`
	FeePaid       float64 `json:"feePaid"`
}

type PlaceBidRequest struct {
	AuctionID     string  `json:"auctionId"     validate:"omitempty,uuid"`
	ProductID     string  `json:"productId"     validate:"omitempty,uuid"`
	ParticipantID string  `json:"participantId"`
	BidValue      float64 `json:"bidValue"      validate:"required,gt=0"`
}

type UpdateBidRequest struct {
	NewBidValue float64 `json:"newBidValue" validate:"required,gt=0"`
}

func (h *AuctionHandler) ListPublicAuctions(c fiber.Ctx) error {
	page, limit, _, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}
	auctions, total, err := h.svc.ListPublicAuctions(page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list auctions", err)
	}
	return response.Success(c, fiber.StatusOK, "Auctions retrieved", fiber.Map{
		"items": auctions,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (h *AuctionHandler) GetPublicAuction(c fiber.Ctx) error {
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	a, err := h.svc.GetAuctionByID(id)
	if err != nil {
		return mapLUBError(c, err)
	}
	if a.Status != domain.AuctionStatusExecuted {
		a.WinnerBidID = nil
		a.WinnerParticipantID = nil
		a.WinnerBidValue = nil
	}
	return response.Success(c, fiber.StatusOK, "Auction found", a)
}

func (h *AuctionHandler) RecordParticipation(c fiber.Ctx) error {
	var req RecordParticipationRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	rawID := strings.TrimSpace(req.AuctionID)
	if rawID == "" {
		rawID = strings.TrimSpace(req.ProductID)
	}
	if rawID == "" {
		rawID = strings.TrimSpace(c.Params("auctionId"))
	}
	if rawID == "" {
		rawID = strings.TrimSpace(c.Params("productId"))
	}
	auctionID, err := uuid.Parse(rawID)
	if err != nil || auctionID == uuid.Nil {
		return response.Error(c, fiber.StatusBadRequest, "valid auctionId is required", nil)
	}

	participantID := strings.TrimSpace(req.ParticipantID)
	if participantID == "" {
		participantID = strings.TrimSpace(GetUserID(c))
	}
	if participantID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}

	txID := strings.TrimSpace(req.TransactionID)
	if txID == "" {
		txID = "tx_" + uuid.New().String()
	}

	feePaid := domain.NewMoney(req.FeePaid)
	if req.FeePaid <= 0 {
		a, aErr := h.svc.GetAuctionByID(auctionID)
		if aErr != nil {
			return mapLUBError(c, aErr)
		}
		feePaid = a.EntranceFee
	}

	part, err := h.svc.RecordParticipation(
		c.Context(),
		auctionID,
		participantID,
		txID,
		feePaid,
	)
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusCreated, "Participation recorded", fiber.Map{
		"participation": part,
	})
}

func (h *AuctionHandler) PlaceBid(c fiber.Ctx) error {
	var req PlaceBidRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	rawID := strings.TrimSpace(req.AuctionID)
	if rawID == "" {
		rawID = strings.TrimSpace(req.ProductID)
	}
	auctionID, err := uuid.Parse(rawID)
	if err != nil || auctionID == uuid.Nil {
		return response.Error(c, fiber.StatusBadRequest, "valid auctionId is required", nil)
	}

	participantID := strings.TrimSpace(req.ParticipantID)
	if participantID == "" {
		participantID = strings.TrimSpace(GetUserID(c))
	}
	if participantID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}

	bid, err := h.svc.PlaceBid(
		c.Context(),
		auctionID,
		participantID,
		domain.NewMoney(req.BidValue),
	)
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusCreated, "Bid placed", bid)
}

func (h *AuctionHandler) UpdateBid(c fiber.Ctx) error {
	bidID, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		var body struct {
			ParticipantID string `json:"participantId"`
		}
		_ = c.Bind().JSON(&body)
		participantID = strings.TrimSpace(body.ParticipantID)
	}
	if participantID == "" {
		participantID = strings.TrimSpace(GetUserID(c))
	}
	if participantID == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}

	var req UpdateBidRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	bid, err := h.svc.UpdateBid(c.Context(), bidID, participantID, domain.NewMoney(req.NewBidValue))
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Bid updated", bid)
}

func (h *AuctionHandler) GetMyBids(c fiber.Ctx) error {
	paramName := "auctionId"
	if c.Params("auctionId") == "" {
		paramName = "productId"
	}
	auctionID, err := PathParamUUID(c, paramName, paramName)
	if err != nil {
		return err
	}
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		participantID = strings.TrimSpace(GetUserID(c))
	}
	if participantID == "" {
		return response.Success(c, fiber.StatusOK, "Bids retrieved", []domain.Bid{})
	}
	bids, err := h.svc.ListMyBids(auctionID, participantID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list bids", err)
	}
	return response.Success(c, fiber.StatusOK, "Bids retrieved", bids)
}

func (h *AuctionHandler) GetMyParticipation(c fiber.Ctx) error {
	paramName := "auctionId"
	if c.Params("auctionId") == "" {
		paramName = "productId"
	}
	auctionID, err := PathParamUUID(c, paramName, paramName)
	if err != nil {
		return err
	}
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		participantID = strings.TrimSpace(GetUserID(c))
	}
	if participantID == "" {
		return response.Success(c, fiber.StatusOK, "Not participating", fiber.Map{"participating": false})
	}
	part, err := h.svc.GetParticipation(auctionID, participantID)
	if err != nil {
		return mapLUBError(c, err)
	}
	if part == nil {
		return response.Success(c, fiber.StatusOK, "Not participating", fiber.Map{"participating": false})
	}
	return response.Success(c, fiber.StatusOK, "Participation found", fiber.Map{
		"participating": true,
		"participation": part,
	})
}
