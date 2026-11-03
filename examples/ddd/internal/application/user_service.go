package application

import (
	"context"

	"github.com/pkg/errors"

	"github.com/jun3372/weaver"
	"github.com/jun3372/weaver/examples/ddd/internal/domain"
	"github.com/jun3372/weaver/examples/ddd/internal/infrastructure/persistence"
)

// UserService 是用户用例入口。
type UserService interface {
	Create(ctx context.Context, name, email string) (*domain.User, error)
	Get(ctx context.Context, id int64) (*domain.User, error)
	List(ctx context.Context, offset, limit int) ([]*domain.User, int64, error)
}

type userService struct {
	weaver.Implements[UserService]
	repoRef weaver.Ref[persistence.UserRepository]
	repo    domain.UserRepository
}

// Init 把组件引用提升为领域端口,业务方法只依赖 domain.UserRepository。
func (s *userService) Init(context.Context) error {
	s.repo = s.repoRef.Get()
	return nil
}

func (s *userService) Create(ctx context.Context, name, email string) (*domain.User, error) {
	addr, err := domain.NewEmail(email)
	if err != nil {
		return nil, err
	}

	if _, err := s.repo.FindByEmail(ctx, addr); err == nil {
		return nil, domain.ErrEmailAlreadyExists
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, errors.WithMessage(err, "检查邮箱是否已存在")
	}

	u, err := domain.NewUser(name, addr)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.Save(ctx, u)
	if err != nil {
		return nil, errors.WithMessage(err, "保存用户")
	}
	return saved, nil
}

func (s *userService) Get(ctx context.Context, id int64) (*domain.User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.WithMessage(err, "查询用户")
	}
	return u, nil
}

func (s *userService) List(ctx context.Context, offset, limit int) ([]*domain.User, int64, error) {
	users, total, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, errors.WithMessage(err, "查询用户列表")
	}
	return users, total, nil
}
