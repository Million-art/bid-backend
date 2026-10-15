package http

import (
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ProfileHandler struct {
	userService *services.UserService
	audit       ports.AuditLogService
}

func NewProfileHandler(userService *services.UserService, audit ports.AuditLogService) *ProfileHandler {
	return &ProfileHandler{userService: userService, audit: audit}
}

// GetMyProfile returns the authenticated user's profile
func (h *ProfileHandler) GetMyProfile(c fiber.Ctx) error {
	userIDStr := GetUserID(c)
	if userIDStr == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}
	userID, _ := uuid.Parse(userIDStr)
	user, err := h.userService.GetByID(userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to get profile", err)
	}
	if user == nil {
		return response.Error(c, fiber.StatusNotFound, "User not found", nil)
	}
	return response.Success(c, fiber.StatusOK, "Profile retrieved", user)
}

type UpdateProfileRequest struct {
	FullName string `json:"fullName" validate:"omitempty,min=2,max=255"`
	Phone    string `json:"phone" validate:"omitempty,max=50"`
}

// UpdateMyProfile updates the authenticated user's profile
func (h *ProfileHandler) UpdateMyProfile(c fiber.Ctx) error {
	userIDStr := GetUserID(c)
	if userIDStr == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}
	userID, _ := uuid.Parse(userIDStr)

	var req UpdateProfileRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	user, err := h.userService.GetByID(userID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to get profile", err)
	}
	if user == nil {
		return response.Error(c, fiber.StatusNotFound, "User not found", nil)
	}

	changed := false
	if req.FullName != "" && req.FullName != user.FullName {
		user.FullName = req.FullName
		changed = true
	}
	if req.Phone != "" && req.Phone != user.Phone {
		user.Phone = req.Phone
		changed = true
	}

	if !changed {
		return response.Success(c, fiber.StatusOK, "Profile unchanged", user)
	}

	if err := h.userService.Update(user); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to update profile", err)
	}

	return response.Success(c, fiber.StatusOK, "Profile updated", user)
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

func (h *ProfileHandler) ChangeMyPassword(c fiber.Ctx) error {
	userIDStr := GetUserID(c)
	if userIDStr == "" {
		return response.Error(c, fiber.StatusUnauthorized, "Authentication required", nil)
	}
	userID, _ := uuid.Parse(userIDStr)

	var req ChangePasswordRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.userService.ChangePassword(userID, req.CurrentPassword, req.NewPassword); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Failed to change password", err)
	}

	return response.Success(c, fiber.StatusOK, "Password changed successfully", nil)
}
