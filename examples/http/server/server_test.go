package server

// 业务组件分层测试示范:
//  1. 纯业务逻辑:不启动框架,直接构造组件 + SetConfig 注入配置 + httptest 打 handler
//  2. 热更新语义:SetConfig 前后 handler 输出立即变化
//
// Listener 的监听/优雅关闭等集成行为由 examples/http 的 TestMain(weaver.Run
// 全栈启动)与根包 listener_test.go 覆盖,此处不再重复。

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 层次 1:业务逻辑单测 —— 组件只是普通结构体,不依赖框架运行时。
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
