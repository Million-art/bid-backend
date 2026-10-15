package domain

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

type Money struct {
	d decimal.Decimal
}

func NewMoney(f float64) Money {
	return Money{d: decimal.NewFromFloat(f).Round(2)}
}

func NewMoneyFromString(s string) (Money, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Money{}, fmt.Errorf("money: cannot parse %q: %w", s, err)
	}
	return Money{d: d.Round(2)}, nil
}

func Zero() Money { return Money{} }

func (m Money) Add(other Money) Money {
	return Money{m.d.Add(other.d).Round(2)}
}

func (m Money) Sub(other Money) Money {
	return Money{m.d.Sub(other.d).Round(2)}
}

func (m Money) Mul(other decimal.Decimal) Money {
	return Money{m.d.Mul(other).Round(2)}
}

func (m Money) Neg() Money {
	return Money{m.d.Neg().Round(2)}
}

func (m Money) String() string {
	if m.d.IsZero() {
		return "0.00"
	}
	return m.d.StringFixed(2)
}

func (m Money) Float64() float64 {
	f, _ := m.d.Float64()
	return f
}

func (m Money) LessThan(other Money) bool {
	return m.d.LessThan(other.d)
}

func (m Money) LessThanOrEqual(other Money) bool {
	return m.d.LessThanOrEqual(other.d)
}

func (m Money) IsZero() bool {
	return m.d.IsZero()
}

func (m Money) IsPositive() bool {
	return m.d.IsPositive()
}

func (m Money) IsNegative() bool {
	return m.d.IsNegative()
}

func (m Money) IntPart() int64 {
	return m.d.IntPart()
}

func (m Money) MarshalJSON() ([]byte, error) {
	s := m.d.Round(2).StringFixed(2)
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil, err
	}
	f, _ := d.Float64()
	return json.Marshal(f)
}

func (m *Money) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		m.d = decimal.Zero
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		d, err := decimal.NewFromString(s)
		if err != nil {
			return fmt.Errorf("money: unmarshal string: %w", err)
		}
		m.d = d.Round(2)
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	m.d = decimal.NewFromFloat(f).Round(2)
	return nil
}

func (m Money) Value() (driver.Value, error) {
	if m.IsZero() {
		return "0.00", nil
	}
	return m.d.StringFixed(2), nil
}

func (m *Money) Scan(src interface{}) error {
	if src == nil {
		m.d = decimal.Zero
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return m.scanString(string(v))
	case string:
		return m.scanString(v)
	case float64:
		m.d = decimal.NewFromFloat(v).Round(2)
		return nil
	case float32:
		m.d = decimal.NewFromFloat(float64(v)).Round(2)
		return nil
	case int64:
		m.d = decimal.NewFromInt(v)
		return nil
	default:
		return fmt.Errorf("money: unsupported Scan source %T", src)
	}
}

func (m *Money) scanString(s string) error {
	if s == "" {
		m.d = decimal.Zero
		return nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	m.d = d.Round(2)
	return nil
}