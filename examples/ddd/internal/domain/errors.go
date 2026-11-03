package domain

import "github.com/pkg/errors"

// 领域错误统一在此定义,交付层通过 errors.Is 映射为 HTTP 错误码。
var (
	ErrValidation          = errors.New("参数校验失败")
	ErrUserNotFound        = errors.New("用户不存在")
	ErrOrderNotFound       = errors.New("订单不存在")
	ErrEmailAlreadyExists  = errors.New("邮箱已存在")
	ErrUserDisabled        = errors.New("用户已禁用")
	ErrOrderStatusConflict = errors.New("订单状态不允许该操作")
)
