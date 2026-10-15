package paymentledger

import (
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
)

func verifyChapaPayment(chapa ports.PaymentProvider, merchantTxRef string) (*ports.ChapaTransactionSnapshot, error) {
	snap, err := chapa.VerifyTransaction(merchantTxRef)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") || strings.Contains(msg, "404") {
			return nil, domain.NewAppError(422,
				"Chapa does not recognize this payment with the admin API key. Confirm CHAPA_SECRET_KEY matches client-backend (same test/live mode) and the payment exists in the Chapa dashboard.",
				err)
		}
		return nil, domain.NewAppError(422, "could not verify transaction with Chapa before refund", err)
	}
	st := strings.ToLower(strings.TrimSpace(snap.Status))
	if st != "success" {
		return nil, domain.NewAppError(422,
			"Chapa reports transaction status \""+snap.Status+"\"; only successful payments can be refunded via API",
			nil)
	}
	return snap, nil
}
