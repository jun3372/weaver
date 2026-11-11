package weaver

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jun3372/weaver/runtime/codegen"
)

// --- Run / RunComponent 退出契约测试组件 ---
//
// 全局注册表以组件接口类型为键,weaver.Main(即 any)在整个测试二进制中
// 只能注册一个实现,故以包级变量控制 Start 行为,多个测试共用。

type runMainComp struct {
	Implements[Main]
}

var (
	runMainMu  sync.Mutex
	runMainErr error // 非 nil 时 Start 立即返回该错误;nil 时阻塞至 ctx 结束
)

func setRunMainErr(err error) {
	runMainMu.Lock()
	defer runMainMu.Unlock()
	runMainErr = err
}

func (c *runMainComp) Start(ctx context.Context) error {
	runMainMu.Lock()
	err := runMainErr
	runMainMu.Unlock()
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

func init() {
	codegen.Register(codegen.Registration{
		Name:      "testapp/runmain",
		Interface: reflect.TypeFor[Main](),
		Impl:      reflect.TypeFor[runMainComp](),
	})
}

// Start 立即返回错误时,Run 必须可靠返回该错误——不得因 goroutine 调度
// 时序漏报(回归:原实现 launched.Wait 后读局部 firstErr,goroutine 第一行
// 即 Done,Start 的错误可能晚于读取,Run 返回 nil)。多轮重复并配合
// -race 最大化暴露时序问题。
func TestRunReturnsStartErrorReliably(t *testing.T) {
	wantErr := errors.New("run-main-start-failed")

	for i := range 10 {
		setRunMainErr(wantErr)

		err := Run(context.Background(), func(ctx context.Context, _ *runMainComp) error {
			<-ctx.Done() // 模拟 RunComponent 形态:等待退出信号
			return ctx.Err()
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("第 %d 轮: Run 应返回 Start 错误 %v, got %v", i+1, wantErr, err)
		}
	}
}

// ctx 被 cancel(等价于 Ctrl-C / 组件 Exit())时,RunComponent 属正常
// 关闭路径,应归一化返回 nil。
func TestRunComponentReturnsNilOnContextCancel(t *testing.T) {
	setRunMainErr(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := RunComponent(ctx, (*runMainComp)(nil)); err != nil {
		t.Fatalf("ctx 取消后 RunComponent 应返回 nil, got %v", err)
	}
}

// 无启动错误时,Run 原样返回 app 的结果;非 cancel 类错误不得被吞掉。
func TestRunReturnsAppErrorAsIs(t *testing.T) {
	setRunMainErr(nil)

	wantErr := errors.New("app-business-error")
	err := Run(context.Background(), func(ctx context.Context, _ *runMainComp) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run 应原样返回 app 错误 %v, got %v", wantErr, err)
	}
}

// RunComponent 的归一化不得掩盖真正的错误:启动失败时即使主逻辑因
// ctx 取消返回 context.Canceled,也必须返回启动错误本身。
func TestRunComponentReturnsStartErrorNotCanceled(t *testing.T) {
	wantErr := errors.New("runcomponent-start-failed")
	setRunMainErr(wantErr)

	err := RunComponent(context.Background(), (*runMainComp)(nil))
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunComponent 应返回启动错误 %v, got %v", wantErr, err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("启动错误不得被归一化为 context.Canceled")
	}
}
