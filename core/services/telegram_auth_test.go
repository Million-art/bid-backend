package services

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

func TestVerifyTelegramWebAppInitData(t *testing.T) {
	const (
		testInitData = "query_id=AAHdF6IQAAAAAN0XohDhrOrc&user=%7B%22id%22%3A279058397%2C%22first_name%22%3A%22Vladislav%22%2C%22last_name%22%3A%22Kibenko%22%2C%22username%22%3A%22vdkfrost%22%2C%22language_code%22%3A%22ru%22%2C%22is_premium%22%3Atrue%7D&auth_date=1662771648&hash=c501b71e775f74ce10e377dea85a7ea24ecd640b223ea86dfe453e0eaed2e2b2"
		testToken    = "5768337691:AAH5YkoiEuPk8-FZa32hStHTqXiLPtAEhx8"
	)

	cfg := &env.Config{
		TelegramBotToken: testToken,
	}
	authSvc := &AuthService{
		config: cfg,
	}

	payload, err := authSvc.VerifyTelegramWebAppInitData(testInitData)
	if err != nil {
		assert.Contains(t, err.Error(), "expired")
	} else {
		require.NotNil(t, payload)
	}

	// Test with official initdata package
	errInitDataPkg := initdata.Validate(testInitData, testToken, 0)
	assert.NoError(t, errInitDataPkg, "initdata.Validate should pass with 0 expiration check")

	const (
		readmeInitData = "user=%7B%22id%22%3A279058397%2C%22first_name%22%3A%22Vladislav%20%2B%20-%20%3F%20%5C%2F%22%2C%22last_name%22%3A%22Kibenko%22%2C%22username%22%3A%22vdkfrost%22%2C%22language_code%22%3A%22ru%22%2C%22is_premium%22%3Atrue%2C%22allows_write_to_pm%22%3Atrue%2C%22photo_url%22%3A%22https%3A%5C%2F%5C%2Ft.me%5C%2Fi%5C%2Fuserpic%5C%2F320%5C%2F4FPEE4tmP3ATHa57u6MqTDih13LTOiMoKoLDRG4PnSA.svg%22%7D&chat_instance=8134722200314281151&chat_type=private&auth_date=1733509682&signature=TYJxVcisqbWjtodPepiJ6ghziUL94-KNpG8Pau-X7oNNLNBM72APCpi_RKiUlBvcqo5L-LAxIc3dnTzcZX_PDg&hash=a433d8f9847bd6addcc563bff7cc82c89e97ea0d90c11fe5729cae6796a36d73"
		readmeToken    = "7342037359:AAHI25ES9xCOMPokpYoz-p8XVrZUdygo2J4"
	)
	errReadmePkg := initdata.Validate(readmeInitData, readmeToken, 0)
	assert.NoError(t, errReadmePkg, "initdata.Validate should pass with 0 expiration check for readme data")

	// Also verify fresh signed payload with signature field through AuthService
	now := time.Now()
	params := map[string]string{
		"auth_date":     strconv.FormatInt(now.Unix(), 10),
		"chat_instance": "8134722200314281151",
		"chat_type":     "private",
		"signature":     "TYJxVcisqbWjtodPepiJ6ghziUL94-KNpG8Pau-X7oNNLNBM72APCpi_RKiUlBvcqo5L-LAxIc3dnTzcZX_PDg",
		"user":          `{"id":279058397,"first_name":"Vladislav","last_name":"Kibenko","username":"vdkfrost","language_code":"ru","is_premium":true,"allows_write_to_pm":true}`,
	}
	hash := initdata.Sign(params, readmeToken, now)

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	q.Set("hash", hash)

	readmeSvc := &AuthService{config: &env.Config{TelegramBotToken: readmeToken}}
	got, verifyErr := readmeSvc.VerifyTelegramWebAppInitData(q.Encode())
	require.NoError(t, verifyErr)
	assert.Equal(t, int64(279058397), got.ID)
	assert.Equal(t, "vdkfrost", got.Username)

	parsed, errParse := initdata.Parse(testInitData)
	require.NoError(t, errParse)
	assert.Equal(t, int64(279058397), parsed.User.ID)
	assert.Equal(t, "Vladislav", parsed.User.FirstName)
}

