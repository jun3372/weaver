package persistence

import (
	"github.com/pkg/errors"

	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

func toUserModel(u *domain.User) userModel {
	return userModel{
		ID:        u.ID(),
		Name:      u.Name(),
		Email:     u.Email().String(),
		Status:    string(u.Status()),
		CreatedAt: u.CreatedAt(),
	}
}

func toDomainUser(m userModel) (*domain.User, error) {
	email, err := domain.NewEmail(m.Email)
	if err != nil {
		return nil, errors.WithMessage(err, "持久化用户邮箱非法")
	}
	u, err := domain.RehydrateUser(m.ID, m.Name, email, domain.UserStatus(m.Status), m.CreatedAt)
	if err != nil {
		return nil, errors.WithMessage(err, "重建用户聚合")
	}
	return u, nil
}

func toOrderModel(o *domain.Order) orderModel {
	items := make([]orderItemModel, 0, len(o.Items()))
	for _, item := range o.Items() {
		items = append(items, orderItemModel{
			ProductName: item.ProductName(),
			UnitAmount:  item.UnitAmount().Amount(),
			Currency:    item.UnitAmount().Currency(),
			Quantity:    item.Quantity(),
		})
	}
	return orderModel{
		ID:          o.ID(),
		UserID:      o.UserID(),
		Items:       items,
		TotalAmount: o.Total().Amount(),
		Currency:    o.Total().Currency(),
		Status:      string(o.Status()),
		CreatedAt:   o.CreatedAt(),
	}
}

func toDomainOrder(m orderModel) (*domain.Order, error) {
	items := make([]domain.OrderItem, 0, len(m.Items))
	for _, item := range m.Items {
		unit, err := domain.NewMoney(item.UnitAmount, item.Currency)
		if err != nil {
			return nil, errors.WithMessagef(err, "持久化订单项 %s 单价非法", item.ProductName)
		}
		orderItem, err := domain.NewOrderItem(item.ProductName, unit, item.Quantity)
		if err != nil {
			return nil, errors.WithMessagef(err, "持久化订单项 %s 非法", item.ProductName)
		}
		items = append(items, orderItem)
	}

	total, err := domain.NewMoney(m.TotalAmount, m.Currency)
	if err != nil {
		return nil, errors.WithMessage(err, "持久化订单总额非法")
	}
	o, err := domain.RehydrateOrder(m.ID, m.UserID, items, total, domain.OrderStatus(m.Status), m.CreatedAt)
	if err != nil {
		return nil, errors.WithMessage(err, "重建订单聚合")
	}
	return o, nil
}
