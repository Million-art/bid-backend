package paymentledger

import (
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
)

// chapaRefundOutcome maps Chapa GET /refund/:ref_id/verify data.status to our refund record state.
// See https://developer.chapa.co  refund statuses: initiated, processing, refunded, reversed.
func chapaRefundOutcome(chapaStatus string) (recordStatus string, paymentRefunded bool) {
	switch strings.ToLower(strings.TrimSpace(chapaStatus)) {
	case "refunded":
		return domain.RefundStatusSuccess, true
	case "reversed":
		return domain.RefundStatusReversed, false
	case "initiated", "processing":
		return domain.RefundStatusProcessing, false
	default:
		return domain.RefundStatusProcessing, false
	}
}
