package http

import (
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// PathParamUUID parses a path UUID; on failure the standard validation envelope is written
// and ErrParamInvalid is returned  handlers should `if err != nil { return nil }`.
func PathParamUUID(c fiber.Ctx, paramName, fieldName string) (uuid.UUID, error) {
	return validate.ParamUUID(c, paramName, fieldName)
}
