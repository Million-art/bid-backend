package http

import (
	"time"

	"github.com/bid-hub/admin-backend/platform/apiresponse"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

// AuthRateLimit middleware limits auth-related endpoints (login, forgot-password, register).
func AuthRateLimit(storage fiber.Storage) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			key := c.IP()
			if key == "" {
				key = c.Get("X-Forwarded-For", "unknown")
			}
			return "limit:auth:" + key
		},
		LimitReached: func(c fiber.Ctx) error {
			return apiresponse.Fail(c, fiber.StatusTooManyRequests, "Too many requests. Please try again later.", "TOO_MANY_REQUESTS", nil)
		},
		Storage: storage,
	})
}
