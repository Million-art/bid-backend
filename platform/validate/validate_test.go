package validate

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBindAndValidate_requiredField(t *testing.T) {
	app := fiber.New()
	app.Post("/t", func(c fiber.Ctx) error {
		var req struct {
			Name string `json:"name" validate:"required"`
		}
		if err := BindAndValidate(c, &req); err != nil {
			return nil
		}
		return c.SendStatus(fiber.StatusCreated)
	})

	req := httptest.NewRequest(fiber.MethodPost, "/t", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

	body := decodeEnvelope(t, resp.Body)
	assert.False(t, body.Success)
	assert.Equal(t, "BAD_REQUEST", body.Error)
}

func TestBindPagination_defaults(t *testing.T) {
	app := fiber.New()
	var gotPage, gotLimit, gotOffset int
	app.Get("/t", func(c fiber.Ctx) error {
		var err error
		gotPage, gotLimit, gotOffset, err = BindPagination(c, 1, 20, 100)
		return err
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/t", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, gotPage)
	assert.Equal(t, 20, gotLimit)
	assert.Equal(t, 0, gotOffset)
}

func TestParamUUID_invalid(t *testing.T) {
	app := fiber.New()
	app.Get("/t/:id", func(c fiber.Ctx) error {
		_, err := ParamUUID(c, "id", "id")
		if IsParamInvalid(err) {
			return nil
		}
		return err
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/t/not-a-uuid", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

func TestParamUUID_valid(t *testing.T) {
	id := uuid.New()
	app := fiber.New()
	var parsed uuid.UUID
	app.Get("/t/:id", func(c fiber.Ctx) error {
		var err error
		parsed, err = ParamUUID(c, "id", "id")
		return err
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/t/"+id.String(), nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, id, parsed)
}

func TestFailVerificationToken(t *testing.T) {
	app := fiber.New()
	app.Get("/t", func(c fiber.Ctx) error {
		_ = FailVerificationToken(c)
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/t", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body := decodeEnvelope(t, resp.Body)
	assert.Equal(t, "UNAUTHORIZED", body.Error)
}

type envelope struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func decodeEnvelope(t *testing.T, r io.Reader) envelope {
	t.Helper()
	var body envelope
	require.NoError(t, json.NewDecoder(r).Decode(&body))
	return body
}
