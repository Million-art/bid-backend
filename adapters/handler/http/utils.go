package http

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ParseQueryInt is deprecated; use validate.BindPagination or validate.BindLimit instead.
func ParseQueryInt(c fiber.Ctx, key string, defaultVal, min, max int) int {
	s := c.Query(key, strconv.Itoa(defaultVal))
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// GetUserID safely extracts userID from auth middleware Locals. Returns empty string if missing or invalid.
func GetUserID(c fiber.Ctx) string {
	v := c.Locals("userID")
	if v == nil {
		v = c.Locals("user_id") // Fallback
	}
	if v == nil {
		return ""
	}

	if id, ok := v.(uuid.UUID); ok {
		return id.String()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetUserIDOrUnauthorized returns userID and nil error if present, or empty string and error to return.
func GetUserIDOrUnauthorized(c fiber.Ctx) (string, error) {
	userID := GetUserID(c)
	if userID == "" {
		// Note: response package is used here
		return "", fmt.Errorf("authentication required")
	}
	return userID, nil
}

func cookieDomainAttr(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	switch domain {
	case "", "localhost", "127.0.0.1", "::1", "[::1]":
		return ""
	default:
		return domain
	}
}

// AuditContext contains info about the request origin
type AuditContext struct {
	IPAddress  string
	UserAgent  string
	ActorRole  string
	ActorEmail string
}

// GetAuditContext extracts IP and User-Agent from Fiber context
func GetAuditContext(c fiber.Ctx) AuditContext {
	ip := strings.TrimSpace(strings.Split(c.Get("X-Forwarded-For"), ",")[0])
	if ip == "" {
		ip = strings.TrimSpace(c.Get("X-Real-IP"))
	}
	if ip == "" {
		ip = c.IP()
	}

	email := fmt.Sprintf("%v", c.Locals("email"))
	if email == "<nil>" {
		email = ""
	}
	return AuditContext{
		IPAddress:  ip,
		UserAgent:  c.Get("User-Agent"),
		ActorRole:  string(GetUserRole(c)),
		ActorEmail: strings.TrimSpace(email),
	}
}

func GetUserRole(c fiber.Ctx) domain.UserRole {
	v := c.Locals("role")
	if v == nil {
		return ""
	}
	if r, ok := v.(domain.UserRole); ok {
		return r
	}
	if s, ok := v.(string); ok {
		return domain.UserRole(strings.ToLower(strings.TrimSpace(s)))
	}
	return domain.UserRole(strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v))))
}
