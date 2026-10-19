package http

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services/chapapayment"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AuctionHandler handles participant-facing auction endpoints.
type AuctionHandler struct {
	svc             *lub.Service
	chapaPaymentSvc *chapapayment.Service
	chapaClient     ports.PaymentProvider
	userRepo        ports.UserRepository
	cfg             *env.Config
}

func NewAuctionHandler(
	svc *lub.Service,
	chapaPaymentSvc *chapapayment.Service,
	chapaClient ports.PaymentProvider,
	userRepo ports.UserRepository,
	cfg *env.Config,
) *AuctionHandler {
	return &AuctionHandler{
		svc:             svc,
		chapaPaymentSvc: chapaPaymentSvc,
		chapaClient:     chapaClient,
		userRepo:        userRepo,
		cfg:             cfg,
	}
}

type RecordParticipationRequest struct {
	AuctionID     string  `json:"auctionId"     validate:"omitempty,uuid"`
	ProductID     string  `json:"productId"     validate:"omitempty,uuid"`
	ParticipantID string  `json:"participantId"`
	TransactionID string  `json:"transactionId"`
	FeePaid       float64 `json:"feePaid"`
	BidValue      float64 `json:"bidValue"`
	CheckoutURL   string  `json:"checkoutUrl"`
	ReturnURL     string  `json:"returnUrl"`
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

func formatChapaPhone(raw string) string {
	digits := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(raw), "+", ""), " ", "")
	if strings.HasPrefix(digits, "251") && len(digits) == 12 {
		digits = "0" + digits[3:]
	}
	if len(digits) == 10 && (strings.HasPrefix(digits, "09") || strings.HasPrefix(digits, "07")) {
		return digits
	}
	return ""
}

