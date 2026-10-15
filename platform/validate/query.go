package validate

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// BindQuery reads `query` struct tags from the request query string, then
// validates with the same tags used for JSON bodies. Use for GET handlers that
// need consistent BAD_REQUEST envelopes instead of ad-hoc checks.
func BindQuery(c fiber.Ctx, dst interface{}) error {
	if err := bindQuery(c, dst); err != nil {
		return writeEnvelope(c, "Invalid query parameters", []FieldError{
			{Field: "query", Rule: "bind", Message: err.Error()},
		})
	}
	if err := Get().Struct(dst); err != nil {
		return respondValidation(c, err)
	}
	return nil
}

func bindQuery(c fiber.Ctx, dst interface{}) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fiber.ErrBadRequest
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fiber.ErrBadRequest
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}
		key := strings.SplitN(sf.Tag.Get("query"), ",", 2)[0]
		if key == "" || key == "-" {
			continue
		}
		raw := strings.TrimSpace(c.Query(key))
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		if err := setQueryField(fv, raw); err != nil {
			return err
		}
	}
	return nil
}

func setQueryField(fv reflect.Value, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if raw == "" {
			fv.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Bool:
		if raw == "" {
			fv.SetBool(false)
			return nil
		}
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	default:
		return fiber.ErrBadRequest
	}
	return nil
}
