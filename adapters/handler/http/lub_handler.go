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

// LUBHandler handles participant-facing LUB endpoints:
//   - Pay entrance fee & register participation
//   - Place a bid
//   - Update a bid
//   - View my bids
//   - View public products
type LUBHandler struct {
	svc *lub.Service
}

func NewLUBHandler(svc *lub.Service) *LUBHandler {
	return &LUBHandler{svc: svc}
}

// ---------------------- Request/Response types ----------------------

// RecordParticipationRequest is sent by the client-backend after a successful
// entrance fee payment via Chapa webhook. The client-backend calls this admin
// API with its shared client secret.
type RecordParticipationRequest struct {
	ProductID        string  `json:"productId"        validate:"required,uuid"`
	ParticipantID    string  `json:"participantId"    validate:"required"`
	ParticipantName  string  `json:"participantName"  validate:"omitempty"`
	ParticipantEmail string  `json:"participantEmail" validate:"omitempty,email"`
	ParticipantPhone string  `json:"participantPhone" validate:"omitempty"`
	TransactionID    string  `json:"transactionId"    validate:"required"`
	FeePaid          float64 `json:"feePaid"          validate:"required,gt=0"`
}

// PlaceBidRequest is the body for placing a new bid entry.
type PlaceBidRequest struct {
	ProductID     string  `json:"productId"     validate:"required,uuid"`
	ParticipantID string  `json:"participantId" validate:"required"`
	BidValue      float64 `json:"bidValue"      validate:"required,gt=0"`
}

// UpdateBidRequest is the body for updating a bid entry's value.
type UpdateBidRequest struct {
	NewBidValue float64 `json:"newBidValue" validate:"required,gt=0"`
}

// ---------------------- Handlers ----------------------

// ListPublicProducts returns all publicly visible products (no auth required).
func (h *LUBHandler) ListPublicProducts(c fiber.Ctx) error {
	page, limit, _, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}
	products, total, err := h.svc.ListPublicProducts(page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list products", err)
	}
	return response.Success(c, fiber.StatusOK, "Products retrieved", fiber.Map{
		"items": products,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetPublicProduct returns a single product by ID (no auth required).
func (h *LUBHandler) GetPublicProduct(c fiber.Ctx) error {
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	p, err := h.svc.GetProductByID(id)
	if err != nil {
		return mapLUBError(c, err)
	}
	// Redact winner details until draw is executed
	if p.Status != domain.ProductStatusExecuted {
		p.WinnerID = nil
		p.WinnerBidValue = nil
		p.WinnerName = ""
		p.WinnerEmail = ""
		p.WinnerPhone = ""
	}
	return response.Success(c, fiber.StatusOK, "Product found", p)
}

// RecordParticipation is called by the client-backend (with client secret) after
// a successful entrance fee payment. It creates a LUBParticipation record.
func (h *LUBHandler) RecordParticipation(c fiber.Ctx) error {
	var req RecordParticipationRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	productID, _ := uuid.Parse(req.ProductID)

	part, err := h.svc.RecordParticipation(
		c.Context(),
		productID,
		strings.TrimSpace(req.ParticipantID),
		req.ParticipantName,
		req.ParticipantEmail,
		req.ParticipantPhone,
		req.TransactionID,
		domain.NewMoney(req.FeePaid),
	)
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusCreated, "Participation recorded", part)
}

// PlaceBid places a new bid entry for a participant.
// Called by client-backend with client secret after verifying participation.
func (h *LUBHandler) PlaceBid(c fiber.Ctx) error {
	var req PlaceBidRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	productID, _ := uuid.Parse(req.ProductID)

	entry, err := h.svc.PlaceBid(
		c.Context(),
		productID,
		strings.TrimSpace(req.ParticipantID),
		domain.NewMoney(req.BidValue),
	)
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusCreated, "Bid placed", entry)
}

// UpdateBid changes the bid_value on an existing BidEntry.
func (h *LUBHandler) UpdateBid(c fiber.Ctx) error {
	entryID, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	// participantId comes from query or body (client-backend supplies it)
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		var body struct {
			ParticipantID string `json:"participantId"`
		}
		_ = c.Bind().JSON(&body)
		participantID = strings.TrimSpace(body.ParticipantID)
	}
	if participantID == "" {
		return response.Error(c, fiber.StatusBadRequest, "participantId is required", nil)
	}

	var req UpdateBidRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	entry, err := h.svc.UpdateBid(c.Context(), entryID, participantID, domain.NewMoney(req.NewBidValue))
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Bid updated", entry)
}

// GetMyBids returns all bid entries a participant has placed on a product.
func (h *LUBHandler) GetMyBids(c fiber.Ctx) error {
	productID, err := PathParamUUID(c, "productId", "productId")
	if err != nil {
		return err
	}
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		return response.Error(c, fiber.StatusBadRequest, "participantId query param is required", nil)
	}
	entries, err := h.svc.ListMyBids(productID, participantID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list bids", err)
	}
	return response.Success(c, fiber.StatusOK, "Bids retrieved", entries)
}

// GetMyParticipation checks whether a participant has paid the entrance fee for a product.
func (h *LUBHandler) GetMyParticipation(c fiber.Ctx) error {
	productID, err := PathParamUUID(c, "productId", "productId")
	if err != nil {
		return err
	}
	participantID := strings.TrimSpace(c.Query("participantId"))
	if participantID == "" {
		return response.Error(c, fiber.StatusBadRequest, "participantId query param is required", nil)
	}
	part, err := h.svc.GetParticipation(productID, participantID)
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
