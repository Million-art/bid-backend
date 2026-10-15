package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/bid-hub/admin-backend/adapters/handler/http"
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/gofiber/fiber/v3"
)

func testApp(t *testing.T) *fiber.App {
	cfg := &env.Config{
		CookieDomain:   "",
		CookieSecure:   false,
		CookieSameSite: "Lax",
		CSRFSecret:     "test-csrf-secret-must-be-at-least-32-chars-long",
	}
	app := fiber.New()
	app.Get("/health", func(c fiber.Ctx) error {
		return response.Success(c, fiber.StatusOK, "Admin Backend is running", fiber.Map{"status": "active"})
	})
	api := app.Group("/api", handler.CSRF(cfg))
	api.Get("/csrf-token", func(c fiber.Ctx) error { return handler.HandleGetCSRFToken(c, cfg) })
	api.Post("/auth/login", func(c fiber.Ctx) error {
		return response.Error(c, fiber.StatusUnauthorized, "Invalid credentials", nil)
	})
	return app
}

func TestIntegration_Health(t *testing.T) {
	app := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health: want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("health: invalid JSON: %v", err)
	}
	if m["success"] != true {
		t.Errorf("health: want success=true, got %v", m["success"])
	}
}

func TestIntegration_CSRFFlow(t *testing.T) {
	app := testApp(t)

	// 1. GET /api/csrf-token: returns signed token + sets sid cookie.
	req := httptest.NewRequest(http.MethodGet, "/api/csrf-token", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("csrf-token request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("csrf-token: want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var csrfResp struct {
		Success bool `json:"success"`
		Data    *struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &csrfResp); err != nil {
		t.Fatalf("csrf-token: invalid JSON: %v", err)
	}
	if !csrfResp.Success || csrfResp.Data == nil || csrfResp.Data.Token == "" {
		t.Errorf("csrf-token: want success with token, got %s", string(body))
	}
	token := csrfResp.Data.Token

	var sidCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == handler.CSRFCookieName {
			sidCookie = c
			break
		}
	}
	if sidCookie == nil {
		t.Fatal("csrf-token: expected csrf_token (sid) cookie to be set")
	}
	// In the signed scheme the cookie holds a sid, NOT the token itself.
	if sidCookie.Value == token {
		t.Errorf("csrf-token: cookie value should hold sid, not the signed token")
	}

	// 2. POST without CSRF header → 403.
	loginBody := []byte(`{"email":"test@example.com","password":"Test1234"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("login without CSRF failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("login without CSRF: want 403, got %d", resp2.StatusCode)
	}

	// 3. POST with CSRF header + sid cookie → bypasses CSRF, returns 401 (bad creds).
	req3 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set(handler.CSRFHeaderName, token)
	req3.AddCookie(sidCookie)
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("login with CSRF failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("login with CSRF: want 401, got %d", resp3.StatusCode)
	}
}
