// Package apiresponse is the single JSON envelope for every HTTP API in the
// monorepo (admin-backend, client-backend, validate, and middleware).
package apiresponse

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/platform/logger"
	"github.com/gofiber/fiber/v3"
)

// Envelope is the standard API response shape.
type Envelope struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorMapper optionally adjusts status/code for domain-specific errors.
// Return ok=false to fall through to domain.MapDomainErrorToHTTP.
type ErrorMapper func(err error) (status int, code string, ok bool)

// Success writes a success envelope.
func Success(c fiber.Ctx, code int, message string, data interface{}) error {
	return c.Status(code).JSON(Envelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error writes a failure envelope and logs when err is non-nil.
func Error(c fiber.Ctx, code int, message string, err interface{}, mapper ErrorMapper) error {
	errCode := domain.MapHTTPToErrorCode(code)

	if e, ok := err.(error); ok && e != nil {
		mapped := false
		if mapper != nil {
			if mappedStatus, mappedCode, ok := mapper(e); ok {
				code = mappedStatus
				errCode = mappedCode
				mapped = true
			}
		}
		if !mapped {
			if mappedStatus, mappedCode := domain.MapDomainErrorToHTTP(e); mappedCode != "INTERNAL_ERROR" {
				code = mappedStatus
				errCode = mappedCode
			}
		}
	} else if customCode, ok := err.(string); ok && customCode != "" {
		errCode = customCode
	}

	if err != nil {
		logger.FromContext(c.Context()).Error(message,
			"error", err,
			"code", errCode,
			"status", code,
			"method", string(c.Method()),
			"path", string(c.Path()),
		)
	}

	return c.Status(code).JSON(Envelope{
		Success: false,
		Message: message,
		Error:   errCode,
	})
}

// Fail writes a failure envelope with explicit status, code, and optional data
// (e.g. validation field list). Does not log  validation/expected client errors.
func Fail(c fiber.Ctx, status int, message, errCode string, data interface{}) error {
	return c.Status(status).JSON(Envelope{
		Success: false,
		Message: message,
		Error:   errCode,
		Data:    data,
	})
}
