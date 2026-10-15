package http

import (
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services/notifications"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	service *notifications.Service
}

func NewNotificationHandler(service *notifications.Service) *NotificationHandler {
	return &NotificationHandler{service: service}
}

// notificationUserType maps an authenticated role to the audience bucket used by
// the notifications service ("participant" | "agent" | "admin").
func notificationUserType(role domain.UserRole) string {
	switch role {
	case domain.RoleAgent:
		return "agent"
	case domain.RoleAdmin, domain.RoleSuperAdmin, domain.RoleCompliance:
		return "admin"
	default:
		return "participant"
	}
}

// GetUserNotifications returns notifications for the authenticated user
func (h *NotificationHandler) GetUserNotifications(c fiber.Ctx) error {
	userIDStr, authErr := GetUserIDOrUnauthorized(c)
	if authErr != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", authErr)
	}
	userID, _ := uuid.Parse(userIDStr)
	userType := notificationUserType(GetUserRole(c))

	page, limit, offset, err := validate.BindPagination(c, 1, 20, 100)
	if err != nil {
		return err
	}

	notifications, total, err := h.service.GetUserNotifications(userID, userType, limit, offset)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch notifications", err)
	}

	return response.Success(c, fiber.StatusOK, "Notifications retrieved", fiber.Map{
		"items": notifications,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetUnreadCount
func (h *NotificationHandler) GetUnreadCount(c fiber.Ctx) error {
	userIDStr, authErr := GetUserIDOrUnauthorized(c)
	if authErr != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", authErr)
	}
	userID, _ := uuid.Parse(userIDStr)
	userType := notificationUserType(GetUserRole(c))

	count, err := h.service.GetUnreadCount(c.Context(), userID, userType)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to fetch unread count", err)
	}

	return response.Success(c, fiber.StatusOK, "Unread count retrieved", fiber.Map{"count": count})
}

// MarkAsRead
func (h *NotificationHandler) MarkAsRead(c fiber.Ctx) error {
	id, err := PathParamUUID(c, "id", "id")
	if err != nil {
		return err
	}

	if err := h.service.MarkAsRead(c.Context(), id); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to mark as read", err)
	}

	return response.Success(c, fiber.StatusOK, "Notification marked as read", nil)
}

// MarkAllAsRead
func (h *NotificationHandler) MarkAllAsRead(c fiber.Ctx) error {
	userIDStr, authErr := GetUserIDOrUnauthorized(c)
	if authErr != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", authErr)
	}
	userID, _ := uuid.Parse(userIDStr)
	userType := notificationUserType(GetUserRole(c))

	if err := h.service.MarkAllAsRead(c.Context(), userID, userType); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to mark all as read", err)
	}

	return response.Success(c, fiber.StatusOK, "All notifications marked as read", nil)
}
