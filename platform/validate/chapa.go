package validate

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ErrMissingChapaTxRef is returned when no tx_ref can be resolved from query or body.
var ErrMissingChapaTxRef = errors.New("missing chapa transaction reference")

// ChapaWebhookBody models the common Chapa webhook/callback JSON shapes. Chapa
// has shipped tx_ref at the root, under data.*, and under meta.*  we accept
// all of them and normalize to a single string.
type ChapaWebhookBody struct {
	TxRef      string          `json:"tx_ref"`
	TrxRef     string          `json:"trx_ref"`
	TxRefCamel string          `json:"txRef"`
	Data       *ChapaNestedRef `json:"data"`
	Meta       *ChapaNestedRef `json:"meta"`
}

// ChapaNestedRef holds tx_ref variants inside data/meta objects.
type ChapaNestedRef struct {
	TxRef      string `json:"tx_ref"`
	TrxRef     string `json:"trx_ref"`
	TxRefCamel string `json:"txRef"`
}

// ChapaQueryRefs binds Chapa redirect/callback query parameters.
type ChapaQueryRefs struct {
	TrxRef     string `query:"trx_ref"`
	TxRef      string `query:"tx_ref"`
	TxRefCamel string `query:"txRef"`
	Reference  string `query:"reference"`
}

// FirstNonEmpty returns the first non-blank string.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ResolveChapaTxRefFromQuery reads tx_ref from common Chapa query param names.
func ResolveChapaTxRefFromQuery(c fiber.Ctx) string {
	return FirstNonEmpty(
		c.Query("trx_ref"),
		c.Query("tx_ref"),
		c.Query("txRef"),
		c.Query("reference"),
	)
}

// ResolveChapaTxRefFromQueryStruct binds query tags then resolves tx_ref.
func ResolveChapaTxRefFromQueryStruct(c fiber.Ctx) (string, error) {
	var q ChapaQueryRefs
	if err := bindQuery(c, &q); err != nil {
		return "", err
	}
	ref := FirstNonEmpty(q.TrxRef, q.TxRef, q.TxRefCamel, q.Reference)
	if ref == "" {
		return "", ErrMissingChapaTxRef
	}
	return ref, nil
}

// ResolveChapaTxRef extracts a transaction reference from an optional query
// value and/or JSON body. Query wins when non-empty (browser redirects).
func ResolveChapaTxRef(queryRef string, body []byte) (string, error) {
	if ref := strings.TrimSpace(queryRef); ref != "" {
		return ref, nil
	}
	if len(body) == 0 {
		return "", ErrMissingChapaTxRef
	}
	var wh ChapaWebhookBody
	if err := json.Unmarshal(body, &wh); err != nil {
		return "", err
	}
	ref := FirstNonEmpty(wh.TrxRef, wh.TxRef, wh.TxRefCamel)
	if wh.Data != nil {
		ref = FirstNonEmpty(ref, wh.Data.TrxRef, wh.Data.TxRef, wh.Data.TxRefCamel)
	}
	if wh.Meta != nil {
		ref = FirstNonEmpty(ref, wh.Meta.TxRef, wh.Meta.TrxRef, wh.Meta.TxRefCamel)
	}
	if ref == "" {
		return "", ErrMissingChapaTxRef
	}
	return ref, nil
}

// ResolveChapaTxRefFromCtx combines query params and request body.
func ResolveChapaTxRefFromCtx(c fiber.Ctx) (string, error) {
	return ResolveChapaTxRef(ResolveChapaTxRefFromQuery(c), c.Body())
}
