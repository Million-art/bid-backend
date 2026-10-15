package paymentledger

import (
	"testing"

	"github.com/bid-hub/admin-backend/core/domain"
)

func TestRefundEligible(t *testing.T) {
	ok := &domain.PaymentStaffView{
		Payment: domain.Payment{Status: domain.PaymentStatusSuccess, Processed: false},
	}
	if !RefundEligible(ok) {
		t.Fatal("expected unprocessed success payment to be refundable")
	}

	processed := &domain.PaymentStaffView{
		Payment: domain.Payment{Status: domain.PaymentStatusSuccess, Processed: true},
	}
	if RefundEligible(processed) {
		t.Fatal("processed payment should not be refundable")
	}
}
