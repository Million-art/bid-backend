package validate

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

var e164Re = regexp.MustCompile(`^\+?[1-9]\d{7,14}$`)

func registerCustomValidators(v *validator.Validate) {
	_ = v.RegisterValidation("e164", func(fl validator.FieldLevel) bool {
		s := strings.TrimSpace(fl.Field().String())
		if s == "" {
			return false
		}
		return e164Re.MatchString(s)
	})

	_ = v.RegisterValidation("strong_password", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if len(s) < 8 {
			return false
		}
		var hasUpper, hasLower, hasDigit bool
		for _, r := range s {
			switch {
			case r >= 'A' && r <= 'Z':
				hasUpper = true
			case r >= 'a' && r <= 'z':
				hasLower = true
			case r >= '0' && r <= '9':
				hasDigit = true
			}
		}
		return hasUpper && hasLower && hasDigit
	})

	// money_amount: positive decimal string (>= 0.01). Used on wire amounts when
	// the handler receives a string rather than a float64.
	_ = v.RegisterValidation("money_amount", func(fl validator.FieldLevel) bool {
		s := strings.TrimSpace(fl.Field().String())
		if s == "" {
			return false
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return false
		}
		return d.GreaterThanOrEqual(decimal.NewFromFloat(0.01))
	})
}
