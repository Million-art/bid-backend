package http

import (
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
)

// ParticipantHandler serves participant_users (client app signups) for admin dashboard.
type ParticipantHandler struct {
	repo ports.ParticipantUserRepository
}

func NewParticipantHandler(repo ports.ParticipantUserRepository) *ParticipantHandler {
	return &ParticipantHandler{repo: repo}
}

// ParticipantItem matches admin User shape for display in ParticipantManagementPage.
type ParticipantItem struct {
	ID        string `json:"id"`
	FullName  string `json:"fullName"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// List returns participants from participant_users (client-backend signups).
func (h *ParticipantHandler) List(c fiber.Ctx) error {
	role := GetUserRole(c)
	if role != domain.RoleAdmin && role != domain.RoleSuperAdmin && role != domain.RoleCompliance {
		return response.Error(c, fiber.StatusForbidden, "Forbidden: Staff access required", nil)
	}

	page, limit, offset, err := validate.BindPagination(c, 1, 10, 100)
	if err != nil {
		return err
	}
	search := strings.TrimSpace(c.Query("search"))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))

	users, total, err := h.repo.List(limit, offset, search, status)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to list participants", err)
	}

	items := make([]ParticipantItem, len(users))
	for i, u := range users {
		s := u.Status
		if s == "" {
			s = "active"
		}
		items[i] = ParticipantItem{
			ID:        u.ID,
			FullName:  u.FullName,
			Email:     u.Email,
			Phone:     u.Phone,
			Status:    s,
			Role:      "participant",
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		}
	}

	return response.Success(c, fiber.StatusOK, "Participants retrieved", fiber.Map{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// UpdateStatus sets participant status (active | suspended).
func (h *ParticipantHandler) UpdateStatus(c fiber.Ctx) error {
	role := GetUserRole(c)
	if role != domain.RoleAdmin && role != domain.RoleSuperAdmin && role != domain.RoleCompliance {
		return response.Error(c, fiber.StatusForbidden, "Forbidden: Admin access required", nil)
	}

	id, err := validate.ParamString(c, "id", "id")
	if err != nil {
		return err
	}
	var req struct {
		Status string `json:"status" validate:"required,oneof=active suspended ACTIVE SUSPENDED Active Suspended"`
	}
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	if _, err := h.repo.GetByID(id); err != nil {
		return response.Error(c, fiber.StatusNotFound, "Participant not found", nil)
	}
	if err := h.repo.UpdateStatus(id, status); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update participant status", err)
	}
	return response.Success(c, fiber.StatusOK, "Participant status updated", nil)
}

// Delete permanently removes a participant (soft delete).
func (h *ParticipantHandler) Delete(c fiber.Ctx) error {
	role := GetUserRole(c)
	if role != domain.RoleAdmin && role != domain.RoleSuperAdmin {
		return response.Error(c, fiber.StatusForbidden, "Forbidden: Admin access required", nil)
	}

	id, err := validate.ParamString(c, "id", "id")
	if err != nil {
		return err
	}
	if _, err := h.repo.GetByID(id); err != nil {
		return response.Error(c, fiber.StatusNotFound, "Participant not found", nil)
	}
	if err := h.repo.Delete(id); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to delete participant", err)
	}
	return response.Success(c, fiber.StatusOK, "Participant deleted successfully", nil)
}
