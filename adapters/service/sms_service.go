package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
)

type ClientBackendSmsService struct {
	internalURL string
	secret      string
	client      *http.Client
}

func NewClientBackendSmsService(cfg *env.Config) *ClientBackendSmsService {
	return &ClientBackendSmsService{
		internalURL: cfg.ClientBackendInternalURL,
		secret:      cfg.ClientSecret,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *ClientBackendSmsService) SendSms(to string, message string) error {
	slog.Info("sms: relaying generic sms", "to", to)
	return s.relay("send-message", map[string]string{
		"to":      to,
		"message": message,
	})
}

func (s *ClientBackendSmsService) relay(endpoint string, payload interface{}) error {
	if s.internalURL == "" || s.secret == "" {
		slog.Warn("sms: relay skipped - missing client config", "endpoint", endpoint)
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqURL := fmt.Sprintf("%s/api/internal/notify/%s", s.internalURL, endpoint)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Secret", s.secret)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sms relay to %s failed with status: %s", endpoint, resp.Status)
	}

	return nil
}

func (s *ClientBackendSmsService) SendWinnerAnnounced(to string, data map[string]string) error {
	payload := map[string]interface{}{
		"to":               to,
		"bidName":          data["bidName"],
		"confirmationLink": data["ConfirmationLink"],
	}
	err := s.relay("winner-announced-sms", payload)
	if err == nil {
		slog.Info("sms: winner announced relay ok", "to", to)
	}
	return err
}

func (s *ClientBackendSmsService) SendbidApproved(to string, data map[string]string) error {
	payload := map[string]interface{}{
		"to":      to,
		"bidName": data["bidName"],
	}
	err := s.relay("bid-approved-sms", payload)
	if err == nil {
		slog.Info("sms: bid approved relay ok", "to", to)
	}
	return err
}

func (s *ClientBackendSmsService) SendbidRejected(to string, data map[string]string) error {
	payload := map[string]interface{}{
		"to":      to,
		"bidName": data["bidName"],
		"reason":  data["Reason"],
	}
	err := s.relay("bid-rejected-sms", payload)
	if err == nil {
		slog.Info("sms: bid rejected relay ok", "to", to)
	}
	return err
}

func (s *ClientBackendSmsService) SendCountdownStarted(to []string, data map[string]string) error {
	payload := map[string]interface{}{
		"recipients":    to,
		"bidName":       data["bidName"],
		"countdownSecs": data["CountdownSecs"],
	}
	err := s.relay("countdown-started-sms", payload)
	if err == nil {
		slog.Info("sms: countdown started relay ok", "count", len(to))
	}
	return err
}

// SendMessage sends an SMS message to the specified phone number.
// This relays the request to the client-backend's internal SMS endpoint.
func (s *ClientBackendSmsService) SendMessage(to string, message string) error {
	return s.relay("send-message", map[string]string{
		"to":      to,
		"message": message,
	})
}

func (s *ClientBackendSmsService) SendPayoutCompleted(to string, data map[string]string) error {
	payload := map[string]interface{}{
		"to":             to,
		"amount":         data["Amount"],
		"transactionRef": data["TransactionRef"],
	}
	err := s.relay("payout-completed-sms", payload)
	if err == nil {
		slog.Info("sms: payout completed relay ok", "to", to)
	}
	return err
}
