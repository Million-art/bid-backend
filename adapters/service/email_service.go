package service

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"

	notifyemail "github.com/bid-hub/admin-backend/adapters/notification/email"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/resend/resend-go/v2"
)

type ResendEmailService struct {
	client    *resend.Client
	from      string
	appName   string
	templates *template.Template
}

func NewResendEmailService(cfg *env.Config) (*ResendEmailService, error) {
	client := resend.NewClient(cfg.ResendAPIKey)

	tmpls, err := template.ParseFS(notifyemail.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse embedded email templates: %w", err)
	}

	return &ResendEmailService{
		client:    client,
		from:      cfg.ResendFromEmail,
		appName:   cfg.AppName,
		templates: tmpls,
	}, nil
}

// withAppName returns a shallow copy of data with AppName set for templates. Caller maps are not mutated.
func withAppName(data interface{}, appName string) interface{} {
	if data == nil {
		return map[string]interface{}{"AppName": appName}
	}
	switch d := data.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(d)+1)
		for k, v := range d {
			out[k] = v
		}
		out["AppName"] = appName
		return out
	case map[string]string:
		out := make(map[string]string, len(d)+1)
		for k, v := range d {
			out[k] = v
		}
		out["AppName"] = appName
		return out
	default:
		return data
	}
}

func (s *ResendEmailService) SendEmail(to []string, subject string, htmlContent string) error {
	slog.Info("resend: sending email", "to_count", len(to), "subject", subject)
	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      to,
		Subject: subject,
		Html:    htmlContent,
	}

	res, err := s.client.Emails.Send(params)
	if err != nil {
		slog.Error("resend: send failed", "error", err, "to_count", len(to), "subject", subject)
		return err
	}
	slog.Info("resend: send ok", "id", res.Id, "to_count", len(to))
	return nil
}

func (s *ResendEmailService) SendTemplateEmail(to []string, subject string, templateName string, data interface{}) error {
	if s.templates == nil {
		slog.Error("email: templates not initialized", "template", templateName)
		return fmt.Errorf("email templates not initialized")
	}

	var body bytes.Buffer
	if err := s.templates.ExecuteTemplate(&body, templateName+".html", withAppName(data, s.appName)); err != nil {
		slog.Error("email: template execute failed", "template", templateName, "error", err)
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return s.SendEmail(to, subject, body.String())
}

func (s *ResendEmailService) SendWelcomeEmail(to string, name string, link string) error {
	data := map[string]string{
		"Name": name,
		"Link": link,
	}
	return s.SendTemplateEmail([]string{to}, fmt.Sprintf("Welcome to %s", s.appName), "welcome", data)
}

func (s *ResendEmailService) SendbidSubmitted(to []string, data map[string]string) error {
	return s.SendTemplateEmail(to, "New bid Submitted", "bid_submitted", data)
}

func (s *ResendEmailService) SendbidApproved(to string, data map[string]string) error {
	return s.SendTemplateEmail([]string{to}, "bid Approved", "bid_approved", data)
}

func (s *ResendEmailService) SendbidRejected(to string, data map[string]string) error {
	return s.SendTemplateEmail([]string{to}, "bid Rejected", "bid_rejected", data)
}

func (s *ResendEmailService) SendbidScheduled(to []string, data map[string]string) error {
	return s.SendTemplateEmail(to, "bid Draw Scheduled", "bid_scheduled", data)
}

func (s *ResendEmailService) SendCountdownStarted(to []string, data map[string]string) error {
	return s.SendTemplateEmail(to, "bid Draw Countdown Started", "countdown", data)
}

func (s *ResendEmailService) SendPrizeCoordinationEmail(to string, data map[string]string) error {
	return s.SendTemplateEmail([]string{to}, "Action Required: Prize Delivery Coordination", "prize_coordination", data)
}

func (s *ResendEmailService) SendbidDrawPostponed(to string, data map[string]string) error {
	subject := "bid draw rescheduled"
	if n := data["bidName"]; n != "" {
		subject = fmt.Sprintf("Draw rescheduled: %s", n)
	}
	return s.SendTemplateEmail([]string{to}, subject, "bid_draw_postponed", data)
}
func (s *ResendEmailService) SendWinnerConfirmationRequest(to string, data map[string]string) error {
	subject := "🎉 You won! Action required to claim your prize"
	if n := data["bidName"]; n != "" {
		subject = fmt.Sprintf("🎉 You won %s!", n)
	}
	return s.SendTemplateEmail([]string{to}, subject, "winner_confirmation", data)
}
func (s *ResendEmailService) SendPayoutCompleted(to string, data map[string]string) error {
	return s.SendTemplateEmail([]string{to}, "💰 Payout Processed Successfully", "payout_completed", data)
}
