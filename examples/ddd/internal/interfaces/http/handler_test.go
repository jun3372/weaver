package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jun3372/weaver/examples/ddd/internal/application"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

type fakeUsers struct {
	user  *domain.User
	total int64
	err   error
}

func (f *fakeUsers) Create(context.Context, string, string) (*domain.User, error) {
	return f.user, f.err
}

func (f *fakeUsers) Get(context.Context, int64) (*domain.User, error) {
	return f.user, f.err
}

func (f *fakeUsers) List(context.Context, int, int) ([]*domain.User, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return []*domain.User{f.user}, f.total, nil
}

type fakeOrders struct {
	order *domain.Order
	total int64
	err   error
}

func (f *fakeOrders) Place(context.Context, int64, []application.PlaceOrderItem) (*domain.Order, error) {
	return f.order, f.err
}

func (f *fakeOrders) Get(context.Context, int64) (*domain.Order, error) {
	return f.order, f.err
}

func (f *fakeOrders) ListByUser(context.Context, int64, int, int) ([]*domain.Order, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return []*domain.Order{f.order}, f.total, nil
}

func (f *fakeOrders) Pay(context.Context, int64) (*domain.Order, error) {
	return f.order, f.err
}

func (f *fakeOrders) Cancel(context.Context, int64) (*domain.Order, error) {
	return f.order, f.err
}

type envelopeResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func doRequest(t *testing.T, h *handler, method, target, body string) (*httptest.ResponseRecorder, envelopeResponse) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.routes().ServeHTTP(rec, req)

	var env envelopeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, rec.Body.String())
	}
	return rec, env
}

func newTestHandler(users application.UserService, orders application.OrderService) *handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newHandler(users, orders, log)
}

