// Package response is the only JSON response entry point for admin HTTP handlers.
// It re-exports platform/apiresponse so handlers do not import apiresponse directly.
package response

import (
	"github.com/bid-hub/admin-backend/platform/apiresponse"
	"github.com/gofiber/fiber/v3"
)

// Envelope is the standard API response shape.
type Envelope = apiresponse.Envelope

// ErrorMapper optionally adjusts status/code for domain-specific errors.
type ErrorMapper = apiresponse.ErrorMapper

// Success writes a success envelope.
func Success(c fiber.Ctx, code int, message string, data interface{}) error {
	return apiresponse.Success(c, code, message, data)
}

// Error writes a failure envelope.
func Error(c fiber.Ctx, code int, message string, err interface{}) error {
	return apiresponse.Error(c, code, message, err, nil)
}

// Fail writes a failure envelope without logging (expected client errors).
func Fail(c fiber.Ctx, status int, message, errCode string, data interface{}) error {
	return apiresponse.Fail(c, status, message, errCode, data)
}
