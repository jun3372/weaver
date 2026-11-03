package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jun3372/weaver"
)

var (
	baseURL string
	runDone chan error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "weaver-ddd")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	conf := filepath.Join(dir, "weaver.yaml")
	cfg := fmt.Sprintf(`
app:
  Name: "ddd-e2e"
http:
  Addr: ":0"
db:
  Enabled: true
  DSN: "sqlite://file:%s/ddd.db?cache=shared&mode=rwc"
weaver:
  Logger:
    Level: "error"
    Type: "text"
`, dir)
	if err := os.WriteFile(conf, []byte(cfg), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("SERVICE_CONFIG", conf)

	// 提前解析 flags,避免与 weaver.Run 内部的 flag 注册/解析并发竞争。
	testing.Init()
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	var appPtr *app
	var mu sync.Mutex
	runDone = make(chan error, 1)
	go func() {
		runDone <- weaver.Run(ctx, func(ctx context.Context, a *app) error {
			mu.Lock()
			appPtr = a
			mu.Unlock()
			<-ctx.Done()
			return nil
		})
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		a := appPtr
		mu.Unlock()
		if a != nil {
			if addr := addrOf(a.server.Get()); addr != "" {
				baseURL = "http://" + addr
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "ddd 示例启动超时")
		cancel()
		os.Exit(1)
	}

	code := m.Run()
	cancel()
	<-runDone
	os.Exit(code)
}

func addrOf(v any) string {
	if a, ok := v.(interface{ Addr() string }); ok {
		return a.Addr()
	}
	return ""
}

type envelope struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

type userData struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type orderData struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	TotalAmount int64  `json:"totalAmount"`
	Status      string `json:"status"`
}

type pageData struct {
	Total int64 `json:"total"`
}

func call(t *testing.T, method, path, body string) (int, envelope) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("%s %s 响应解析失败: %v", method, path, err)
	}
	return resp.StatusCode, env
}

func TestE2EUserAndOrderFlow(t *testing.T) {
	status, env := call(t, http.MethodGet, "/healthz", "")
	if status != http.StatusOK || env.Code != 0 {
		t.Fatalf("healthz = %d/%d", status, env.Code)
	}

	status, env = call(t, http.MethodPost, "/api/v1/users", `{"name":"张三","email":"zhangsan@example.com"}`)
	if status != http.StatusCreated || env.Code != 0 {
		t.Fatalf("创建用户 = %d/%d %s", status, env.Code, env.Message)
	}
	var user userData
	if err := json.Unmarshal(env.Data, &user); err != nil || user.ID == 0 {
		t.Fatalf("用户返回异常: %s, %v", env.Data, err)
	}

	orderBody := `{"items":[{"productName":"键盘","unitAmount":39900,"currency":"CNY","quantity":2}]}`
	status, env = call(t, http.MethodPost, fmt.Sprintf("/api/v1/users/%d/orders", user.ID), orderBody)
	if status != http.StatusCreated || env.Code != 0 {
		t.Fatalf("下单 = %d/%d %s", status, env.Code, env.Message)
	}
	var order orderData
	if err := json.Unmarshal(env.Data, &order); err != nil || order.TotalAmount != 79800 {
		t.Fatalf("订单返回异常: %s, %v", env.Data, err)
	}

	status, env = call(t, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/pay", order.ID), "")
	if status != http.StatusOK || env.Code != 0 {
		t.Fatalf("支付 = %d/%d %s", status, env.Code, env.Message)
	}
	if err := json.Unmarshal(env.Data, &order); err != nil || order.Status != "paid" {
		t.Fatalf("支付后状态 = %s", env.Data)
	}

	status, env = call(t, http.MethodPost, fmt.Sprintf("/api/v1/orders/%d/cancel", order.ID), "")
	if status != http.StatusConflict || env.Code != 40902 {
		t.Fatalf("已支付取消 = %d/%d, 期望 409/40902", status, env.Code)
	}

	status, env = call(t, http.MethodGet, fmt.Sprintf("/api/v1/users/%d/orders", user.ID), "")
	if status != http.StatusOK || env.Code != 0 {
		t.Fatalf("查询用户订单 = %d/%d", status, env.Code)
	}
	var page pageData
	if err := json.Unmarshal(env.Data, &page); err != nil || page.Total != 1 {
		t.Fatalf("订单分页异常: %s, %v", env.Data, err)
	}
}