func sampleUser(t *testing.T) *domain.User {
	t.Helper()
	email, err := domain.NewEmail("zhangsan@example.com")
	if err != nil {
		t.Fatal(err)
	}
	u, err := domain.NewUser("张三", email)
	if err != nil {
		t.Fatal(err)
	}
	u, err = domain.RehydrateUser(1, u.Name(), u.Email(), u.Status(), u.CreatedAt())
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func sampleOrder(t *testing.T) *domain.Order {
	t.Helper()
	unit, _ := domain.NewMoney(39900, "CNY")
	item, _ := domain.NewOrderItem("键盘", unit, 2)
	o, err := domain.NewOrder(1, []domain.OrderItem{item})
	if err != nil {
		t.Fatal(err)
	}
	o, err = domain.RehydrateOrder(1, o.UserID(), o.Items(), o.Total(), o.Status(), o.CreatedAt())
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestHealth(t *testing.T) {
	rec, env := doRequest(t, newTestHandler(&fakeUsers{}, &fakeOrders{}), http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK || env.Code != codeOK {
		t.Fatalf("health = %d/%d", rec.Code, env.Code)
	}
}

func TestCreateUser(t *testing.T) {
	h := newTestHandler(&fakeUsers{user: sampleUser(t)}, &fakeOrders{})
	rec, env := doRequest(t, h, http.MethodPost, "/api/v1/users", `{"name":"张三","email":"zhangsan@example.com"}`)
	if rec.Code != http.StatusCreated || env.Code != codeOK || !strings.Contains(string(env.Data), `"zhangsan@example.com"`) {
		t.Fatalf("create user = %d/%d data=%s", rec.Code, env.Code, env.Data)
	}
}

func TestRequestValidation(t *testing.T) {
	h := newTestHandler(&fakeUsers{user: sampleUser(t)}, &fakeOrders{})

	tests := []struct {
		name   string
		method string
		target string
		body   string
		status int
		code   int
	}{
		{"未知字段", http.MethodPost, "/api/v1/users", `{"name":"张三","email":"a@example.com","extra":1}`, http.StatusBadRequest, codeInvalid},
		{"坏 JSON", http.MethodPost, "/api/v1/users", `{`, http.StatusBadRequest, codeInvalid},
		{"空请求体", http.MethodPost, "/api/v1/users", ``, http.StatusBadRequest, codeInvalid},
		{"路径 ID 非法", http.MethodGet, "/api/v1/users/abc", ``, http.StatusBadRequest, codeInvalid},
		{"limit 越界", http.MethodGet, "/api/v1/users?limit=0", ``, http.StatusBadRequest, codeInvalid},
		{"offset 为负", http.MethodGet, "/api/v1/users?offset=-1", ``, http.StatusBadRequest, codeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, env := doRequest(t, h, tt.method, tt.target, tt.body)
			if rec.Code != tt.status || env.Code != tt.code {
				t.Fatalf("得到 %d/%d, 期望 %d/%d (message=%s)", rec.Code, env.Code, tt.status, tt.code, env.Message)
			}
		})
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		users  *fakeUsers
		orders *fakeOrders
		status int
		code   int
	}{
		{
			name: "用户不存在", method: http.MethodGet, target: "/api/v1/users/9",
			users: &fakeUsers{err: domain.ErrUserNotFound}, orders: &fakeOrders{},
			status: http.StatusNotFound, code: codeUserNotFound,
		},
		{
			name: "订单不存在", method: http.MethodGet, target: "/api/v1/orders/9",
			users: &fakeUsers{}, orders: &fakeOrders{err: domain.ErrOrderNotFound},
			status: http.StatusNotFound, code: codeOrderNotFound,
		},
		{
			name: "邮箱冲突", method: http.MethodPost, target: "/api/v1/users",
			users: &fakeUsers{err: domain.ErrEmailAlreadyExists}, orders: &fakeOrders{},
			status: http.StatusConflict, code: codeEmailExists,
		},
		{
			name: "订单状态冲突", method: http.MethodPost, target: "/api/v1/orders/1/pay",
			users: &fakeUsers{}, orders: &fakeOrders{err: domain.ErrOrderStatusConflict},
			status: http.StatusConflict, code: codeOrderConflict,
		},
		{
			name: "用户禁用", method: http.MethodPost, target: "/api/v1/users/1/orders",
			users: &fakeUsers{}, orders: &fakeOrders{err: domain.ErrUserDisabled},
			status: http.StatusConflict, code: codeUserDisabled,
		},
		{
			name: "内部错误", method: http.MethodGet, target: "/api/v1/users/1",
			users: &fakeUsers{err: errors.New("boom")}, orders: &fakeOrders{},
			status: http.StatusInternalServerError, code: codeInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.users, tt.orders)
			body := `{"items":[{"productName":"键盘","unitAmount":100,"currency":"CNY","quantity":1}]}`
			if tt.target == "/api/v1/users" {
				body = `{"name":"张三","email":"a@example.com"}`
			}
			rec, env := doRequest(t, h, tt.method, tt.target, body)
			if rec.Code != tt.status || env.Code != tt.code {
				t.Fatalf("得到 %d/%d, 期望 %d/%d (message=%s)", rec.Code, env.Code, tt.status, tt.code, env.Message)
			}
		})
	}
}

func TestListAndStatusEndpoints(t *testing.T) {
	h := newTestHandler(&fakeUsers{user: sampleUser(t), total: 1}, &fakeOrders{order: sampleOrder(t), total: 1})

	rec, env := doRequest(t, h, http.MethodGet, "/api/v1/users?offset=0&limit=10", "")
	if rec.Code != http.StatusOK || env.Code != codeOK || !strings.Contains(string(env.Data), `"total":1`) {
		t.Fatalf("list users = %d/%d data=%s", rec.Code, env.Code, env.Data)
	}

	rec, env = doRequest(t, h, http.MethodGet, "/api/v1/users/1/orders", "")
	if rec.Code != http.StatusOK || env.Code != codeOK {
		t.Fatalf("list orders = %d/%d", rec.Code, env.Code)
	}

	rec, env = doRequest(t, h, http.MethodPost, "/api/v1/orders/1/cancel", "")
	if rec.Code != http.StatusOK || env.Code != codeOK {
		t.Fatalf("cancel = %d/%d", rec.Code, env.Code)
	}
}
