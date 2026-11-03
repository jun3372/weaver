package httpapi

import (
	"time"

	"github.com/jun3372/weaver/examples/ddd/internal/application"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type createOrderRequest struct {
	Items []createOrderItemRequest `json:"items"`
}

type createOrderItemRequest struct {
	ProductName string `json:"productName"`
	UnitAmount  int64  `json:"unitAmount"`
	Currency    string `json:"currency"`
	Quantity    int    `json:"quantity"`
}

func (r createOrderRequest) toCommandItems() []application.PlaceOrderItem {
	items := make([]application.PlaceOrderItem, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, application.PlaceOrderItem{
			ProductName: item.ProductName,
			UnitAmount:  item.UnitAmount,
			Currency:    item.Currency,
			Quantity:    item.Quantity,
		})
	}
	return items
}

type userResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type orderItemResponse struct {
	ProductName string `json:"productName"`
	UnitAmount  int64  `json:"unitAmount"`
	Quantity    int    `json:"quantity"`
	Amount      int64  `json:"amount"`
}

type orderResponse struct {
	ID          int64               `json:"id"`
	UserID      int64               `json:"userId"`
	Items       []orderItemResponse `json:"items"`
	TotalAmount int64               `json:"totalAmount"`
	Currency    string              `json:"currency"`
	Status      string              `json:"status"`
	CreatedAt   time.Time           `json:"createdAt"`
}

type pageResponse[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:        u.ID(),
		Name:      u.Name(),
		Email:     u.Email().String(),
		Status:    string(u.Status()),
		CreatedAt: u.CreatedAt(),
	}
}

func toUserResponses(users []*domain.User) []userResponse {
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}
	return out
}

func toOrderResponse(o *domain.Order) orderResponse {
	items := make([]orderItemResponse, 0, len(o.Items()))
	for _, item := range o.Items() {
		amount, _ := item.Amount()
		items = append(items, orderItemResponse{
			ProductName: item.ProductName(),
			UnitAmount:  item.UnitAmount().Amount(),
			Quantity:    item.Quantity(),
			Amount:      amount.Amount(),
		})
	}
	return orderResponse{
		ID:          o.ID(),
		UserID:      o.UserID(),
		Items:       items,
		TotalAmount: o.Total().Amount(),
		Currency:    o.Total().Currency(),
		Status:      string(o.Status()),
		CreatedAt:   o.CreatedAt(),
	}
}

func toOrderResponses(orders []*domain.Order) []orderResponse {
	out := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, toOrderResponse(o))
	}
	return out
}
