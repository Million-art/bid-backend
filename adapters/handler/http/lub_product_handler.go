package http

import (
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// LUBProductHandler handles admin CRUD for LUB products.
type LUBProductHandler struct {
	svc *lub.Service
}

func NewLUBProductHandler(svc *lub.Service) *LUBProductHandler {
	return &LUBProductHandler{svc: svc}
}

// ---------------------- Request/Response types ----------------------

type CreateProductRequest struct {
	Name        string   `json:"name"        validate:"required,min=2,max=255"`
	Description string   `json:"description" validate:"omitempty"`
	ImageURL    string   `json:"imageUrl"    validate:"omitempty,url"`
	VideoURL    string   `json:"videoUrl"    validate:"omitempty,url"`
	EntranceFee float64  `json:"entranceFee" validate:"required,gt=0"`
	MinBidValue float64  `json:"minBidValue" validate:"omitempty,gt=0"`
	MaxBidValue *float64 `json:"maxBidValue" validate:"omitempty,gt=0"`
	StartAt     *string  `json:"startAt"     validate:"omitempty"`
	CloseAt     *string  `json:"closeAt"     validate:"omitempty"`
}

type UpdateProductRequest struct {
	Name        string   `json:"name"        validate:"omitempty,min=2,max=255"`
	Description string   `json:"description" validate:"omitempty"`
	ImageURL    string   `json:"imageUrl"    validate:"omitempty"`
	VideoURL    string   `json:"videoUrl"    validate:"omitempty"`
	EntranceFee float64  `json:"entranceFee" validate:"omitempty,gt=0"`
	MinBidValue float64  `json:"minBidValue" validate:"omitempty,gt=0"`
	MaxBidValue *float64 `json:"maxBidValue" validate:"omitempty,gt=0"`
	StartAt     *string  `json:"startAt"     validate:"omitempty"`
	CloseAt     *string  `json:"closeAt"     validate:"omitempty"`
}

type PostponeProductRequest struct {
	NewCloseAt string `json:"newCloseAt" validate:"required"`
}

// ---------------------- Handlers ----------------------

// Create creates a new LUB product (admin only).
func (h *LUBProductHandler) Create(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	actorIDStr, err := GetUserIDOrUnauthorized(c)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", err)
	}
	actorID, _ := uuid.Parse(actorIDStr)

	var req CreateProductRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	p := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		VideoURL:    req.VideoURL,
		EntranceFee: domain.NewMoney(req.EntranceFee),
		MinBidValue: domain.NewMoney(req.MinBidValue),
	}
	if req.MaxBidValue != nil {
		v := domain.NewMoney(*req.MaxBidValue)
		p.MaxBidValue = &v
	}
	if req.StartAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartAt); err == nil {
			p.StartAt = &t
		}
	}
	if req.CloseAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.CloseAt); err == nil {
			p.CloseAt = &t
		}
	}

	if err := h.svc.CreateProduct(c.Context(), p, actorID); err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusCreated, "Product created", p)
}

// List returns a paginated list of all products (admin).
func (h *LUBProductHandler) List(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	page, limit, _, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(c.Query("status"))
	search := strings.TrimSpace(c.Query("search"))

	products, total, err := h.svc.ListProducts(status, search, page, limit)
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

// GetByID returns a single product.
func (h *LUBProductHandler) GetByID(c fiber.Ctx) error {
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	p, err := h.svc.GetProductByID(id)
	if err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Product found", p)
}

// Update updates mutable fields on a product.
func (h *LUBProductHandler) Update(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	var req UpdateProductRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	updates := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		VideoURL:    req.VideoURL,
		EntranceFee: domain.NewMoney(req.EntranceFee),
		MinBidValue: domain.NewMoney(req.MinBidValue),
	}
	if req.MaxBidValue != nil {
		v := domain.NewMoney(*req.MaxBidValue)
		updates.MaxBidValue = &v
	}
	if req.StartAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.StartAt); err == nil {
			updates.StartAt = &t
		}
	}
	if req.CloseAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.CloseAt); err == nil {
			updates.CloseAt = &t
		}
	}

	if err := h.svc.UpdateProduct(c.Context(), id, updates, actorID); err != nil {
		return mapLUBError(c, err)
	}
	p, _ := h.svc.GetProductByID(id)
	return response.Success(c, fiber.StatusOK, "Product updated", p)
}

// Activate transitions a product from draft → active.
func (h *LUBProductHandler) Activate(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	if err := h.svc.ActivateProduct(c.Context(), id, actorID); err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Product activated", nil)
}

// Postpone extends the CloseAt of a product.
func (h *LUBProductHandler) Postpone(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	var req PostponeProductRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	newCloseAt, parseErr := time.Parse(time.RFC3339, req.NewCloseAt)
	if parseErr != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid newCloseAt format (use RFC3339)", parseErr)
	}
	if err := h.svc.PostponeProduct(c.Context(), id, newCloseAt, actorID); err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Product postponed", nil)
}

// Close stops accepting bids for a product without executing the draw.
func (h *LUBProductHandler) Close(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	if err := h.svc.CloseProduct(c.Context(), id, actorID); err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Product closed", nil)
}

// Cancel cancels a product.
func (h *LUBProductHandler) Cancel(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	if err := h.svc.CancelProduct(c.Context(), id, actorID); err != nil {
		return mapLUBError(c, err)
	}
	return response.Success(c, fiber.StatusOK, "Product cancelled", nil)
}

// ExecuteDraw manually triggers the draw for a product.
func (h *LUBProductHandler) ExecuteDraw(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	actorIDStr, _ := GetUserIDOrUnauthorized(c)
	actorID, _ := uuid.Parse(actorIDStr)

	result, err := h.svc.ExecuteDraw(c.Context(), id, "manual", actorID)
	if err != nil {
		return mapLUBError(c, err)
	}
	if !result.HasWinner {
		return response.Success(c, fiber.StatusOK, "Draw executed: no unique bid found — all bid values were duplicated", result)
	}
	return response.Success(c, fiber.StatusOK, "Draw executed successfully", result)
}

// ListParticipants returns participants for a product (admin).
func (h *LUBProductHandler) ListParticipants(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	page, limit, _, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}
	parts, total, err := h.svc.ListParticipations(id, page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list participants", err)
	}
	return response.Success(c, fiber.StatusOK, "Participants retrieved", fiber.Map{
		"items": parts,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ListBidEntries returns all bid entries for a product (admin).
func (h *LUBProductHandler) ListBidEntries(c fiber.Ctx) error {
	if err := requireAdminRole(c); err != nil {
		return err
	}
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}
	page, limit, _, err := validate.BindPagination(c, 1, 20, 200)
	if err != nil {
		return err
	}
	entries, total, err := h.svc.ListProductBidEntries(id, page, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list bid entries", err)
	}
	return response.Success(c, fiber.StatusOK, "Bid entries retrieved", fiber.Map{
		"items": entries,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// ---------------------- Helpers ----------------------

func requireAdminRole(c fiber.Ctx) error {
	role := GetUserRole(c)
	if role != domain.RoleAdmin && role != domain.RoleSuperAdmin {
		return response.Error(c, fiber.StatusForbidden, "Admin access required", nil)
	}
	return nil
}

func mapLUBError(c fiber.Ctx, err error) error {
	status, _ := domain.MapDomainErrorToHTTP(err)
	msg := err.Error()
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.Code, appErr.Message, nil)
	}
	return response.Error(c, status, msg, err)
}
