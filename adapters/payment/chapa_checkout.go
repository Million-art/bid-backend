package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Minimal Chapa checkout + verify for featured campaigns.

type InitializeRequest struct {
	Amount        string                 `json:"amount"`
	Currency      string                 `json:"currency"`
	Email         string                 `json:"email"`
	FirstName     string                 `json:"first_name"`
	LastName      string                 `json:"last_name"`
	PhoneNumber   string                 `json:"phone_number,omitempty"`
	TxRef         string                 `json:"tx_ref"`
	CallbackURL   string                 `json:"callback_url,omitempty"`
	ReturnURL     string                 `json:"return_url,omitempty"`
	Customization map[string]string      `json:"customization,omitempty"`
	Meta          map[string]interface{} `json:"meta,omitempty"`
}

type InitializeResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	Data    struct {
		CheckoutURL string `json:"checkout_url"`
	} `json:"data"`
}

type VerifyResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
	Data    struct {
		Status   string      `json:"status"` // success, failed, pending
		TxRef    string      `json:"tx_ref"`
		TrxRef   string      `json:"trx_ref"`
		Amount   interface{} `json:"amount"`
		Currency string      `json:"currency"`
		Meta     interface{} `json:"meta"`
		Customer struct {
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"customer"`
	} `json:"data"`
}

func (c *ChapaClient) InitializeTransaction(req InitializeRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal chapa init request: %w", err)
	}
	url := fmt.Sprintf("%s/transaction/initialize", c.baseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create chapa init request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chapa init request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("chapa init api error (status %d): %s", resp.StatusCode, string(respBody))
	}
	var initResp InitializeResponse
	if err := json.Unmarshal(respBody, &initResp); err != nil {
		return "", fmt.Errorf("decode chapa init response: %w", err)
	}
	if initResp.Data.CheckoutURL == "" {
		return "", fmt.Errorf("missing checkout_url from chapa")
	}
	return initResp.Data.CheckoutURL, nil
}

func (c *ChapaClient) VerifyCheckout(txRef string) (*VerifyResponse, error) {
	url := fmt.Sprintf("%s/transaction/verify/%s", c.baseURL, txRef)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create chapa verify request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chapa verify request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chapa verify api error (status %d): %s", resp.StatusCode, string(respBody))
	}
	var verifyResp VerifyResponse
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return nil, fmt.Errorf("decode chapa verify response: %w", err)
	}
	return &verifyResp, nil
}
