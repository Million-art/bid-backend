package http

import (
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
)

type DashboardHandler struct {
	svc *services.DashboardService
}

func NewDashboardHandler(svc *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

func (h *DashboardHandler) requireStaff(c fiber.Ctx) bool {
	role := GetUserRole(c)
	if role != domain.RoleSuperAdmin && role != domain.RoleAdmin && role != domain.RoleCompliance {
		response.Error(c, fiber.StatusForbidden, "Forbidden: Staff access required", nil)
		return false
	}
	return true
}

func (h *DashboardHandler) GetStats(c fiber.Ctx) error {
	if !h.requireStaff(c) {
		return nil
	}
	stats, err := h.svc.GetStats()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch dashboard stats", err)
	}
	return response.Success(c, fiber.StatusOK, "Dashboard stats retrieved", stats)
}

func (h *DashboardHandler) GetCharts(c fiber.Ctx) error {
	if !h.requireStaff(c) {
		return nil
	}
	revChart, err := h.svc.GetPlatformRevenueChart()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch revenue chart", err)
	}
	userChart, err := h.svc.GetUserGrowthChart()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch user growth chart", err)
	}
	partChart, err := h.svc.GetParticipationsChart()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch participations chart", err)
	}
	bidsChart, err := h.svc.GetBidsChart()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch bids chart", err)
	}
	payChart, err := h.svc.GetPaymentsChart()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch payments chart", err)
	}
	return response.Success(c, fiber.StatusOK, "Dashboard charts retrieved", fiber.Map{
		"revenue":        revChart,
		"userGrowth":     userChart,
		"participations": partChart,
		"bids":           bidsChart,
		"payments":       payChart,
	})
}

func (h *DashboardHandler) ListAuditLogs(c fiber.Ctx) error {
	if !h.requireStaff(c) {
		return nil
	}
	page, limit, _, err := validate.BindPagination(c, 1, 20, 500)
	if err != nil {
		return err
	}
	logs, total, err := h.svc.ListAuditLogs(nil, page, limit)
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
