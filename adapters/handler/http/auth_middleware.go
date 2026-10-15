package http

import (
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/gofiber/fiber/v3"
)

// ClientSecretMiddleware authenticates internal service-to-service requests
func ClientSecretMiddleware(clientSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if clientSecret == "" {
			Logger(c).Warn("CLIENT_SECRET is not configured  rejecting request")
			return response.Error(c, fiber.StatusServiceUnavailable, "Service not configured", nil)
		}
		provided := c.Get("X-Client-Secret")
		// Use constant-time comparison to prevent timing attacks.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(clientSecret)) != 1 {
			return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", nil)
		}
		return c.Next()
	}
}

func AuthMiddleware(authService *services.AuthService, secret string) fiber.Handler {
	return AuthMiddlewareWithCache(authService, secret, nil)
}

// AuthMiddlewareWithCache is the full middleware that also checks the token revocation list.
func AuthMiddlewareWithCache(authService *services.AuthService, secret string, cache ports.CacheRepository) fiber.Handler {
	return func(c fiber.Ctx) error {
		var token string

		// 1. Try cookie first
		token = c.Cookies("access_token")

		// 2. Fallback to Authorization header
		if token == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			Logger(c).Info("Missing authorization token")
			return response.Error(c, fiber.StatusUnauthorized, "Missing authorization token", nil)
		}

		claims, err := authService.VerifyToken(token, secret)
		if err != nil {
			Logger(c).Error("Token verification failed", "error", err)
			return response.Error(c, fiber.StatusUnauthorized, "Invalid or expired token", nil)
		}

		// Verify purpose
		purpose, _ := claims["purpose"].(string)
		if purpose != "access" {
			Logger(c).Error("Invalid token purpose", "purpose", purpose)
			return response.Error(c, fiber.StatusUnauthorized, "Invalid token purpose", nil)
		}

		// Check revocation list  reject tokens that have been explicitly revoked (e.g. after logout).
		if cache != nil {
			if jti, ok := claims["jti"].(string); ok && jti != "" {
				revoked, err := cache.IsTokenRevoked(c.Context(), jti)
				if err != nil {
					Logger(c).Warn("Redis revocation check error; continuing with valid token", "error", err)
				} else if revoked {
					return response.Error(c, fiber.StatusUnauthorized, "Token has been revoked", nil)
				}
			}
		}

		// Strictly cast role to domain.UserRole (which is lowercase)
		rawRole := claims["role"]
		var finalRole string
		if r, ok := rawRole.(string); ok {
			finalRole = strings.ToLower(r)
		} else {
			finalRole = fmt.Sprintf("%v", rawRole)
		}

		// Set claims in locals
		c.Locals("userID", claims["sub"])
		c.Locals("role", domain.UserRole(finalRole))
		c.Locals("email", claims["email"])

		return c.Next()
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but doesn't return 401 if token is missing or invalid.
// It still sets userID and role if valid token is found.
func OptionalAuthMiddleware(authService *services.AuthService, secret string, cache ports.CacheRepository) fiber.Handler {
	return func(c fiber.Ctx) error {
		var token string

		// 1. Try cookie first
		token = c.Cookies("access_token")

		// 2. Fallback to Authorization header
		if token == "" {
			authHeader := c.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			return c.Next()
		}

		claims, err := authService.VerifyToken(token, secret)
		if err != nil {
			return c.Next()
		}

		// Verify purpose
		purpose, _ := claims["purpose"].(string)
		if purpose != "access" {
			return c.Next()
		}

		// Check revocation
		if cache != nil {
			if jti, ok := claims["jti"].(string); ok && jti != "" {
				revoked, err := cache.IsTokenRevoked(c.Context(), jti)
				if err != nil || revoked {
					if err != nil {
						Logger(c).Warn("Optional auth revocation check failed; continuing unauthenticated", "error", err)
					}
					return c.Next()
				}
			}
		}

		// Strictly cast role to domain.UserRole (which is lowercase)
		rawRole := claims["role"]
		var finalRole string
		if r, ok := rawRole.(string); ok {
			finalRole = strings.ToLower(r)
		} else {
			finalRole = fmt.Sprintf("%v", rawRole)
		}

		// Set claims in locals
		c.Locals("userID", claims["sub"])
		c.Locals("role", domain.UserRole(finalRole))
		c.Locals("email", claims["email"])

		return c.Next()
	}
}
