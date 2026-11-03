package persistence

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/jun3372/weaver/examples/ddd/internal/domain"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库: %v", err)
	}
	if err := gdb.AutoMigrate(&userModel{}, &orderModel{}); err != nil {
		t.Fatalf("迁移表结构: %v", err)
	}
	return gdb
}

func mustEmail(t *testing.T, raw string) domain.Email {
	t.Helper()
	e, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestUserRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &userRepository{gdb: newTestDB(t)}

	u, err := domain.NewUser("张三", mustEmail(t, "zhangsan@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.Save(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID() == 0 {
		t.Fatal("保存后应回填 ID")
	}

	found, err := repo.FindByID(ctx, saved.ID())
	if err != nil || found.Name() != "张三" {
		t.Fatalf("FindByID = %+v, %v", found, err)
	}
	byEmail, err := repo.FindByEmail(ctx, mustEmail(t, "zhangsan@example.com"))
	if err != nil || byEmail.ID() != saved.ID() {
		t.Fatalf("FindByEmail = %+v, %v", byEmail, err)
	}

	if err := found.Disable(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Save(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := repo.FindByID(ctx, saved.ID())
	if err != nil || reloaded.Status() != domain.UserDisabled {
		t.Fatalf("状态更新未持久化: %+v, %v", reloaded, err)
	}

	users, total, err := repo.List(ctx, 0, 20)
	if err != nil || total != 1 || len(users) != 1 {
		t.Fatalf("List = %d/%d, %v", len(users), total, err)
	}

	if _, err := repo.FindByID(ctx, 99); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("缺失用户应返回 ErrUserNotFound,得到 %v", err)
	}
}

func TestOrderRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &orderRepository{gdb: newTestDB(t)}

	unit, _ := domain.NewMoney(39900, "CNY")
	item, err := domain.NewOrderItem("键盘", unit, 2)
	if err != nil {
		t.Fatal(err)
	}
	o, err := domain.NewOrder(1, []domain.OrderItem{item})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.Save(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID() == 0 {
		t.Fatal("保存后应回填 ID")
	}

	found, err := repo.FindByID(ctx, saved.ID())
	if err != nil {
		t.Fatal(err)
	}
	if found.Total().Amount() != 79800 || len(found.Items()) != 1 {
		t.Fatalf("订单往返异常: total=%d items=%d", found.Total().Amount(), len(found.Items()))
	}
	if found.Items()[0].ProductName() != "键盘" || found.Items()[0].UnitAmount().Currency() != "CNY" {
		t.Fatalf("订单项 JSON 往返异常: %+v", found.Items()[0])
	}

	if err := found.Pay(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Save(ctx, found); err != nil {
		t.Fatal(err)
	}
	paid, err := repo.FindByID(ctx, saved.ID())
	if err != nil || paid.Status() != domain.OrderPaid {
		t.Fatalf("订单状态更新未持久化: %+v, %v", paid, err)
	}

	orders, total, err := repo.ListByUser(ctx, 1, 0, 20)
	if err != nil || total != 1 || len(orders) != 1 {
		t.Fatalf("ListByUser = %d/%d, %v", len(orders), total, err)
	}
	if _, err := repo.FindByID(ctx, 99); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("缺失订单应返回 ErrOrderNotFound,得到 %v", err)
	}
}
