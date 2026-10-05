package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jun3372/weaver"
)

// option 为动态配置字段,与 Listener 共用 conf key "http":
// yaml 同一段落中 Addr 由 listener 消费,Message 由本组件消费。
type option struct {
	Message string
}

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.WithConfig[option]       `conf:"http"`
	weaver.Listener[weaver.Handler] `conf:"http"`
}

// Init 注册 handler 即可,监听与优雅关闭由 Listener 负责,无需 Start;
// Message 每次请求实时读取,配置热更新后无需重启立即生效。
func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("http server init", "conf", i.WithConfig.Config())
	mux := http.NewServeMux()
	mux.HandleFunc("/", i.handle)
	i.Listener.Handler(mux)
	return nil
}

func (i *impl) handle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%s\n", i.WithConfig.Config().Message)
}
