package persistence

import (
	"context"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	dbcomp "github.com/jun3372/weaver/components/db"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"

	"github.com/jun3372/weaver"
)

// UserRepository 与实现同包:weaver 生成器要求 Implements[T] 的接口与实现位于同一包。
// 内嵌 domain.UserRepository 保证实现方法集与领域端口完全一致。
type UserRepository interface {
	domain.UserRepository
}

type userRepository struct {
	weaver.Implements[UserRepository]
	dbRef weaver.Ref[dbcomp.T]
	gdb   *gorm.DB
}

func (r *userRepository) Init(ctx context.Context) error {
	gdb, err := r.dbRef.Get().WithContext(ctx)
	if err != nil {
		return errors.WithMessage(err, "persistence: 获取数据库连接")
	}
	if err := gdb.AutoMigrate(&userModel{}); err != nil {
		return errors.WithMessage(err, "persistence: 迁移用户表")
	}
	r.gdb = gdb
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	var m userModel
	if err := r.gdb.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, errors.WithMessage(err, "查询用户")
	}
	return toDomainUser(m)
}

func (r *userRepository) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	var m userModel
	if err := r.gdb.WithContext(ctx).Where("email = ?", email.String()).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, errors.WithMessage(err, "按邮箱查询用户")
	}
	return toDomainUser(m)
}

func (r *userRepository) Save(ctx context.Context, u *domain.User) (*domain.User, error) {
	m := toUserModel(u)
	if m.ID == 0 {
		if err := r.gdb.WithContext(ctx).Create(&m).Error; err != nil {
			return nil, errors.WithMessage(err, "创建用户")
		}
		return toDomainUser(m)
	}
	result := r.gdb.WithContext(ctx).Save(&m)
	if result.Error != nil {
		return nil, errors.WithMessage(result.Error, "更新用户")
	}
	if result.RowsAffected == 0 {
		return nil, errors.WithMessage(domain.ErrUserNotFound, "更新用户")
	}
	return toDomainUser(m)
}

func (r *userRepository) List(ctx context.Context, offset, limit int) ([]*domain.User, int64, error) {
	query := r.gdb.WithContext(ctx).Model(&userModel{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.WithMessage(err, "统计用户数")
	}

	var rows []userModel
	if err := query.Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, errors.WithMessage(err, "查询用户列表")
	}
	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		u, err := toDomainUser(row)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}
