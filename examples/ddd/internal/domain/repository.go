package domain

import "context"

// UserRepository 是用户仓储端口,定义在领域层,由基础设施层实现。
type UserRepository interface {
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByEmail(ctx context.Context, email Email) (*User, error)
	Save(ctx context.Context, u *User) (*User, error)
	List(ctx context.Context, offset, limit int) ([]*User, int64, error)
}

// OrderRepository 是订单仓储端口。
type OrderRepository interface {
	FindByID(ctx context.Context, id int64) (*Order, error)
	Save(ctx context.Context, o *Order) (*Order, error)
	ListByUser(ctx context.Context, userID int64, offset, limit int) ([]*Order, int64, error)
}
