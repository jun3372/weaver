package weaver

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func waitAddr(t *testing.T, get func() string) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if addr := get(); addr != "" {
			return addr
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("server 未能在 3 秒内完成监听")
	return ""
}

func requireServeReturn(t *testing.T, errCh <-chan error, what string) {
	t.Helper()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("%s: Serve 应优雅返回 nil,实际: %v", what, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("%s: Serve 未能在 3 秒内随 ctx 取消返回", what)
	}
}

func okHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func TestHTTPServerServeAndGracefulShutdown(t *testing.T) {
	var srv HTTPServer
	srv.SetConfig(HTTPOption{Addr: ":0"})

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errCh <- srv.Serve(ctx, mux) }()

	addr := waitAddr(t, srv.Addr)
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("响应异常: status=%d body=%q", resp.StatusCode, body)
	}

	cancel()
	requireServeReturn(t, errCh, "HTTP")

	// Serve 返回后运行态应清空:Addr 归零,且可重新 Serve
	if addr := srv.Addr(); addr != "" {
		t.Fatalf("Serve 返回后 Addr 应为空,实际 %q", addr)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	errCh2 := make(chan error, 1)
	go func() { errCh2 <- srv.Serve(ctx2, mux) }()
	waitAddr(t, srv.Addr)
	cancel2()
	requireServeReturn(t, errCh2, "HTTP retry")

	// 关闭后端口应释放:重新绑定同地址族应成功
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("关闭后端口未释放: %v", err)
	}
	_ = ln.Close()
}

func TestHTTPServerRequiresAddr(t *testing.T) {
	var srv HTTPServer
	err := srv.Serve(context.Background(), http.NewServeMux())
	if err == nil || !strings.Contains(err.Error(), "未配置监听地址") {
		t.Fatalf("空 Addr 应返回明确错误,实际: %v", err)
	}
}

func TestHTTPServerDoubleServe(t *testing.T) {
	var srv HTTPServer
	srv.SetConfig(HTTPOption{Addr: ":0"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx, http.NewServeMux()) }()
	waitAddr(t, srv.Addr)

	if err := srv.Serve(ctx, http.NewServeMux()); err == nil {
		t.Fatal("重复 Serve 应返回错误")
	}
	cancel()
	requireServeReturn(t, errCh, "HTTP")
}

func TestHTTPServerListenError(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var srv HTTPServer
	srv.SetConfig(HTTPOption{Addr: ln.Addr().String()})
	if err := srv.Serve(context.Background(), http.NewServeMux()); err == nil {
		t.Fatal("端口占用时 Serve 应返回错误")
	}

	// listen 失败后修正配置应可重试
	srv.SetConfig(HTTPOption{Addr: ":0"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx, okHandler()) }()
	waitAddr(t, srv.Addr)
	cancel()
	requireServeReturn(t, errCh, "HTTP retry after listen failure")
}

type echoTCPHandler struct{}

func (echoTCPHandler) ServeTCP(_ context.Context, conn net.Conn) {
	buf := make([]byte, 512)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		if _, err := conn.Write(buf[:n]); err != nil {
			return
		}
	}
}

func TestTCPServerEchoAndGracefulShutdown(t *testing.T) {
	var srv TCPServer
	srv.SetConfig(TCPOption{Addr: ":0"})

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errCh <- srv.Serve(ctx, echoTCPHandler{}) }()

	conn, err := net.Dial("tcp", waitAddr(t, srv.Addr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("echo 内容异常: %q", buf)
	}

	cancel()
	requireServeReturn(t, errCh, "TCP")

	// Serve 返回后 Addr 归零,且可重新 Serve
	if addr := srv.Addr(); addr != "" {
		t.Fatalf("Serve 返回后 Addr 应为空,实际 %q", addr)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	errCh2 := make(chan error, 1)
	go func() { errCh2 <- srv.Serve(ctx2, echoTCPHandler{}) }()
	waitAddr(t, srv.Addr)
	cancel2()
	requireServeReturn(t, errCh2, "TCP retry")

	// 关闭后连接应被框架回收:Read 应返回错误
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("服务关闭后连接应被关闭")
	}
}

func TestTCPServerRequiresAddr(t *testing.T) {
	var srv TCPServer
	if err := srv.Serve(context.Background(), echoTCPHandler{}); err == nil ||
		!strings.Contains(err.Error(), "未配置监听地址") {
		t.Fatalf("空 Addr 应返回明确错误,实际: %v", err)
	}
}

func TestTCPServerRetryAfterListenFailure(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var srv TCPServer
	srv.SetConfig(TCPOption{Addr: ln.Addr().String()})
	if err := srv.Serve(context.Background(), echoTCPHandler{}); err == nil {
		t.Fatal("端口占用时 Serve 应返回错误")
	}

	srv.SetConfig(TCPOption{Addr: ":0"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx, echoTCPHandler{}) }()
	waitAddr(t, srv.Addr)
	cancel()
	requireServeReturn(t, errCh, "TCP retry after listen failure")
}

type echoUDPHandler struct{}

func (echoUDPHandler) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	return pkt.Data, nil
}

func TestUDPServerEcho(t *testing.T) {
	var srv UDPServer
	srv.SetConfig(UDPOption{Addr: ":0"})

	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errCh <- srv.Serve(ctx, echoUDPHandler{}) }()

	conn, err := net.Dial("udp", waitAddr(t, srv.Addr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Fatalf("echo 内容异常: %q", buf[:n])
	}

	cancel()
	requireServeReturn(t, errCh, "UDP")

	// Serve 返回后 Addr 归零,且可重新 Serve
	if addr := srv.Addr(); addr != "" {
		t.Fatalf("Serve 返回后 Addr 应为空,实际 %q", addr)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	errCh2 := make(chan error, 1)
	go func() { errCh2 <- srv.Serve(ctx2, echoUDPHandler{}) }()
	waitAddr(t, srv.Addr)
	cancel2()
	requireServeReturn(t, errCh2, "UDP retry")
}

// 同一组件声明多个服务实例(不同 conf key)应可并发服务。
func TestMultipleServersConcurrently(t *testing.T) {
	var (
		api   HTTPServer
		admin HTTPServer
	)
	api.SetConfig(HTTPOption{Addr: ":0"})
	admin.SetConfig(HTTPOption{Addr: ":0"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	apiErr := make(chan error, 1)
	adminErr := make(chan error, 1)
	go func() { apiErr <- api.Serve(ctx, okHandler()) }()
	go func() { adminErr <- admin.Serve(ctx, okHandler()) }()

	apiAddr := waitAddr(t, api.Addr)
	adminAddr := waitAddr(t, admin.Addr)
	if apiAddr == adminAddr {
		t.Fatal("两个实例应绑定不同地址")
	}

	for _, addr := range []string{apiAddr, adminAddr} {
		resp, err := http.Get("http://" + addr)
		if err != nil {
			t.Fatalf("GET %s: %v", addr, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status=%d", addr, resp.StatusCode)
		}
	}

	cancel()
	requireServeReturn(t, apiErr, "HTTP api")
	requireServeReturn(t, adminErr, "HTTP admin")
}
