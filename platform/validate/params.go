package validate

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// PaginationQuery is the standard page/limit query pair.
type PaginationQuery struct {
	Page  int `query:"page" validate:"omitempty,gte=1"`
	Limit int `query:"limit" validate:"omitempty,gte=1,lte=500"`
}

// BindPagination binds and validates page/limit query params with defaults.
func BindPagination(c fiber.Ctx, defaultPage, defaultLimit, maxLimit int) (page, limit, offset int, err error) {
	var q PaginationQuery
	if err := BindQuery(c, &q); err != nil {
		return 0, 0, 0, err
	}
	page = q.Page
	if page < 1 {
		page = defaultPage
	}
	limit = q.Limit
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset = (page - 1) * limit
	return page, limit, offset, nil
}

// ParamUUID parses a path param as UUID using the standard validation envelope.
func ParamUUID(c fiber.Ctx, paramName, fieldName string) (uuid.UUID, error) {
	s := strings.TrimSpace(c.Params(paramName))
	if s == "" {
		_ = Fail(c, "Invalid request", FieldError{
			Field: fieldName, Rule: "required", Message: fieldName + " is required",
		})
		return uuid.Nil, ErrParamInvalid
	}
	id, err := uuid.Parse(s)
	if err != nil {
		_ = Fail(c, "Invalid request", FieldError{
			Field: fieldName, Rule: "uuid", Message: fieldName + " must be a valid UUID",
		})
		return uuid.Nil, ErrParamInvalid
	}
	return id, nil
}

// LimitQuery is a single limit query param (no page).
type LimitQuery struct {
	Limit int `query:"limit" validate:"omitempty,gte=1,lte=500"`
}

// BindLimit binds limit with defaults and max cap.
func BindLimit(c fiber.Ctx, defaultLimit, maxLimit int) (limit int, err error) {
	var q LimitQuery
	if err := BindQuery(c, &q); err != nil {
		return 0, err
	}
	limit = q.Limit
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return limit, nil
}

// OffsetPaginationQuery is limit/offset style pagination (client notifications).
type OffsetPaginationQuery struct {
	Limit  int `query:"limit" validate:"omitempty,gte=1,lte=500"`
	Offset int `query:"offset" validate:"omitempty,gte=0,lte=100000"`
}

// BindOffsetPagination binds limit/offset with defaults.
func BindOffsetPagination(c fiber.Ctx, defaultLimit, maxLimit int) (limit, offset int, err error) {
	var q OffsetPaginationQuery
	if err := BindQuery(c, &q); err != nil {
		return 0, 0, err
	}
	limit = q.Limit
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset = q.Offset
	if offset < 0 {
		offset = 0
	}
	return limit, offset, nil
}

// ParamString parses a required path param as a non-empty string.
func ParamString(c fiber.Ctx, paramName, fieldName string) (string, error) {
	s := strings.TrimSpace(c.Params(paramName))
	if s == "" {
		_ = Fail(c, "Invalid request", FieldError{
			Field: fieldName, Rule: "required", Message: fieldName + " is required",
		})
		return "", ErrParamInvalid
	}
	return s, nil
}

// ErrParamInvalid signals ParamUUID already wrote the response.
var ErrParamInvalid = errParamInvalid{}

type errParamInvalid struct{}

func (errParamInvalid) Error() string { return "invalid path parameter" }

// IsParamInvalid reports whether err means the handler should stop (response written).
func IsParamInvalid(err error) bool {
	return errors.Is(err, ErrParamInvalid) || errors.Is(err, ErrValidation)
}
