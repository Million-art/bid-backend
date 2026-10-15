package payment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/ports"
)

type ChapaClient struct {
	secretKey string
	baseURL   string
}

func NewChapaClient(secretKey, baseURL string) *ChapaClient {
	if baseURL == "" {
		baseURL = "https://api.chapa.co/v1"
	}
	return &ChapaClient{
		secretKey: secretKey,
		baseURL:   strings.TrimRight(baseURL, "/"),
	}
}

type verifyResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	Data    struct {
		TxRef     string      `json:"tx_ref"`
		Reference string      `json:"reference"`
		Amount    interface{} `json:"amount"`
		Currency  string      `json:"currency"`
		Status    string      `json:"status"`
		Meta      interface{} `json:"meta"`
		Customer  struct {
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"customer"`
	} `json:"data"`
}

func (c *ChapaClient) VerifyTransaction(txRef string) (*ports.ChapaTransactionSnapshot, error) {
	txRef = strings.TrimSpace(txRef)
	if txRef == "" {
		return nil, fmt.Errorf("empty tx_ref")
	}
	httpReq, err := http.NewRequest(http.MethodGet, c.baseURL+"/transaction/verify/"+url.PathEscape(txRef), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create verify request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chapa verify request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("chapa verify: transaction not found for tx_ref %q (check test vs live CHAPA_SECRET_KEY): %s", txRef, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chapa verify api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var vr verifyResponse
	if err := json.Unmarshal(respBody, &vr); err != nil {
		return nil, fmt.Errorf("failed to decode verify response: %w", err)
	}
	amount := ""
	switch v := vr.Data.Amount.(type) {
	case string:
		amount = v
	case float64:
		amount = fmt.Sprintf("%.2f", v)
	}
	return &ports.ChapaTransactionSnapshot{
		TxRef:         vr.Data.TxRef,
		Reference:     vr.Data.Reference,
		Status:        vr.Data.Status,
		Amount:        amount,
		Currency:      vr.Data.Currency,
		Meta:          vr.Data.Meta,
		CustomerEmail: vr.Data.Customer.Email,
		CustomerFirst: vr.Data.Customer.FirstName,
	}, nil
}

type RefundResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	Data    struct {
		RefID string `json:"ref_id"`
		TxRef string `json:"tx_ref"`
	} `json:"data"`
}

type VerifyRefundResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	Data    struct {
		Amount           interface{} `json:"amount"`
		Currency         string      `json:"currency"`
		RefID            string      `json:"ref_id"`
		PaymentReference string      `json:"payment_reference"`
		Status           string      `json:"status"`
		CreatedAt        string      `json:"created_at"`
		UpdatedAt        string      `json:"updated_at"`
	} `json:"data"`
}

func (c *ChapaClient) InitiateRefund(txRef string, amount float64, reason string, internalRef string) (string, error) {
	txRef = strings.TrimSpace(txRef)
	form := url.Values{}
	if strings.TrimSpace(reason) != "" {
		form.Set("reason", reason)
	}
	if amount > 0 {
		form.Set("amount", fmt.Sprintf("%.2f", amount))
	}
	if strings.TrimSpace(internalRef) != "" {
		form.Set("reference", internalRef)
	}

	refURL := c.baseURL + "/refund/" + url.PathEscape(txRef)
	httpReq, err := http.NewRequest(http.MethodPost, refURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create refund request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chapa refund request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("chapa refund api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var refundResp RefundResponse
	if err := json.Unmarshal(respBody, &refundResp); err != nil {
		return "", fmt.Errorf("failed to decode refund response: %w", err)
	}
	ref := refundResp.Data.RefID
	if ref == "" {
		ref = refundResp.Data.TxRef
	}
	if ref == "" {
		return "", fmt.Errorf("chapa refund response missing ref_id: %s", string(respBody))
	}
	return ref, nil
}

func (c *ChapaClient) VerifyRefund(refID string) (string, error) {
	refURL := c.baseURL + "/refund/" + url.PathEscape(refID) + "/verify"
	httpReq, err := http.NewRequest(http.MethodGet, refURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create verify request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chapa verify request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chapa verify api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var verifyResp VerifyRefundResponse
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return "", fmt.Errorf("failed to decode verify response: %w", err)
	}

	return verifyResp.Data.Status, nil
}

// Ensure ChapaClient implements PaymentProvider at compile time.
var _ ports.PaymentProvider = (*ChapaClient)(nil)
