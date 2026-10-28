package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/platform/apiresponse"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// CSRFCookieName holds the per-browser CSRF session id (sid). The actual signed token is
	// returned to JS via the JSON body and submitted back in the X-CSRF-Token header.
	CSRFCookieName = "csrf_token"
	CSRFHeaderName = "X-CSRF-Token"

	csrfSidByteLength = 32
	csrfSidCookieTTL  = 30 * 24 * time.Hour
	csrfTokenLifetime = 12 * time.Hour
)

// ErrCSRFSecretMissing indicates CSRF_SECRET was not configured at runtime.
var ErrCSRFSecretMissing = errors.New("CSRF_SECRET is not configured")

// csrfPayload is the data block HMAC-signed inside the X-CSRF-Token header.
// sid binds the token to a specific browser; jti binds it to a specific JWT session
// (empty string means the token was minted for an unauthenticated visitor).
type csrfPayload struct {
	Sid string `json:"sid"`
	Jti string `json:"jti"`
	Exp int64  `json:"exp"`
}

func isExempt(path string) bool {
	if strings.HasPrefix(path, "/api/public/auctions/") && strings.HasSuffix(path, "/participate") {
		return true
	}
	if path == "/api/public/bids" {
		return true
	}
	if strings.HasPrefix(path, "/api/public/bids/") {
		return true
	}
	if strings.HasPrefix(path, "/api/webhooks/chapa") {
		return true
	}
	if strings.HasPrefix(path, "/api/webhooks/telegram") {
		return true
	}
	if strings.HasPrefix(path, "/api/public/payments/verify") {
		return true
	}
	return false
}

// CSRF validates the signed X-CSRF-Token header for state-changing requests.
// A request is accepted only when all of the following hold:
//  1. The HMAC signature on the token matches CSRF_SECRET.
//  2. payload.exp is in the future.
//  3. payload.sid equals the csrf_token cookie (when cookie is present on non-auth routes).
//  4. payload.jti equals the access_token cookie's jti (when both are present on non-auth routes).
func CSRF(cfg *env.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		method := string(c.Method())
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}
		path := string(c.Path())
		if isExempt(path) {
			return c.Next()
		}

		header := c.Get(CSRFHeaderName)
		if header == "" {
			return csrfDenied(c)
		}
		payload, ok := verifyCSRFToken(header, cfg.CSRFSecret)
		if !ok {
			return csrfDenied(c)
		}
		if payload.Exp <= time.Now().Unix() {
			return csrfDenied(c)
		}
		if !strings.HasPrefix(path, "/api/auth/") {
			sid := c.Cookies(CSRFCookieName)
			if sid != "" && payload.Sid != "" && !constantTimeEqual(payload.Sid, sid) {
				return csrfDenied(c)
			}
			accessJti := jtiFromAccessCookie(c)
			if accessJti != "" && payload.Jti != "" && !constantTimeEqual(payload.Jti, accessJti) {
				return csrfDenied(c)
			}
		}
		return c.Next()
	}
}

func csrfDenied(c fiber.Ctx) error {
	return apiresponse.Fail(c, fiber.StatusForbidden, "Invalid or missing CSRF token", "CSRF_INVALID", nil)
}

func constantTimeEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// jtiFromAccessCookie extracts the `jti` claim from the access_token cookie without
// verifying the signature. The CSRF layer doesn't trust this value  it only compares it
// to what was signed into the CSRF token. The auth middleware verifies the JWT downstream.
func jtiFromAccessCookie(c fiber.Ctx) string {
	return JTIFromToken(c.Cookies("access_token"))
}

// JTIFromToken returns the `jti` claim from a JWT without verifying its signature.
// Returns "" if the token is empty or malformed. Useful at issue time to bind a CSRF
// token to a JWT we just minted.
func JTIFromToken(raw string) string {
	if raw == "" {
		return ""
	}
	tok, _, err := jwt.NewParser().ParseUnverified(raw, jwt.MapClaims{})
	if err != nil {
		return ""
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}
	if jti, ok := claims["jti"].(string); ok {
		return jti
	}
	return ""
}

// IssueCSRF mints a fresh signed token bound to (sid, jti). It also ensures the sid cookie
// is set (creating a new sid if absent). Pass jti == "" for unauthenticated callers.
func IssueCSRF(c fiber.Ctx, cfg *env.Config, jti string) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.CSRFSecret) == "" {
		return "", ErrCSRFSecretMissing
	}

	sid := c.Cookies(CSRFCookieName)
	if sid == "" {
		b := make([]byte, csrfSidByteLength)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		sid = base64.RawURLEncoding.EncodeToString(b)
	}
	setSidCookie(c, cfg, sid)

	payload := csrfPayload{
		Sid: sid,
		Jti: jti,
		Exp: time.Now().Add(csrfTokenLifetime).Unix(),
	}
	return signCSRFToken(payload, cfg.CSRFSecret)
}

// ClearCSRF erases the sid cookie so a fresh sid is minted on the user's next visit.
// Call this on logout to invalidate any in-memory CSRF tokens the client may still hold.
func ClearCSRF(c fiber.Ctx, cfg *env.Config) {
	c.Cookie(ClearSessionCookie(CSRFCookieName, cfg))
}

func setSidCookie(c fiber.Ctx, cfg *env.Config, sid string) {
	c.Cookie(SessionCookie(CSRFCookieName, sid, csrfSidCookieTTL, cfg))
}

func sameSiteAttr(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return "Strict"
	case "none":
		return "None"
	default:
		return "Lax"
	}
}

func signCSRFToken(p csrfPayload, secret string) (string, error) {
	j, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(j)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadB64 + "." + sigB64, nil
}

func verifyCSRFToken(token, secret string) (csrfPayload, bool) {
	var zero csrfPayload
	if secret == "" {
		return zero, false
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return zero, false
	}
	payloadB64, sigB64 := parts[0], parts[1]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return zero, false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadB64))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return zero, false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return zero, false
	}
	var p csrfPayload
	if err := json.Unmarshal(payloadBytes, &p); err != nil {
		return zero, false
	}
	return p, true
}

// HandleGetCSRFToken issues a fresh signed CSRF token. When the request carries a
// (possibly expired) access_token cookie, the token is bound to that session's jti.
func HandleGetCSRFToken(c fiber.Ctx, cfg *env.Config) error {
	token, err := IssueCSRF(c, cfg, jtiFromAccessCookie(c))
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to generate CSRF token", err)
	}
	return response.Success(c, fiber.StatusOK, "OK", fiber.Map{"token": token})
}
