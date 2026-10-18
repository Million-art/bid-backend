package ports

import "strings"

// ChapaInitializeRequest contains parameters for POST /v1/transaction/initialize.
type ChapaInitializeRequest struct {
	Amount      string                 `json:"amount"`
	Currency    string                 `json:"currency"`
	Email       string                 `json:"email"`
	FirstName   string                 `json:"first_name,omitempty"`
	LastName    string                 `json:"last_name,omitempty"`
	PhoneNumber string                 `json:"phone_number,omitempty"`
	TxRef       string                 `json:"tx_ref"`
	CallbackURL string                 `json:"callback_url,omitempty"`
	ReturnURL   string                 `json:"return_url,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

// ChapaInitializeResponse is returned from Chapa POST /transaction/initialize.
type ChapaInitializeResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
	TxRef       string `json:"txRef"`
}

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
