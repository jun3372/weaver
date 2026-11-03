package domain

import (
	"strings"

	"github.com/pkg/errors"
)

// Email 是邮箱值对象,只允许经 NewEmail 构造,保证内部值始终合法。
type Email struct {
	value string
}

func NewEmail(raw string) (Email, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > 254 {
		return Email{}, errors.WithMessage(ErrValidation, "邮箱长度非法")
	}
	if strings.ContainsAny(s, " \t\r\n") || strings.Count(s, "@") != 1 {
		return Email{}, errors.WithMessagef(ErrValidation, "邮箱格式非法 %q", raw)
	}
	local, host, _ := strings.Cut(s, "@")
	if local == "" || host == "" || !strings.Contains(host, ".") {
		return Email{}, errors.WithMessage(ErrValidation, "邮箱格式非法")
	}
	return Email{value: s}, nil
}

func (e Email) String() string {
	return e.value
}
