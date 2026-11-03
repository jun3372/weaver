package persistence

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	dbcomp "github.com/jun3372/weaver/components/db"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"

	"github.com/jun3372/weaver"
)

// OrderRepository 与实现同包,内嵌领域端口。
type OrderRepository interface {
	domain.OrderRepository
}

type orderRepository struct {
	weaver.Implements[OrderRepository]
	dbRef weaver.Ref[dbcomp.T]
	gdb   *gorm.DB
}

func (r *orderRepository) Init(ctx context.Context) error {
	gdb, err := r.dbRef.Get().WithContext(ctx)
	if err != nil {
		return errors.WithMessage(err, "persistence: 获取数据库连接")
	}
	if err := gdb.AutoMigrate(&orderModel{}); err != nil {
		return errors.WithMessage(err, "persistence: 迁移订单表")
	}
	r.gdb = gdb
	return nil
}

func (r *orderRepository) FindByID(ctx context.Context, id int64) (*domain.Order, error) {
	var m orderModel
	if err := r.gdb.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, errors.WithMessage(err, "查询订单")
	}
	return toDomainOrder(m)
}

func (r *orderRepository) Save(ctx context.Context, o *domain.Order) (*domain.Order, error) {
	m := toOrderModel(o)
	if m.ID == 0 {
		if err := r.gdb.WithContext(ctx).Create(&m).Error; err != nil {
			return nil, errors.WithMessage(err, "创建订单")
		}
		return toDomainOrder(m)
	}
	result := r.gdb.WithContext(ctx).Save(&m)
	if result.Error != nil {
		return nil, errors.WithMessage(result.Error, "更新订单")
	}
	if result.RowsAffected == 0 {
		return nil, errors.WithMessage(domain.ErrOrderNotFound, "更新订单")
	}
	return toDomainOrder(m)
}

func (r *orderRepository) ListByUser(ctx context.Context, userID int64, offset, limit int) ([]*domain.Order, int64, error) {
	query := r.gdb.WithContext(ctx).Model(&orderModel{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithMessage(err, "统计用户订单数")
	}

	var rows []orderModel
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, errors.WithMessage(err, "查询用户订单列表")
	}
	orders := make([]*domain.Order, 0, len(rows))
	for _, row := range rows {
		o, err := toDomainOrder(row)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, nil
}
