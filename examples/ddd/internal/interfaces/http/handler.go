package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/pkg/errors"

	"github.com/jun3372/weaver/examples/ddd/internal/application"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

const (
	codeOK             = 0
	codeInvalid        = 40001
	codeUserNotFound   = 40401
	codeOrderNotFound  = 40402
	codeEmailExists    = 40901
	codeOrderConflict  = 40902
	codeUserDisabled   = 40903
	codeInternal       = 50000
	maxRequestBodySize = 1 << 20
)

type handler struct {
	users  application.UserService
	orders application.OrderService
	log    *slog.Logger
}

func newHandler(users application.UserService, orders application.OrderService, log *slog.Logger) *handler {
	return &handler{users: users, orders: orders, log: log}
}

func (h *handler) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /api/v1/users", h.createUser)
	mux.HandleFunc("GET /api/v1/users", h.listUsers)
	mux.HandleFunc("GET /api/v1/users/{id}", h.getUser)
	mux.HandleFunc("POST /api/v1/users/{id}/orders", h.createOrder)
	mux.HandleFunc("GET /api/v1/users/{id}/orders", h.listUserOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", h.getOrder)
	mux.HandleFunc("POST /api/v1/orders/{id}/pay", h.payOrder)
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", h.cancelOrder)
	return mux
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.WithMessagef(domain.ErrValidation, "请求体解析失败: %v", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.WithMessage(domain.ErrValidation, "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.WithMessage(domain.ErrValidation, "路径 ID 非法")
	}
	return id, nil
}

func pagination(r *http.Request) (offset, limit int, err error) {
	offset, limit = 0, 20
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, errors.WithMessage(domain.ErrValidation, "offset 非法")
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, errors.WithMessage(domain.ErrValidation, "limit 必须在 1~100 之间")
		}
	}
	return offset, limit, nil
}

func writeData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{Code: codeOK, Data: data, Message: "ok"})
}

func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	code, status, message := classify(err)
	if status >= http.StatusInternalServerError && log != nil {
		log.Error("请求处理失败", "error", err)
		message = "服务器内部错误"
	}
	writeJSON(w, status, envelope{Code: code, Message: message})
}

func classify(err error) (code, status int, message string) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return codeInvalid, http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrUserNotFound):
		return codeUserNotFound, http.StatusNotFound, err.Error()
	case errors.Is(err, domain.ErrOrderNotFound):
		return codeOrderNotFound, http.StatusNotFound, err.Error()
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return codeEmailExists, http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrOrderStatusConflict):
		return codeOrderConflict, http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrUserDisabled):
		return codeUserDisabled, http.StatusConflict, err.Error()
	default:
		return codeInternal, http.StatusInternalServerError, "服务器内部错误"
	}
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type envelope struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
}
