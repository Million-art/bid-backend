package http

import (
	"log/slog"

	"github.com/bid-hub/admin-backend/platform/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	RequestIDKey     = "request_id"
	RequestLoggerKey = "logger"

	RequestIDHeader = "X-Request-ID"
)

func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(RequestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}
		c.Set(RequestIDHeader, id)
		c.Locals(RequestIDKey, id)
		reqLogger := slog.Default().With("request_id", id)
		c.Locals(RequestLoggerKey, reqLogger)
		ctx := logger.WithContext(c.Context(), reqLogger)
		ctx = logger.WithRequestID(ctx, id)
		c.SetContext(ctx)
		return c.Next()
	}
}

func Logger(c fiber.Ctx) *slog.Logger {
	if l, ok := c.Locals(RequestLoggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

func GetRequestID(c fiber.Ctx) string {
	if id, ok := c.Locals(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
