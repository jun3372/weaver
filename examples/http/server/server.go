package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jun3372/weaver"
)

// option 为动态配置字段,与 HTTPServer 共用 conf key "http":
// yaml 同一段落中 Addr 由服务组件消费,Message 由本组件消费。
type option struct {
	Message string
}

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.WithConfig[option] `conf:"http"`
	weaver.HTTPServer         `conf:"http"`
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("http server init", "conf", i.WithConfig.Config())
	return nil
}

// Start 只需挂载 handler,监听与优雅关闭由 weaver.HTTPServer 负责;
// Message 每次请求实时读取,配置热更新后无需重启立即生效。
func (i *impl) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", i.handle)
	return i.HTTPServer.Serve(ctx, mux)
}

func (i *impl) handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%s\n", i.WithConfig.Config().Message)
}
