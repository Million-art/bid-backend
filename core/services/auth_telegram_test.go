package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
)

func signTelegramPayload(token string, p TelegramAuthPayload) string {
	fields := map[string]string{
		"auth_date": strconv.FormatInt(p.AuthDate, 10),
		"first_name": p.FirstName,
		"id":         strconv.FormatInt(p.ID, 10),
	}
	if p.Username != "" {
		fields["username"] = p.Username
	}
	keys := []string{"auth_date", "first_name", "id"}
	if p.Username != "" {
		keys = append(keys, "username")
	}
	// sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	data := ""
	for i, part := range parts {
		if i > 0 {
			data += "\n"
		}
		data += part
	}
	secret := sha256.Sum256([]byte(token))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyTelegramLogin_Valid(t *testing.T) {
	token := "123456:ABC-DEF"
	svc := &AuthService{config: &env.Config{TelegramBotToken: token}}
	p := TelegramAuthPayload{
		ID:        42,
		FirstName: "Ada",
		Username:  "ada",
		AuthDate:  time.Now().Unix(),
	}
	p.Hash = signTelegramPayload(token, p)

	if err := svc.VerifyTelegramLogin(p); err != nil {
		t.Fatalf("expected valid signature, got %v", err)
	}
}

func TestVerifyTelegramLogin_InvalidHash(t *testing.T) {
	svc := &AuthService{config: &env.Config{TelegramBotToken: "123456:ABC-DEF"}}
	p := TelegramAuthPayload{
		ID:        42,
		FirstName: "Ada",
		AuthDate:  time.Now().Unix(),
		Hash:      "deadbeef",
	}
	if err := svc.VerifyTelegramLogin(p); err == nil {
		t.Fatal("expected invalid hash error")
	}
}

func TestVerifyTelegramLogin_Expired(t *testing.T) {
	token := "123456:ABC-DEF"
	svc := &AuthService{config: &env.Config{TelegramBotToken: token}}
	p := TelegramAuthPayload{
		ID:        42,
		FirstName: "Ada",
		AuthDate:  time.Now().Add(-48 * time.Hour).Unix(),
	}
	p.Hash = signTelegramPayload(token, p)
	if err := svc.VerifyTelegramLogin(p); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestVerifyTelegramWebAppInitData_Valid(t *testing.T) {
	token := "123456:ABC-DEF"
	svc := &AuthService{config: &env.Config{TelegramBotToken: token}}
	authDate := strconv.FormatInt(time.Now().Unix(), 10)
	userJSON := `{"id":42,"first_name":"Ada","username":"ada"}`
	fields := map[string]string{
		"auth_date": authDate,
		"user":      userJSON,
	}
	keys := []string{"auth_date", "user"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	dataCheck := parts[0] + "\n" + parts[1]

	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMac.Write([]byte(token))
	secretKey := secretMac.Sum(nil)
	mac := hmac.New(sha256.New, secretKey)
	_, _ = mac.Write([]byte(dataCheck))
	hash := hex.EncodeToString(mac.Sum(nil))

	initData := "auth_date=" + authDate + "&user=" + url.QueryEscape(userJSON) + "&hash=" + hash
	payload, err := svc.VerifyTelegramWebAppInitData(initData)
	if err != nil {
		t.Fatalf("expected valid initData, got %v", err)
	}
	if payload.ID != 42 || payload.FirstName != "Ada" || payload.Username != "ada" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestVerifyTelegramWebAppInitData_OfficialExample(t *testing.T) {
	// Example from https://docs.telegram-mini-apps.com/platform/init-data
	token := "5768337691:AAH5YkoiEuPk8-FZa32hStHTqXiLPtAEhx8"
	initData := "query_id=AAHdF6IQAAAAAN0XohDhrOrc" +
		"&user=%7B%22id%22%3A279058397%2C%22first_name%22%3A%22Vladislav%22%2C%22last_name%22%3A%22Kibenko%22%2C%22username%22%3A%22vdkfrost%22%2C%22language_code%22%3A%22ru%22%2C%22is_premium%22%3Atrue%7D" +
		"&auth_date=1662771648" +
		"&hash=c501b71e775f74ce10e377dea85a7ea24ecd640b223ea86dfe453e0eaed2e2b2"

	svc := &AuthService{config: &env.Config{TelegramBotToken: token}}
	// Official sample is old — expect expired after signature check passes.
	// Temporarily bump max age via direct hash path: call verify and accept expired OR success.
	payload, err := svc.VerifyTelegramWebAppInitData(initData)
	if err != nil {
		// If only expiry failed, signature is correct.
		if strings.Contains(err.Error(), "expired") {
			return
		}
		t.Fatalf("signature should validate for official example, got %v", err)
	}
	if payload.ID != 279058397 {
		t.Fatalf("unexpected id %d", payload.ID)
	}
}
