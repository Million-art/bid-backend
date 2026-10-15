package http

import (
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
)

type AuditHandler struct {
	repo ports.AuditLogRepository
}

func NewAuditHandler(repo ports.AuditLogRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

func (h *AuditHandler) List(c fiber.Ctx) error {
	actorRole := GetUserRole(c)
	if actorRole == "" {
		return response.Error(c, fiber.StatusForbidden, "Forbidden", nil)
	}
	// Audit logs contain sensitive platform-wide activity; only super_admin, admin, and compliance may read them.
	if actorRole != domain.RoleSuperAdmin && actorRole != domain.RoleAdmin && actorRole != domain.RoleCompliance {
		return response.Error(c, fiber.StatusForbidden, "Forbidden: Staff access required", nil)
	}

	page, limit, _, err := validate.BindPagination(c, 1, 20, 500)
	if err != nil {
		return err
	}

	offset := (page - 1) * limit
	logs, total, err := h.repo.List(nil, limit, offset)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to retrieve audit logs", err)
	}

	return response.Success(c, fiber.StatusOK, "Audit logs retrieved", fiber.Map{
		"items": logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