func (h *AuctionHandler) RecordParticipation(c fiber.Ctx) error {
	var req RecordParticipationRequest
	_ = c.Bind().JSON(&req)

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

	participantID := strings.TrimSpace(GetUserID(c))
	if participantID == "" {
		participantID = strings.TrimSpace(req.ParticipantID)
	}
	userUUID, err := uuid.Parse(participantID)
	if err != nil || userUUID == uuid.Nil {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}

	bidMoney := domain.Zero()
	if req.BidValue > 0 {
		bidMoney = domain.NewMoney(req.BidValue)
	}

	a, err := h.svc.ValidateAuctionAndBid(auctionID, participantID, bidMoney)
	if err != nil {
		return mapLUBError(c, err)
	}

	if h.chapaClient == nil || h.chapaPaymentSvc == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Chapa payment gateway is not configured", nil)
	}

	txRef := "R-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:20]

	if err := h.chapaPaymentSvc.CreatePendingPayment(c.Context(), txRef, userUUID, auctionID, a.EntranceFee); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to create pending payment", err)
	}
	if bidMoney.IsPositive() {
		h.chapaPaymentSvc.SetPendingBid(txRef, bidMoney)
	}

	email := "bidder@carramarket.com"
	firstName := "CarraMarket"
	lastName := "Bidder"
	phone := ""

	if h.userRepo != nil {
		if u, uErr := h.userRepo.GetByID(userUUID); uErr == nil && u != nil {
			if strings.Contains(u.Email, "@") {
				email = strings.TrimSpace(u.Email)
			}
			parts := strings.Fields(strings.TrimSpace(u.FullName))
			if len(parts) > 0 {
				firstName = parts[0]
				if len(parts) > 1 {
					lastName = strings.Join(parts[1:], " ")
				}
			}
			phone = formatChapaPhone(u.Phone)
		}
	}

	callbackURL := ""
	if h.cfg != nil && (strings.HasPrefix(h.cfg.CallbackURL, "http://") || strings.HasPrefix(h.cfg.CallbackURL, "https://")) {
		callbackURL = strings.TrimSpace(h.cfg.CallbackURL)
	} else {
		scheme := c.Protocol()
		if proto := strings.TrimSpace(c.Get("X-Forwarded-Proto")); proto != "" {
			scheme = strings.Split(proto, ",")[0]
		}
		host := c.Host()
		if host != "" {
			callbackURL = fmt.Sprintf("%s://%s/api/webhooks/chapa", scheme, host)
		}
	}

	returnBase := strings.TrimSpace(req.ReturnURL)
	if returnBase == "" {
		returnBase = strings.TrimSpace(req.CheckoutURL)
	}
	if returnBase == "" {
		origin := strings.TrimSpace(c.Get("Origin"))
		if origin == "" && h.cfg != nil {
			origin = strings.TrimSpace(h.cfg.ParticipantAppURL)
		}
		if origin == "" {
			origin = "https://carramarket.com"
		}
		returnBase = fmt.Sprintf("%s/auctions/%s", strings.TrimRight(origin, "/"), auctionID.String())
	}

	parsedReturn, uErr := url.Parse(returnBase)
	if uErr == nil {
		q := parsedReturn.Query()
		q.Set("tx_ref", txRef)
		if bidMoney.IsPositive() {
			q.Set("bid_value", bidMoney.String())
		}
		parsedReturn.RawQuery = q.Encode()
		returnBase = parsedReturn.String()
	}

	meta := map[string]interface{}{
		"auction_id": auctionID.String(),
		"user_id":    userUUID.String(),
	}
	if bidMoney.IsPositive() {
		meta["bid_value"] = bidMoney.String()
	}

	initResp, err := h.chapaClient.InitializeTransaction(ports.ChapaInitializeRequest{
		Amount:      a.EntranceFee.String(),
		Currency:    "ETB",
		Email:       email,
		FirstName:   firstName,
		LastName:    lastName,
		PhoneNumber: phone,
		TxRef:       txRef,
		CallbackURL: callbackURL,
		ReturnURL:   returnBase,
		Title:       "CarraMarket",
		Description: "Auction entrance fee and bid",
		Meta:        meta,
	})
	if err != nil {
		return response.Error(c, fiber.StatusBadGateway, "Could not initialize Chapa payment: "+err.Error(), err)
	}

	return response.Success(c, fiber.StatusOK, "Chapa checkout initialized", fiber.Map{
		"checkoutUrl": initResp.CheckoutURL,
		"txRef":       initResp.TxRef,
	})
}

func (h *AuctionHandler) VerifyChapaPayment(c fiber.Ctx) error {
	txRef := strings.TrimSpace(c.Params("txRef"))
	if txRef == "" {
		txRef = strings.TrimSpace(c.Query("tx_ref"))
	}
	if txRef == "" {
		txRef = strings.TrimSpace(c.Query("trx_ref"))
	}
	var body struct {
		TxRef    string  `json:"txRef"`
		TxRefAlt string  `json:"tx_ref"`
		BidValue float64 `json:"bidValue"`
	}
	_ = c.Bind().JSON(&body)
	if txRef == "" {
		txRef = strings.TrimSpace(body.TxRef)
	}
	if txRef == "" {
		txRef = strings.TrimSpace(body.TxRefAlt)
	}
	if txRef == "" {
		return response.Error(c, fiber.StatusBadRequest, "txRef is required", nil)
	}

	bidVal := body.BidValue
	if bidVal <= 0 {
		if qVal := strings.TrimSpace(c.Query("bid_value")); qVal != "" {
			if parsed, pErr := strconv.ParseFloat(qVal, 64); pErr == nil && parsed > 0 {
				bidVal = parsed
			}
		}
	}

	if h.chapaClient == nil || h.chapaPaymentSvc == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "Chapa payment gateway is not configured", nil)
	}

	snap, err := h.chapaClient.VerifyTransaction(txRef)
	if err != nil {
		return response.Error(c, fiber.StatusBadGateway, "Failed to verify Chapa transaction: "+err.Error(), err)
	}

	metaMap, _ := snap.Meta.(map[string]interface{})
	verifiedSnap := &chapapayment.VerifiedSnapshot{
		TxRef:          txRef,
		ChapaReference: snap.Reference,
		APIStatus:      "success",
		DataStatus:     snap.Status,
		Amount:         snap.Amount,
		Meta:           metaMap,
		CustomerEmail:  snap.CustomerEmail,
		CustomerFirst:  snap.CustomerFirst,
	}
	if bidVal > 0 {
		verifiedSnap.BidValue = domain.NewMoney(bidVal)
	}

	if err := h.chapaPaymentSvc.ProcessVerified(c.Context(), verifiedSnap); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to process verified payment", err)
	}

	out := fiber.Map{
		"verified": strings.EqualFold(snap.Status, "success"),
		"status":   snap.Status,
		"txRef":    txRef,
	}

	if p, pErr := h.chapaPaymentSvc.GetPaymentByTxRef(c.Context(), txRef); pErr == nil && p != nil && p.AuctionID != nil && p.UserID != nil {
		part, _ := h.svc.GetParticipation(*p.AuctionID, p.UserID.String())
		bids, _ := h.svc.ListMyBids(*p.AuctionID, p.UserID.String())
		out["auctionId"] = p.AuctionID.String()
		out["participating"] = part != nil && part.Status == domain.ParticipationStatusActive
		out["participation"] = part
		out["bids"] = bids
	}

	return response.Success(c, fiber.StatusOK, "Payment verification complete", out)
}

