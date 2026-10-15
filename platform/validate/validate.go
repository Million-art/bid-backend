// Package validate centralises request DTO validation for both Bind+validator
// invocations and ad-hoc struct checks. Every handler that takes a JSON body
// should go through BindAndValidate so error envelopes stay consistent and
// hand-rolled `if req.X == "" { ... }` chains disappear.
//
// Conventions:
//   - Struct tags follow https://pkg.go.dev/github.com/go-playground/validator/v10
//   - Field names in error payloads use the JSON tag so clients can map
//     errors back onto the wire shape they sent.
//   - Custom registered tags:
//     money_amount   string that parses as a positive decimal (>= 0.01)
//     e164           phone in E.164-ish form ("+" + 8..15 digits)
//   - To attach the validator to a Fiber handler, call:
//     if err := validate.BindAndValidate(c, &req); err != nil { return err }
//
// The package returns Fiber responses directly using the project's standard
// envelope, so handlers do not need to translate errors themselves.
package validate

import (
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"

	"github.com/bid-hub/admin-backend/platform/apiresponse"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

// FieldError is the per-field item we surface to clients in the response
// envelope's Data slot.
//
//	{
//	  "success": false,
//	  "message": "Invalid request",
//	  "error":   "BAD_REQUEST",
//	  "data": { "fields": [ {"field":"email","rule":"required","message":"..."} ] }
//	}
type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

// ValidationPayload is what we stuff into GenericResponse.Data on a 400.
type ValidationPayload struct {
	Fields []FieldError `json:"fields"`
}

var (
	once sync.Once
	v    *validator.Validate
)

// Get returns the process-wide validator. Cached because validator.New() is
// non-trivial (it warms up the tag cache).
func Get() *validator.Validate {
	once.Do(func() {
		v = validator.New(validator.WithRequiredStructEnabled())

		// Use json tag names in error output so the wire field name matches
		// what the caller actually sent.
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			for _, key := range []string{"json", "query"} {
				tag := strings.SplitN(fld.Tag.Get(key), ",", 2)[0]
				if tag != "" && tag != "-" {
					return tag
				}
			}
			return fld.Name
		})

		registerCustomValidators(v)
	})
	return v
}

// Struct validates `dst` against its tags and returns nil or a fully populated
// 400 Fiber response. Use this when you have already bound the request body
// yourself (rare  prefer BindAndValidate).
func Struct(c fiber.Ctx, dst interface{}) error {
	if err := Get().Struct(dst); err != nil {
		return respondValidation(c, err)
	}
	return nil
}

// BindAndValidate calls c.Bind().Body(dst), then validates the populated
// struct. On any failure it writes the standard envelope and returns the
// resulting error so handlers can simply `return validate.BindAndValidate(c, &req)`.
//
// Bind failures and validation failures both surface as 400 BAD_REQUEST with
// the standard envelope; bind failures carry a single synthetic FieldError
// with rule="bind" so clients can still react programmatically.
func BindAndValidate(c fiber.Ctx, dst interface{}) error {
	if err := c.Bind().Body(dst); err != nil {
		return writeEnvelope(c, "Invalid request body", []FieldError{{Field: "body", Rule: "bind", Message: err.Error()}})
	}
	if err := Get().Struct(dst); err != nil {
		return respondValidation(c, err)
	}
	return nil
}

// respondValidation converts a validator.ValidationErrors into the standard
// envelope and writes it.
func respondValidation(c fiber.Ctx, err error) error {
	return writeEnvelope(c, "Invalid request", formatFieldErrors(err))
}

// writeEnvelope emits the project-standard {success,message,error,data} shape
// for a validation failure. Keeps the success path and ad-hoc error paths
// using the same JSON contract so consumers only have to handle one shape.
func writeEnvelope(c fiber.Ctx, message string, fields []FieldError) error {
	return FailWithStatus(c, fiber.StatusBadRequest, message, "BAD_REQUEST", fields...)
}

// ErrValidation means the validation envelope was already written; handlers should return nil.
var ErrValidation = errValidation{}

type errValidation struct{}

func (errValidation) Error() string { return "validation failed" }

// IsValidation reports whether err means the response was already written.
func IsValidation(err error) bool {
	return errors.Is(err, ErrValidation) || errors.Is(err, ErrParamInvalid)
}

// FailWithStatus writes the standard envelope with an explicit HTTP status and
// error code (e.g. 401 UNAUTHORIZED for invalid verification tokens).
func FailWithStatus(c fiber.Ctx, status int, message, errCode string, fields ...FieldError) error {
	var data interface{}
	if len(fields) > 0 {
		data = ValidationPayload{Fields: fields}

		// Automatically log all validation failures with their detailed fields
		slog.Warn("Validation failed", "status", status, "message", message, "path", c.Path(), "fields", fields)
	}
	_ = apiresponse.Fail(c, status, message, errCode, data)
	return ErrValidation
}

// formatFieldErrors flattens validator's error list into our wire shape.
// Unknown / non-validator errors come back as a single body-level entry so we
// never panic on an unexpected error type.
func formatFieldErrors(err error) []FieldError {
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return []FieldError{{Field: "body", Rule: "invalid", Message: err.Error()}}
	}

	out := make([]FieldError, 0, len(verrs))
	for _, fe := range verrs {
		out = append(out, FieldError{
			Field:   fe.Field(),
			Rule:    fe.Tag(),
			Param:   fe.Param(),
			Message: humanMessage(fe),
		})
	}
	return out
}

// humanMessage produces an operator-friendly summary for the common tags so
// callers don't have to translate "len" into "must be N characters".
func humanMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email"
	case "min":
		return fe.Field() + " must be at least " + fe.Param()
	case "max":
		return fe.Field() + " must be at most " + fe.Param()
	case "len":
		return fe.Field() + " must be exactly " + fe.Param() + " characters"
	case "gt":
		return fe.Field() + " must be greater than " + fe.Param()
	case "gte":
		return fe.Field() + " must be >= " + fe.Param()
	case "lt":
		return fe.Field() + " must be less than " + fe.Param()
	case "lte":
		return fe.Field() + " must be <= " + fe.Param()
	case "oneof":
		return fe.Field() + " must be one of: " + fe.Param()
	case "uuid", "uuid4":
		return fe.Field() + " must be a valid UUID"
	case "url":
		return fe.Field() + " must be a valid URL"
	case "e164":
		return fe.Field() + " must be a valid phone number (digits, optional leading +)"
	case "strong_password":
		return fe.Field() + " must be at least 8 characters with one uppercase letter, one lowercase letter, and one number"
	case "money_amount":
		return fe.Field() + " must be a valid amount of at least 0.01"
	case "eqfield":
		return fe.Field() + " must match " + fe.Param()
	case "nefield":
		return fe.Field() + " must not match " + fe.Param()
	case "boolean":
		return fe.Field() + " must be a boolean"
	case "eq":
		return fe.Field() + " must equal " + fe.Param()
	case "ne":
		return fe.Field() + " must not equal " + fe.Param()
	case "datetime":
		return fe.Field() + " must be a valid datetime (format: " + fe.Param() + ")"
	case "dive":
		return fe.Field() + " contains invalid items"
	default:
		return fe.Field() + " failed " + fe.Tag() + " check"
	}
}

// Fail writes a 400 envelope with caller-supplied field errors. Handlers can
// use this for semantic checks that can't be expressed as struct tags
// (e.g. "password must match confirmation", "exactly one of A/B required").
func Fail(c fiber.Ctx, message string, fields ...FieldError) error {
	return writeEnvelope(c, message, fields)
}
