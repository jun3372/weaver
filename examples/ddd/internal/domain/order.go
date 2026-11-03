package domain

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderPaid      OrderStatus = "paid"
	OrderCancelled OrderStatus = "cancelled"
)

const (
	maxOrderItems     = 50
	maxItemQuantity   = 999
	maxProductNameLen = 128
)

// OrderItem 是订单项值对象。
type OrderItem struct {
	productName string
	unitAmount  Money
	quantity    int
}

func NewOrderItem(productName string, unitAmount Money, quantity int) (OrderItem, error) {
	name := strings.TrimSpace(productName)
	if name == "" || len(name) > maxProductNameLen {
		return OrderItem{}, errors.WithMessage(ErrValidation, "商品名长度必须在 1~128 之间")
	}
	if quantity <= 0 || quantity > maxItemQuantity {
		return OrderItem{}, errors.WithMessage(ErrValidation, "商品数量必须在 1~999 之间")
	}
	return OrderItem{productName: name, unitAmount: unitAmount, quantity: quantity}, nil
}

func (i OrderItem) ProductName() string { return i.productName }
func (i OrderItem) UnitAmount() Money   { return i.unitAmount }
func (i OrderItem) Quantity() int       { return i.quantity }

func (i OrderItem) Amount() (Money, error) {
	return i.unitAmount.Multiply(i.quantity)
}

// Order 是订单聚合根,总额由订单项计算得出。
type Order struct {
	id        int64
	userID    int64
	items     []OrderItem
	total     Money
	status    OrderStatus
	createdAt time.Time
}

func NewOrder(userID int64, items []OrderItem) (*Order, error) {
	if userID <= 0 {
		return nil, errors.WithMessage(ErrValidation, "用户 ID 非法")
	}
	if len(items) == 0 || len(items) > maxOrderItems {
		return nil, errors.WithMessage(ErrValidation, "订单项数量必须在 1~50 之间")
	}
	total, err := sumItems(items)
	if err != nil {
		return nil, err
	}
	return &Order{
		userID:    userID,
		items:     append([]OrderItem(nil), items...),
		total:     total,
		status:    OrderPending,
		createdAt: time.Now().UTC(),
	}, nil
}

func sumItems(items []OrderItem) (Money, error) {
	var total Money
	for idx, item := range items {
		amount, err := item.Amount()
		if err != nil {
			return Money{}, err
		}
		if idx == 0 {
			total = amount
			continue
		}
		total, err = total.Add(amount)
		if err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

// RehydrateOrder 从持久化数据重建订单,仅供仓储实现使用。
func RehydrateOrder(id, userID int64, items []OrderItem, total Money, status OrderStatus, createdAt time.Time) (*Order, error) {
	if id <= 0 || userID <= 0 || len(items) == 0 {
		return nil, errors.WithMessage(ErrValidation, "订单数据损坏")
	}
	computed, err := sumItems(items)
	if err != nil {
		return nil, err
	}
	if computed != total {
		return nil, errors.WithMessage(ErrValidation, "订单金额与明细不一致")
	}
	if status != OrderPending && status != OrderPaid && status != OrderCancelled {
		return nil, errors.WithMessage(ErrValidation, "订单状态非法")
	}
	return &Order{
		id:        id,
		userID:    userID,
		items:     append([]OrderItem(nil), items...),
		total:     total,
		status:    status,
		createdAt: createdAt,
	}, nil
}

func (o *Order) ID() int64            { return o.id }
func (o *Order) UserID() int64        { return o.userID }
func (o *Order) Total() Money         { return o.total }
func (o *Order) Status() OrderStatus  { return o.status }
func (o *Order) CreatedAt() time.Time { return o.createdAt }
func (o *Order) Items() []OrderItem   { return append([]OrderItem(nil), o.items...) }

func (o *Order) Pay() error {
	if o.status != OrderPending {
		return errors.WithMessagef(ErrOrderStatusConflict, "当前状态 %s 不能支付", o.status)
	}
	o.status = OrderPaid
	return nil
}

func (o *Order) Cancel() error {
	if o.status != OrderPending {
		return errors.WithMessagef(ErrOrderStatusConflict, "当前状态 %s 不能取消", o.status)
	}
	o.status = OrderCancelled
	return nil
}
