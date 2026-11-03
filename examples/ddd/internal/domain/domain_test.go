package domain

import (
	"errors"
	"strings"
	"testing"
)

func mustEmail(t *testing.T, raw string) Email {
	t.Helper()
	e, err := NewEmail(raw)
	if err != nil {
		t.Fatalf("NewEmail(%q): %v", raw, err)
	}
	return e
}

func TestNewEmail(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		ok   bool
		want string
	}{
		{"合法并归一化", " User@Example.COM ", true, "user@example.com"},
		{"缺少 @", "userexample.com", false, ""},
		{"多个 @", "a@b@c.com", false, ""},
		{"主机无点", "user@localhost", false, ""},
		{"空白", "   ", false, ""},
		{"超长", strings.Repeat("a", 250) + "@example.com", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewEmail(tt.raw)
			if tt.ok {
				if err != nil {
					t.Fatalf("期望成功,得到 %v", err)
				}
				if got.String() != tt.want {
					t.Fatalf("归一化 = %q, 期望 %q", got.String(), tt.want)
				}
				return
			}
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("期望 ErrValidation,得到 %v", err)
			}
		})
	}
}

func TestMoney(t *testing.T) {
	m, err := NewMoney(100, "cny")
	if err != nil {
		t.Fatal(err)
	}
	if m.Amount() != 100 || m.Currency() != "CNY" {
		t.Fatalf("金额 = %d %s", m.Amount(), m.Currency())
	}

	other, _ := NewMoney(50, "CNY")
	sum, err := m.Add(other)
	if err != nil || sum.Amount() != 150 {
		t.Fatalf("Add = %v, %v", sum, err)
	}
	product, err := m.Multiply(3)
	if err != nil || product.Amount() != 300 {
		t.Fatalf("Multiply = %v, %v", product, err)
	}

	usd, _ := NewMoney(1, "USD")
	if _, err := m.Add(usd); !errors.Is(err, ErrValidation) {
		t.Fatalf("币种不一致应返回 ErrValidation,得到 %v", err)
	}
	if _, err := NewMoney(0, "CNY"); !errors.Is(err, ErrValidation) {
		t.Fatalf("零金额应返回 ErrValidation,得到 %v", err)
	}
	if _, err := m.Multiply(0); !errors.Is(err, ErrValidation) {
		t.Fatalf("零数量应返回 ErrValidation,得到 %v", err)
	}
}

func TestUser(t *testing.T) {
	u, err := NewUser(" 张三 ", mustEmail(t, "zhangsan@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Name() != "张三" || !u.CanPlaceOrder() {
		t.Fatalf("用户创建异常: %+v", u)
	}
	if err := u.Disable(); err != nil {
		t.Fatal(err)
	}
	if err := u.Disable(); err != nil {
		t.Fatalf("Disable 应幂等: %v", err)
	}
	if u.CanPlaceOrder() {
		t.Fatal("禁用用户不应允许下单")
	}
	if _, err := NewUser("", mustEmail(t, "a@example.com")); !errors.Is(err, ErrValidation) {
		t.Fatalf("空用户名应返回 ErrValidation,得到 %v", err)
	}
}

func mustOrder(t *testing.T) *Order {
	t.Helper()
	unit1, _ := NewMoney(100, "CNY")
	unit2, _ := NewMoney(250, "CNY")
	item1, err := NewOrderItem("键盘", unit1, 2)
	if err != nil {
		t.Fatal(err)
	}
	item2, err := NewOrderItem("鼠标", unit2, 1)
	if err != nil {
		t.Fatal(err)
	}
	o, err := NewOrder(1, []OrderItem{item1, item2})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOrderTotalAndStatus(t *testing.T) {
	o := mustOrder(t)
	if o.Total().Amount() != 450 || o.Total().Currency() != "CNY" {
		t.Fatalf("总额 = %d %s, 期望 450 CNY", o.Total().Amount(), o.Total().Currency())
	}
	if o.Status() != OrderPending {
		t.Fatalf("初始状态 = %s", o.Status())
	}

	if err := o.Pay(); err != nil {
		t.Fatal(err)
	}
	if o.Status() != OrderPaid {
		t.Fatalf("支付后状态 = %s", o.Status())
	}
	if err := o.Cancel(); !errors.Is(err, ErrOrderStatusConflict) {
		t.Fatalf("已支付取消应冲突,得到 %v", err)
	}
	if err := o.Pay(); !errors.Is(err, ErrOrderStatusConflict) {
		t.Fatalf("重复支付应冲突,得到 %v", err)
	}
}

func TestOrderValidation(t *testing.T) {
	unit, _ := NewMoney(100, "CNY")
	item, _ := NewOrderItem("键盘", unit, 1)

	if _, err := NewOrder(0, []OrderItem{item}); !errors.Is(err, ErrValidation) {
		t.Fatalf("用户 ID 为 0 应返回 ErrValidation,得到 %v", err)
	}
	if _, err := NewOrder(1, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("空订单项应返回 ErrValidation,得到 %v", err)
	}
	if _, err := NewOrderItem("", unit, 1); !errors.Is(err, ErrValidation) {
		t.Fatalf("空商品名应返回 ErrValidation,得到 %v", err)
	}
	if _, err := NewOrderItem("键盘", unit, 0); !errors.Is(err, ErrValidation) {
		t.Fatalf("零数量应返回 ErrValidation,得到 %v", err)
	}
}
