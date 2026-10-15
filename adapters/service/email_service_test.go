package service

import (
	"testing"
)

func TestWithAppName(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		appName string
	}{
		{
			name:    "nil data",
			input:   nil,
			appName: "carra market",
		},
		{
			name: "map[string]string",
			input: map[string]string{
				"Name": "Test",
			},
			appName: "carra market",
		},
		{
			name: "map[string]interface{}",
			input: map[string]interface{}{
				"bidName": "Test bid",
			},
			appName: "carra market",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withAppName(tt.input, tt.appName)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
		})
	}
}

func TestResendEmailService_TemplatesNil(t *testing.T) {
	s := &ResendEmailService{
		templates: nil,
	}

	err := s.SendTemplateEmail([]string{"[EMAIL_ADDRESS]"}, "Test", "welcome", nil)
	if err == nil {
		t.Error("expected error when templates is nil")
	}
	if err != nil && err.Error() != "email templates not initialized" {
		t.Errorf("unexpected error message: %v", err)
	}
}
