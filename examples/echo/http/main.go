package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jun3372/weaver"
)

// option 为动态配置字段,与 Listener 共用 conf key "listener"。
type option struct {
	Greeting string
}

type T interface{}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[T]
}

// echo 内嵌 Listener 即可:在 Init 中通过 Mux() 注册多路由,或 Set 外部框架
// 引擎(gin/echo 等一切实现 http.Handler 的类型);配置注入、监听、优雅关闭
// 全部由框架负责,无需实现 ServeHTTP 与 Start。
type echo struct {
	weaver.Implements[T]
	weaver.Listener[weaver.Handler] `conf:"listener"`
	weaver.WithConfig[option]       `conf:"listener"`
}

func (e *echo) Init(ctx context.Context) error {
	mux := http.NewServeMux()
	// Greeting 每次请求实时读取,配置热更新后无需重启立即生效
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "%s\n", e.WithConfig.Config().Greeting)
	})
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "pong")
	})

	e.Handler(mux)
	return nil
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("http echo 已启动,监听 :8080")
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
