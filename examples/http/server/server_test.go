package server

// 业务组件分层测试示范:
//  1. 纯业务逻辑:不启动框架,直接构造组件 + SetConfig 注入配置 + httptest 打 handler
//  2. 热更新语义:SetConfig 前后 handler 输出立即变化
//  3. 服务组件集成:Start 监听 :0,发真实请求,再验证优雅关闭

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jun3372/weaver"
)

// 层次 1:业务逻辑单测 —— 组件只是普通结构体,不依赖框架运行时。
// 注意:组件同时内嵌 WithConfig 与 HTTPServer 时,SetConfig 被提升为二义方法,
// 必须用显式选择器 i.WithConfig.SetConfig / i.HTTPServer.SetConfig 区分。
func TestHandleReturnsConfiguredMessage(t *testing.T) {
	var i impl
	i.WithConfig.SetConfig(option{Message: "unit-test"})

	rec := httptest.NewRecorder()
	i.handle(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Body.String(); got != "unit-test\n" {
		t.Errorf("响应体 = %q, 期望 %q", got, "unit-test\n")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d", rec.Code)
	}
}

// 层次 2:热更新语义 —— 配置替换后 handler 立即读到新值,无需重建组件。
func TestHandleReflectsHotReload(t *testing.T) {
	var i impl
	i.WithConfig.SetConfig(option{Message: "v1"})

	rec := httptest.NewRecorder()
	i.handle(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Body.String(); got != "v1\n" {
		t.Fatalf("热更新前响应体 = %q", got)
	}

	i.WithConfig.SetConfig(option{Message: "v2"})
	rec = httptest.NewRecorder()
	i.handle(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Body.String(); got != "v2\n" {
		t.Fatalf("热更新后响应体 = %q, 期望立即生效", got)
	}
}

// 层次 3:服务组件集成 —— Start 长驻监听 :0,真实请求,取消 ctx 后优雅退出。
func TestHTTPServerServeIntegration(t *testing.T) {
	var i impl
	i.WithConfig.SetConfig(option{Message: "integration"})
	i.HTTPServer.SetConfig(weaver.HTTPOption{Addr: "127.0.0.1:0"})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- i.Start(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for i.Addr() == "" {
		if time.Now().After(deadline) {
			t.Fatal("服务启动超时")
		}
		time.Sleep(5 * time.Millisecond)
	}

	resp, err := http.Get("http://" + i.Addr() + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "integration\n" {
		t.Errorf("响应体 = %q, 期望 %q", body, "integration\n")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("优雅关闭应返回 nil,得到 %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("服务关闭超时")
	}
}
