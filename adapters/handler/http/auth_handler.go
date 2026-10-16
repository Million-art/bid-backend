package http

import (
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
)

type AuthHandler struct {
	authService *services.AuthService
	config      *env.Config
}

func NewAuthHandler(authService *services.AuthService, config *env.Config) *AuthHandler {
	return &AuthHandler{authService: authService, config: config}
}

type LoginRequest struct {
	LoginIdentifier string `json:"loginIdentifier"`
	Email           string `json:"email"`
	Password        string `json:"password" validate:"required,min=1"`
}

type ForgotPasswordRequest struct {
	Identifier string `json:"identifier"`
	Email      string `json:"email"`
}

type ResetPasswordOTPRequest struct {
	Identifier  string `json:"identifier"  validate:"required"`
	OTP         string `json:"otp"         validate:"required,min=6,max=6"`
	NewPassword string `json:"newPassword" validate:"required,min=8"`
}

// @Summary Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login"
// @Success 200 {object} map[string]interface{}
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	identifier := strings.TrimSpace(req.LoginIdentifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		return response.Error(c, fiber.StatusBadRequest, "Email or phone number is required", nil)
	}

	Logger(c).Info("Attempting login", "identifier", identifier)
	auditCtx := domain.AuditContext(GetAuditContext(c))
	accessToken, refreshToken, user, err := h.authService.Login(c.Context(), identifier, req.Password, &auditCtx)
	if err != nil {
		Logger(c).Error("Login service error", "identifier", identifier, "error", err)
		if errors.Is(err, domain.ErrUnauthorized) {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid credentials", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "Login failed", err)
	}

	h.setRefreshTokenCookie(c, refreshToken)
	h.setAccessTokenCookie(c, accessToken)

	csrfToken, err := IssueCSRF(c, h.config, JTIFromToken(accessToken))
	if err != nil {
		Logger(c).Error("Failed to issue CSRF token on login", "error", err)
		return response.Error(c, fiber.StatusInternalServerError, "Login failed", err)
	}

	return response.Success(c, fiber.StatusOK, "Login successful", fiber.Map{
		"user":      user,
		"csrfToken": csrfToken,
	})
}

// @Summary Refresh token
// @Tags Auth
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		Logger(c).Warn("Refresh failed: refresh_token cookie missing")
		return response.Error(c, fiber.StatusUnauthorized, "Refresh token required", nil)
	}

	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(c.Context(), refreshToken)
	if err != nil {
		Logger(c).Error("Refresh failed for token", "error", err)
		return response.Error(c, fiber.StatusUnauthorized, "Invalid refresh token", err)
	}

	h.setRefreshTokenCookie(c, newRefreshToken)
	h.setAccessTokenCookie(c, newAccessToken)

	csrfToken, err := IssueCSRF(c, h.config, JTIFromToken(newAccessToken))
	if err != nil {
		Logger(c).Error("Failed to issue CSRF token on refresh", "error", err)
		return response.Error(c, fiber.StatusInternalServerError, "Refresh failed", err)
	}

	return response.Success(c, fiber.StatusOK, "Token refreshed", fiber.Map{
		"csrfToken": csrfToken,
	})
}

// @Summary Logout
// @Tags Auth
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	// Revoke both tokens server-side so they cannot be replayed even if an attacker
	// captured them before the cookies were cleared.
	if accessToken := c.Cookies("access_token"); accessToken != "" {
		h.authService.RevokeTokenByString(c.Context(), accessToken, h.config.JWTAccessSecret, 24*time.Hour)
	}
	if refreshToken := c.Cookies("refresh_token"); refreshToken != "" {
		h.authService.RevokeTokenByString(c.Context(), refreshToken, h.config.JWTRefreshSecret, 30*24*time.Hour)
	}

	h.clearAuthCookies(c)
	ClearCSRF(c, h.config)
	return response.Success(c, fiber.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) clearAuthCookies(c fiber.Ctx) {
	for _, name := range []string{"access_token", "refresh_token"} {
		c.Cookie(ClearSessionCookie(name, h.config))
	}
}

// @Summary Forgot password
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body ForgotPasswordRequest true "Forgot"
// @Router /api/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c fiber.Ctx) error {
	var req ForgotPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "እባክዎ ትክክለኛ መረጃ ያስገቡ", err)
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		return response.Error(c, fiber.StatusBadRequest, "እባክዎ ኢሜይል ወይም የስልክ ቁጥር ያስገቡ", nil)
	}

	method, err := h.authService.ForgotPassword(c.Context(), identifier)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "የይለፍ ቃል መቀየሪያ ጥያቄን ማስተናገድ አልተቻለም", err)
	}

	return response.Success(c, fiber.StatusOK, "የይለፍ ቃል መቀየሪያ ጥያቄ ተልኳል", fiber.Map{
		"method": method,
	})
}

type SetPasswordRequest struct {
	Token    string `json:"token" validate:"required"`
	Password string `json:"password" validate:"required,min=4"`
}

// @Summary Set password
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body SetPasswordRequest true "SetPassword"
// @Router /api/auth/set-password [post]
func (h *AuthHandler) SetPassword(c fiber.Ctx) error {
	var req SetPasswordRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	auditCtx := domain.AuditContext(GetAuditContext(c))
	if err := h.authService.SetPassword(c.Context(), req.Token, req.Password, &auditCtx); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired token", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "Failed to set password", err)
	}

	return response.Success(c, fiber.StatusOK, "Password set successfully", nil)
}

func (h *AuthHandler) setRefreshTokenCookie(c fiber.Ctx, token string) {
	c.Cookie(SessionCookie("refresh_token", token, 30*24*time.Hour, h.config))
}

func (h *AuthHandler) setAccessTokenCookie(c fiber.Ctx, token string) {
	c.Cookie(SessionCookie("access_token", token, 24*time.Hour, h.config))
}

// @Summary Reset password OTP
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body ResetPasswordOTPRequest true "ResetOTP"
// @Router /api/auth/reset-password-otp [post]
// ResetPasswordOTP handles phone-based password reset by verifying an OTP then updating the password.
func (h *AuthHandler) ResetPasswordOTP(c fiber.Ctx) error {
	var req ResetPasswordOTPRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.authService.ResetPasswordWithOTP(c.Context(), req.Identifier, req.OTP, req.NewPassword); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrInvalidInput) {
			return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired OTP", nil)
		}
		if err.Error() == "invalid OTP" || err.Error() == "OTP expired or not found" {
			return response.Error(c, fiber.StatusUnauthorized, err.Error(), nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "Password reset failed", err)
	}

	return response.Success(c, fiber.StatusOK, "Password updated successfully", nil)
}