func (h *AuctionHandler) ChapaWebhook(c fiber.Ctx) error {
	txRef := strings.TrimSpace(c.Query("tx_ref"))
	if txRef == "" {
		txRef = strings.TrimSpace(c.Query("trx_ref"))
	}
	var payload map[string]interface{}
	_ = c.Bind().JSON(&payload)
	if txRef == "" && payload != nil {
		if v, ok := payload["tx_ref"].(string); ok {
			txRef = strings.TrimSpace(v)
		}
		if txRef == "" {
			if v, ok := payload["trx_ref"].(string); ok {
				txRef = strings.TrimSpace(v)
			}
		}
	}
	if txRef == "" {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ignored"})
	}

	if h.chapaClient == nil || h.chapaPaymentSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "unavailable"})
	}

	snap, err := h.chapaClient.VerifyTransaction(txRef)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"status": "verify_failed"})
	}
	metaMap, _ := snap.Meta.(map[string]interface{})
	verifiedSnap := &chapapayment.VerifiedSnapshot{
		TxRef:          txRef,
		ChapaReference: snap.Reference,
		APIStatus:      "success",
		DataStatus:     snap.Status,
		Amount:         snap.Amount,
		Meta:           metaMap,
		CustomerEmail:  snap.CustomerEmail,
		CustomerFirst:  snap.CustomerFirst,
	}
	if err := h.chapaPaymentSvc.ProcessVerified(c.Context(), verifiedSnap); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "retry"})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func (h *AuctionHandler) ListMyParticipations(c fiber.Ctx) error {
	userID, err := GetUserIDOrUnauthorized(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", err)
	}
	page, limit, _, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}
	parts, total, err := h.svc.ListParticipantParticipations(userID, page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list participations", err)
	}
	items := make([]fiber.Map, 0, len(parts))
	for i := range parts {
		part := parts[i]
		auction, aErr := h.svc.GetAuctionByID(part.AuctionID)
		if aErr != nil || auction == nil {
			continue
		}
		if auction.Status != domain.AuctionStatusExecuted {
			auction.WinnerBidID = nil
			auction.WinnerParticipantID = nil
			auction.WinnerBidValue = nil
		}
		bids, _ := h.svc.ListMyBids(part.AuctionID, userID)
		items = append(items, fiber.Map{
			"auction":       auction,
			"participation": part,
			"bids":          bids,
		})
	}
	return response.Success(c, fiber.StatusOK, "Participations retrieved", fiber.Map{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
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
