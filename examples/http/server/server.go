package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jun3372/weaver"
)

// option 中 Addr 在 Start 时读取(修改需重启监听);
// Message 每次请求实时读取,配置热更新后无需重启立即生效。
type option struct {
	Addr    string
	Message string
}

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.WithConfig[option] `conf:"http"`
	srv                       *http.Server
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("http server init", "conf", i.Config())
	return nil
}

// Start 启动 HTTP 监听并长驻阻塞;ctx 结束后优雅关闭。
// 监听地址固定,运行期配置热更新只影响处理器读取的动态字段(如 Message)。
func (i *impl) Start(ctx context.Context) error {
	addr := i.Config().Addr
	mux := http.NewServeMux()
	mux.HandleFunc("/", i.handle)
	i.srv = &http.Server{Addr: addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() { errCh <- i.srv.ListenAndServe() }()

	i.Logger(ctx).Info("http server listening", "addr", addr)

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			i.Logger(ctx).Error("http server exited", "err", err)
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := i.srv.Shutdown(shutdownCtx); err != nil {
		i.Logger(ctx).Error("http server shutdown failed", "err", err)
		return err
	}
	i.Logger(ctx).Info("http server shutdown")
	return nil
}

func (i *impl) handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%s\n", i.Config().Message)
}
