package http

import (
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/gofiber/fiber/v3"
)

// SessionCookie builds an HttpOnly cookie carrying the cfg's security attributes
// (Secure, SameSite, Domain). It is the single source of truth for how this
// service emits auth / csrf cookies  keeping construction here makes it
// impossible for clear-side and set-side attributes to drift apart (a real
// browser-eviction footgun, see M1 fix).
//
// ttl > 0 sets a normal cookie expiring at now+ttl.
// ttl <= 0 mints a "clearing" cookie (MaxAge=-1 + Expires in the past).
func SessionCookie(name, value string, ttl time.Duration, cfg *env.Config) *fiber.Cookie {
	ck := &fiber.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HTTPOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: sameSiteAttr(cfg.CookieSameSite),
		Domain:   cookieDomainAttr(cfg.CookieDomain),
	}
	if ttl > 0 {
		ck.Expires = time.Now().Add(ttl)
		ck.MaxAge = int(ttl / time.Second)
	} else {
		ck.MaxAge = -1
		ck.Expires = time.Now().Add(-24 * time.Hour)
	}
	return ck
}

// ClearSessionCookie is a thin alias for SessionCookie(name, "", 0, cfg).
// Use at logout / invalid-session / revocation paths so the browser actually
// evicts the cookie (it only evicts when set-side attributes match).
func ClearSessionCookie(name string, cfg *env.Config) *fiber.Cookie {
	return SessionCookie(name, "", 0, cfg)
}
