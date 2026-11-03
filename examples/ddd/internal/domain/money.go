package domain

import (
	"strings"

	"github.com/pkg/errors"
)

// Money 是金额值对象,最小单位为分,避免浮点误差。
type Money struct {
	amount   int64
	currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if amount <= 0 {
		return Money{}, errors.WithMessage(ErrValidation, "金额必须大于 0")
	}
	if len(cur) != 3 || !isUpperAlpha(cur) {
		return Money{}, errors.WithMessage(ErrValidation, "币种必须是 3 位字母")
	}
	return Money{amount: amount, currency: cur}, nil
}

func isUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }

func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, errors.WithMessage(ErrValidation, "币种不一致")
	}
	return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}

func (m Money) Multiply(n int) (Money, error) {
	if n <= 0 {
		return Money{}, errors.WithMessage(ErrValidation, "数量必须大于 0")
	}
	return Money{amount: m.amount * int64(n), currency: m.currency}, nil
}
