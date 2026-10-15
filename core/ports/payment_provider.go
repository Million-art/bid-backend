package ports

import "strings"

// ChapaTransactionSnapshot is returned from Chapa GET /transaction/verify/{tx_ref}.
type ChapaTransactionSnapshot struct {
	TxRef         string
	Reference     string // Chapa-side reference (APqDvY…)  use for POST /refund/{ref}
	Status        string // success, failed, pending, …
	Amount        string
	Currency      string
	Meta          interface{}
	CustomerEmail string
	CustomerFirst string
}

// PickChapaRefundPathRef returns the path segment for POST /v1/refund/{ref}.
// Verify with the merchant tx_ref (e.g. R-…); refund with Chapa's reference when present.
func PickChapaRefundPathRef(snap *ChapaTransactionSnapshot, merchantTxRef string) string {
	if snap != nil {
		if r := strings.TrimSpace(snap.Reference); r != "" {
			return r
		}
		if r := strings.TrimSpace(snap.TxRef); r != "" && !strings.HasPrefix(r, "R-") {
			return r
		}
	}
	return strings.TrimSpace(merchantTxRef)
}
