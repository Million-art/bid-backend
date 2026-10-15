package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bid-hub/admin-backend/platform/env"
)

func TestClientBackendSmsService_SendSms(t *testing.T) {
	t.Run("skips when config missing", func(t *testing.T) {
		cfg := &env.Config{}
		s := NewClientBackendSmsService(cfg)
		err := s.SendSms("+251911000000", "Test message")
		if err != nil {
			t.Errorf("expected no error when config missing, got %v", err)
		}
	})

	t.Run("relays to internal endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/internal/notify/send-message" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("X-Client-Secret") != "test-secret" {
				t.Errorf("missing or wrong secret header")
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &env.Config{
			ClientBackendInternalURL: server.URL,
			ClientSecret:             "test-secret",
		}
		s := NewClientBackendSmsService(cfg)
		err := s.SendSms("+251911000000", "Test message")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on non-2xx response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := &env.Config{
			ClientBackendInternalURL: server.URL,
			ClientSecret:             "test-secret",
		}
		s := NewClientBackendSmsService(cfg)
		err := s.SendSms("+251911000000", "Test message")
		if err == nil {
			t.Error("expected error on 500 response")
		}
	})
}

func TestClientBackendSmsService_WinnerAnnounced(t *testing.T) {
	t.Run("relays winner announced sms", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/internal/notify/winner-announced-sms" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &env.Config{
			ClientBackendInternalURL: server.URL,
			ClientSecret:             "test-secret",
		}
		s := NewClientBackendSmsService(cfg)
		data := map[string]string{
			"bidName":          "Test bid",
			"ConfirmationLink": "https://carramarket.com/confirm",
		}
		err := s.SendWinnerAnnounced("+251911000000", data)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestClientBackendSmsService_PayoutCompleted(t *testing.T) {
	t.Run("relays payout completed sms", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/internal/notify/payout-completed-sms" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &env.Config{
			ClientBackendInternalURL: server.URL,
			ClientSecret:             "test-secret",
		}
		s := NewClientBackendSmsService(cfg)
		data := map[string]string{
			"Amount":         "100.00",
			"TransactionRef": "TX-123",
		}
		err := s.SendPayoutCompleted("+251911000000", data)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestClientBackendSmsService_bidApproved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/internal/notify/bid-approved-sms" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &env.Config{
		ClientBackendInternalURL: server.URL,
		ClientSecret:             "test-secret",
	}
	s := NewClientBackendSmsService(cfg)
	data := map[string]string{"bidName": "Test"}
	err := s.SendbidApproved("+251911000000", data)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestClientBackendSmsService_bidRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/internal/notify/bid-rejected-sms" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &env.Config{
		ClientBackendInternalURL: server.URL,
		ClientSecret:             "test-secret",
	}
	s := NewClientBackendSmsService(cfg)
	data := map[string]string{"bidName": "Test", "Reason": "Test reason"}
	err := s.SendbidRejected("+251911000000", data)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestClientBackendSmsService_CountdownStarted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/internal/notify/countdown-started-sms" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &env.Config{
		ClientBackendInternalURL: server.URL,
		ClientSecret:             "test-secret",
	}
	s := NewClientBackendSmsService(cfg)
	data := map[string]string{"bidName": "Test", "CountdownSecs": "60"}
	err := s.SendCountdownStarted([]string{"+251911000000"}, data)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestClientBackendSmsService_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/internal/notify/send-message" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &env.Config{
		ClientBackendInternalURL: server.URL,
		ClientSecret:             "test-secret",
	}
	s := NewClientBackendSmsService(cfg)
	err := s.SendMessage("+251911000000", "Test message")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
