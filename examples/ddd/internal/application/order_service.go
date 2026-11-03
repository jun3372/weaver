package application

import (
	"context"

	"github.com/pkg/errors"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"
	"github.com/jun3372/weaver/examples/ddd/internal/infrastructure/persistence"
)

// PlaceOrderItem 是下单命令中的商品项。
type PlaceOrderItem struct {
	ProductName string
	UnitAmount  int64
	Currency    string
	Quantity    int
}

// OrderService 是订单用例入口。
type OrderService interface {
	Place(ctx context.Context, userID int64, items []PlaceOrderItem) (*domain.Order, error)
	Get(ctx context.Context, id int64) (*domain.Order, error)
	ListByUser(ctx context.Context, userID int64, offset, limit int) ([]*domain.Order, int64, error)
	Pay(ctx context.Context, id int64) (*domain.Order, error)
	Cancel(ctx context.Context, id int64) (*domain.Order, error)
}

type orderService struct {
	weaver.Implements[OrderService]
	userRef  weaver.Ref[UserService]
	orderRef weaver.Ref[persistence.OrderRepository]
	users    UserService
	orders   domain.OrderRepository
}

func (s *orderService) Init(context.Context) error {
	s.users = s.userRef.Get()
	s.orders = s.orderRef.Get()
	return nil
}

func (s *orderService) Place(ctx context.Context, userID int64, items []PlaceOrderItem) (*domain.Order, error) {
	u, err := s.users.Get(ctx, userID)
	if err != nil {
		return nil, errors.WithMessage(err, "校验下单用户")
	}
	if !u.CanPlaceOrder() {
		return nil, errors.WithMessagef(domain.ErrUserDisabled, "用户 %s", u.Email())
	}

	orderItems := make([]domain.OrderItem, 0, len(items))
	for _, item := range items {
		unit, err := domain.NewMoney(item.UnitAmount, item.Currency)
		if err != nil {
			return nil, errors.WithMessagef(err, "商品 %s 的单价非法", item.ProductName)
		}
		orderItem, err := domain.NewOrderItem(item.ProductName, unit, item.Quantity)
		if err != nil {
			return nil, errors.WithMessagef(err, "商品 %s 非法", item.ProductName)
		}
		orderItems = append(orderItems, orderItem)
	}

	o, err := domain.NewOrder(userID, orderItems)
	if err != nil {
		return nil, err
	}
	saved, err := s.orders.Save(ctx, o)
	if err != nil {
		return nil, errors.WithMessage(err, "保存订单")
	}
	return saved, nil
}

func (s *orderService) Get(ctx context.Context, id int64) (*domain.Order, error) {
	o, err := s.orders.FindByID(ctx, id)
	if err != nil {
		return nil, errors.WithMessage(err, "查询订单")
	}
	return o, nil
}

func (s *orderService) ListByUser(ctx context.Context, userID int64, offset, limit int) ([]*domain.Order, int64, error) {
	if _, err := s.users.Get(ctx, userID); err != nil {
		return nil, 0, err
	}
	orders, total, err := s.orders.ListByUser(ctx, userID, offset, limit)
	if err != nil {
		return nil, 0, errors.WithMessage(err, "查询用户订单列表")
	}
	return orders, total, nil
}

func (s *orderService) Pay(ctx context.Context, id int64) (*domain.Order, error) {
	o, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := o.Pay(); err != nil {
		return nil, err
	}
	saved, err := s.orders.Save(ctx, o)
	if err != nil {
		return nil, errors.WithMessage(err, "更新订单状态")
	}
	return saved, nil
}

func (s *orderService) Cancel(ctx context.Context, id int64) (*domain.Order, error) {
	o, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := o.Cancel(); err != nil {
		return nil, err
	}
	saved, err := s.orders.Save(ctx, o)
	if err != nil {
		return nil, errors.WithMessage(err, "更新订单状态")
	}
	return saved, nil
}
