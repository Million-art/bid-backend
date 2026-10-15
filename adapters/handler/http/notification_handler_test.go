package http

import (
	"testing"

	"github.com/bid-hub/admin-backend/core/domain"
)

func TestNotificationUserType(t *testing.T) {
	tests := []struct {
		role     domain.UserRole
		expected string
	}{
		{domain.RoleParticipant, "participant"},
		{domain.RoleAgent, "agent"},
		{domain.RoleAdmin, "admin"},
		{domain.RoleSuperAdmin, "admin"},
		{domain.RoleCompliance, "admin"},
		{domain.UserRole("unknown"), "participant"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			result := notificationUserType(tt.role)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
