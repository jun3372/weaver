package domain

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

type UserStatus string

const (
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User 是用户聚合根。
type User struct {
	id        int64
	name      string
	email     Email
	status    UserStatus
	createdAt time.Time
}

func NewUser(name string, email Email) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return nil, errors.WithMessage(ErrValidation, "用户名长度必须在 1~64 之间")
	}
	if email.value == "" {
		return nil, errors.WithMessage(ErrValidation, "邮箱不能为空")
	}
	return &User{
		name:      name,
		email:     email,
		status:    UserActive,
		createdAt: time.Now().UTC(),
	}, nil
}

// RehydrateUser 从持久化数据重建用户,仅供仓储实现使用。
func RehydrateUser(id int64, name string, email Email, status UserStatus, createdAt time.Time) (*User, error) {
	if id <= 0 || name == "" || email.value == "" {
		return nil, errors.WithMessage(ErrValidation, "用户数据损坏")
	}
	if status != UserActive && status != UserDisabled {
		return nil, errors.WithMessage(ErrValidation, "用户状态非法")
	}
	return &User{id: id, name: name, email: email, status: status, createdAt: createdAt}, nil
}

func (u *User) ID() int64            { return u.id }
func (u *User) Name() string         { return u.name }
func (u *User) Email() Email         { return u.email }
func (u *User) Status() UserStatus   { return u.status }
func (u *User) CreatedAt() time.Time { return u.createdAt }

func (u *User) CanPlaceOrder() bool { return u.status == UserActive }

// Disable 幂等地禁用用户。
func (u *User) Disable() error {
	if u.status == UserDisabled {
		return nil
	}
	u.status = UserDisabled
	return nil
}
