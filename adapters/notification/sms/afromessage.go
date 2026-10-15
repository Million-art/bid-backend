package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
)

type AfroMessageService struct {
	cfg    *env.Config
	client *http.Client
}

func NewAfroMessageService(cfg *env.Config) *AfroMessageService {
	return &AfroMessageService{
		cfg:    cfg,
		client: &http.Client{Timeout: 45 * time.Second},
	}
}

func normalizeAfroAPIKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"'`")
	for _, prefix := range []string{"AFROMESSAGE_TOKEN=", "AFRO_MESSAGE_API_KEY=", "Bearer "} {
		if strings.HasPrefix(key, prefix) {
			key = strings.TrimSpace(strings.TrimPrefix(key, prefix))
		}
	}
	key = strings.Trim(key, "\"'`")
	if strings.HasPrefix(key, ".ey") {
		key = strings.TrimPrefix(key, ".")
	}
	return key
}

func normalizePhoneForAfro(phone string) string {
	s := strings.TrimSpace(phone)
	s = strings.TrimPrefix(s, "+")
	for len(s) > 0 && s[0] == '0' {
		s = s[1:]
	}
	if !strings.HasPrefix(s, "251") {
		s = "251" + s
	}
	return s
}

func (s *AfroMessageService) SendSms(to string, message string) error {
	apiKey := normalizeAfroAPIKey(s.cfg.AfroMessageAPIKey)
	if apiKey == "" {
		slog.Warn("AfroMessage API key not configured, skipping SMS", "to", to)
		return fmt.Errorf("afromessage: API key not configured")
	}

	baseURL := strings.TrimSuffix(s.cfg.AfroMessageBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.afromessage.com"
	}
	if !strings.HasSuffix(baseURL, "/api/send") {
		baseURL = baseURL + "/api/send"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		slog.Error("failed to parse afromessage url", "error", err)
		return fmt.Errorf("failed to parse afromessage url: %w", err)
	}

	formattedTo := normalizePhoneForAfro(to)

	q := u.Query()
	q.Set("to", formattedTo)
	q.Set("message", message)
	q.Set("token", apiKey)
	if s.cfg.AfroMessageSenderName != "" {
		q.Set("sender", s.cfg.AfroMessageSenderName)
	}
	if s.cfg.AfroMessageIdentifierID != "" {
		q.Set("from", s.cfg.AfroMessageIdentifierID)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		slog.Error("failed to create afromessage request", "error", err)
		return fmt.Errorf("failed to create afromessage request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("failed to execute afromessage request", "to", formattedTo, "error", err)
		return fmt.Errorf("failed to execute afromessage request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		slog.Error("afro message: failed to read response", "error", err)
		return fmt.Errorf("afro message: failed to read response: %w", err)
	}

	var result struct {
		Acknowledge string `json:"acknowledge"`
		Response    struct {
			Errors []string `json:"errors"`
		} `json:"response"`
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &result)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		slog.Error("AfroMessage API key unauthorized", "status", resp.StatusCode)
		return fmt.Errorf("afro message: invalid or expired API key (401)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(result.Response.Errors) > 0 {
			slog.Error("AfroMessage API error", "status", resp.StatusCode, "error", result.Response.Errors[0])
			return fmt.Errorf("afro message: %s", result.Response.Errors[0])
		}
		slog.Error("AfroMessage API status error", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("afro message api returned status %d", resp.StatusCode)
	}
	if result.Acknowledge != "success" {
		msg := "afro message api failed"
		if len(result.Response.Errors) > 0 {
			msg = result.Response.Errors[0]
		}
		slog.Error("AfroMessage acknowledgement failed", "msg", msg, "body", string(body))
		return fmt.Errorf("afro message: %s", msg)
	}

	slog.Info("SMS sent successfully via AfroMessage", "to", formattedTo, "ack", result.Acknowledge)
	return nil
}

func (s *AfroMessageService) SendWinnerAnnounced(to string, data map[string]string) error {
	return s.SendSms(to, fmt.Sprintf("እንኳን ደስ አለዎት! '%s' አሸንፈዋል። ከጨራ ገበያ ቡድን!", data["bidName"]))
}

func (s *AfroMessageService) SendbidApproved(to string, data map[string]string) error {
	return s.SendSms(to, fmt.Sprintf("'%s' ጸድቋል። አሁን ለተሳታፊዎች ክፍት ነው። - ጨራ ገበያ", data["bidName"]))
}

func (s *AfroMessageService) SendbidRejected(to string, data map[string]string) error {
	return s.SendSms(to, fmt.Sprintf("'%s' አልጸደቀም። ምክንያት፡ %s። ለተጨማሪ መረጃ ጨራ ገበያን ያነጋግሩ።", data["bidName"], data["reason"]))
}

func (s *AfroMessageService) SendCountdownStarted(to []string, data map[string]string) error {
	for _, num := range to {
		_ = s.SendSms(num, fmt.Sprintf("'%s' ዕጣ ሊወጣ ነው! ዝግጁ ሁኑ። - ጨራ ገበያ", data["bidName"]))
	}
	return nil
}

func (s *AfroMessageService) SendPayoutCompleted(to string, data map[string]string) error {
	return s.SendSms(to, fmt.Sprintf("የ%s ብር ክፍያዎ ተጠናቋል። ወደ አካውንትዎ ተላልፏል። - ጨራ ገበያ", data["amount"]))
}
