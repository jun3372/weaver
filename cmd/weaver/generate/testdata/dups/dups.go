// Package dups 是 generate 失败路径测试夹具:同一个组件接口被两个实现结构体
// 重复实现,生成器会报 "Duplicate implementation" 并中止生成。
package dups

import (
	"context"

	"github.com/jun3372/weaver"
)

type Svc interface {
	Ping(ctx context.Context) (string, error)
}

type firstImpl struct{ weaver.Implements[Svc] }

func (firstImpl) Ping(ctx context.Context) (string, error) { return "first", nil }

type secondImpl struct{ weaver.Implements[Svc] }

func (secondImpl) Ping(ctx context.Context) (string, error) { return "second", nil }
