package application

import (
	"context"
	"errors"
	"testing"

	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

type fakeUserRepo struct {
	users  map[int64]*domain.User
	nextID int64
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[int64]*domain.User{}, nextID: 1}
}

func (r *fakeUserRepo) FindByID(_ context.Context, id int64) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) FindByEmail(_ context.Context, email domain.Email) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email() == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *fakeUserRepo) Save(_ context.Context, u *domain.User) (*domain.User, error) {
	if u.ID() == 0 {
		id := r.nextID
		r.nextID++
		rebuilt, err := domain.RehydrateUser(id, u.Name(), u.Email(), u.Status(), u.CreatedAt())
		if err != nil {
			return nil, err
		}
		r.users[id] = rebuilt
		return rebuilt, nil
	}
	r.users[u.ID()] = u
	return u, nil
}

func (r *fakeUserRepo) List(_ context.Context, offset, limit int) ([]*domain.User, int64, error) {
	out := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	total := int64(len(out))
	if offset >= len(out) {
		return nil, total, nil
	}
	end := min(offset+limit, len(out))
	return out[offset:end], total, nil
}

type fakeOrderRepo struct {
	orders map[int64]*domain.Order
	nextID int64
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{orders: map[int64]*domain.Order{}, nextID: 1}
}

func (r *fakeOrderRepo) FindByID(_ context.Context, id int64) (*domain.Order, error) {
	o, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}

func (r *fakeOrderRepo) Save(_ context.Context, o *domain.Order) (*domain.Order, error) {
	if o.ID() == 0 {
		id := r.nextID
		r.nextID++
		rebuilt, err := domain.RehydrateOrder(id, o.UserID(), o.Items(), o.Total(), o.Status(), o.CreatedAt())
		if err != nil {
			return nil, err
		}
		r.orders[id] = rebuilt
		return rebuilt, nil
	}
	r.orders[o.ID()] = o
	return o, nil
}

func (r *fakeOrderRepo) ListByUser(_ context.Context, userID int64, offset, limit int) ([]*domain.Order, int64, error) {
	out := make([]*domain.Order, 0)
	for _, o := range r.orders {
		if o.UserID() == userID {
			out = append(out, o)
		}
	}
	total := int64(len(out))
	if offset >= len(out) {
		return nil, total, nil
	}
	end := min(offset+limit, len(out))
	return out[offset:end], total, nil
}

func TestUserServiceCreate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := &userService{repo: repo}

	u, err := svc.Create(ctx, "张三", "zhangsan@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID() != 1 || u.Email().String() != "zhangsan@example.com" {
		t.Fatalf("创建结果异常: %+v", u)
	}

	if _, err := svc.Create(ctx, "张三", "zhangsan@example.com"); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("重复邮箱应返回 ErrEmailAlreadyExists,得到 %v", err)
	}
	if _, err := svc.Create(ctx, "李四", "bad-email"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("非法邮箱应返回 ErrValidation,得到 %v", err)
	}
	if _, err := svc.Get(ctx, 99); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("不存在用户应返回 ErrUserNotFound,得到 %v", err)
	}
}

func TestOrderServicePlaceAndTransitions(t *testing.T) {
	ctx := context.Background()
	userRepo := newFakeUserRepo()
	userSvc := &userService{repo: userRepo}
	orderRepo := newFakeOrderRepo()
	orderSvc := &orderService{users: userSvc, orders: orderRepo}

	u, err := userSvc.Create(ctx, "张三", "zhangsan@example.com")
	if err != nil {
		t.Fatal(err)
	}

	o, err := orderSvc.Place(ctx, u.ID(), []PlaceOrderItem{
		{ProductName: "键盘", UnitAmount: 39900, Currency: "CNY", Quantity: 2},
		{ProductName: "鼠标", UnitAmount: 9900, Currency: "CNY", Quantity: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.Total().Amount() != 89700 {
		t.Fatalf("总额 = %d, 期望 89700", o.Total().Amount())
	}

	if _, err := orderSvc.Place(ctx, 99, []PlaceOrderItem{{ProductName: "键盘", UnitAmount: 1, Currency: "CNY", Quantity: 1}}); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("未知用户下单应返回 ErrUserNotFound,得到 %v", err)
	}

	if _, err := orderSvc.Pay(ctx, o.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := orderSvc.Cancel(ctx, o.ID()); !errors.Is(err, domain.ErrOrderStatusConflict) {
		t.Fatalf("已支付取消应返回 ErrOrderStatusConflict,得到 %v", err)
	}

	current, err := userRepo.FindByID(ctx, u.ID())
	if err != nil {
		t.Fatal(err)
	}
	if err := current.Disable(); err != nil {
		t.Fatal(err)
	}
	if _, err := orderSvc.Place(ctx, u.ID(), []PlaceOrderItem{{ProductName: "键盘", UnitAmount: 100, Currency: "CNY", Quantity: 1}}); !errors.Is(err, domain.ErrUserDisabled) {
		t.Fatalf("禁用用户下单应返回 ErrUserDisabled,得到 %v", err)
	}
}
