package domain

import (
	"errors"
	"strings"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrInternalServer      = errors.New("internal server error")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrConflict            = errors.New("resource already exists")
	ErrEmailConflict       = NewAppError(409, "በዚህ ኢሜይል አድራሻ የተመዘገበ ተጠቃሚ አለ", ErrConflict)
	ErrPhoneConflict       = NewAppError(409, "በዚህ ስልክ ቁጥር የተመዘገበ ተጠቃሚ አለ", ErrConflict)
	ErrInsufficientTickets = errors.New("not enough tickets available")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

// Unwrap exposes the underlying cause (e.g. Postgres duplicate key) for errors.Is/As
// and for FormatErrorDetail when persisting payment ledger last_error.
func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) HTTPStatus() int {
	return e.Code
}

func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// MapDomainErrorToHTTP turns a domain error into a suitable HTTP status code
func MapDomainErrorToHTTP(err error) (int, string) {
	if err == nil {
		return 200, ""
	}

	// Check if it's already an AppError
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code, MapHTTPToErrorCode(appErr.Code)
	}

	// Map sentinel errors. Every leaf error in the codebase must wrap one of
	// these (via fmt.Errorf("...: %w", domain.ErrX)) so the mapping is exact;
	// substring matching on err.Error() was removed because it produced false
	// positives on legitimate user-facing messages that contained "not found".
	switch {
	case errors.Is(err, ErrNotFound):
		return 404, "NOT_FOUND"
	case errors.Is(err, ErrUnauthorized):
		return 401, "UNAUTHORIZED"
	case errors.Is(err, ErrForbidden):
		return 403, "FORBIDDEN"
	case errors.Is(err, ErrConflict):
		return 409, "CONFLICT"
	case errors.Is(err, ErrInsufficientTickets):
		return 400, "INSUFFICIENT_TICKETS"
	case errors.Is(err, ErrInsufficientBalance):
		return 400, "INSUFFICIENT_BALANCE"
	case errors.Is(err, ErrInvalidInput):
		return 400, "INVALID_INPUT"
	default:
		return 500, "INTERNAL_ERROR"
	}
}

// FormatErrorDetail builds an ops-friendly message with the full error chain.
// Use for payment ledger last_error  never rely on bare sentinel Error() strings alone.
func FormatErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	seen := make(map[string]struct{})
	var parts []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		parts = append(parts, s)
	}
	for cur := err; cur != nil; {
		var app *AppError
		if errors.As(cur, &app) {
			add(app.Message)
			cur = app.Err
			continue
		}
		add(cur.Error())
		cur = errors.Unwrap(cur)
	}
	return truncateErrorDetail(strings.Join(parts, " | "))
}

// MaxPaymentLedgerErrorLen is the cap for payments.last_error (TEXT column).
const MaxPaymentLedgerErrorLen = 8000

// IsRetryableFulfillmentError returns true when the error is transient (e.g. DB/network)
// and automatic payment retry should keep trying. Business rule failures (400, 404, 409, 422)
// are not retryable.
func IsRetryableFulfillmentError(err error) bool {
	if err == nil {
		return false
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case 400, 401, 403, 404, 409, 422:
			return false
		}
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrForbidden) {
		return false
	}
	return true
}

func truncateErrorDetail(s string) string {
	if len(s) <= MaxPaymentLedgerErrorLen {
		return s
	}
	return s[:MaxPaymentLedgerErrorLen]
}

func MapHTTPToErrorCode(code int) string {
	switch code {
	case 400:
		return "BAD_REQUEST"
	case 401:
		return "UNAUTHORIZED"
	case 403:
		return "FORBIDDEN"
	case 404:
		return "NOT_FOUND"
	case 405:
		return "METHOD_NOT_ALLOWED"
	case 409:
		return "CONFLICT"
	case 422:
		return "UNPROCESSABLE_ENTITY"
	case 429:
		return "TOO_MANY_REQUESTS"
	default:
		return "INTERNAL_ERROR"
	}
}
