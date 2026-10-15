package paymentledger

import (
	"testing"

	"github.com/bid-hub/admin-backend/core/domain"
)

func TestChapaRefundOutcome(t *testing.T) {
	tests := []struct {
		in       string
		record   string
		refunded bool
	}{
		{"refunded", domain.RefundStatusSuccess, true},
		{"REFUNDED", domain.RefundStatusSuccess, true},
		{"reversed", domain.RefundStatusReversed, false},
		{"initiated", domain.RefundStatusProcessing, false},
		{"processing", domain.RefundStatusProcessing, false},
		{"", domain.RefundStatusProcessing, false},
		{"unknown", domain.RefundStatusProcessing, false},
	}
	for _, tc := range tests {
		r, p := chapaRefundOutcome(tc.in)
		if r != tc.record || p != tc.refunded {
			t.Fatalf("chapaRefundOutcome(%q) = (%q, %v), want (%q, %v)", tc.in, r, p, tc.record, tc.refunded)
		}
	}
}
