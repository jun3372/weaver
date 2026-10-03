package weaver

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitAddr(t testing.TB, get func() string) string {
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

// startTCPServe 启动 TCPServer 并等待监听就绪,返回停止函数。
func startTCPServe(t *testing.T, opt TCPOption, h TCPHandler) (*TCPServer, func()) {
	t.Helper()
	var s TCPServer
	s.SetConfig(opt)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, h) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.Addr() != "" {
			return &s, func() {
				cancel()
				select {
				case <-errCh:
				case <-time.After(6 * time.Second):
					t.Error("TCPServer 退出超时")
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("TCPServer 启动超时")
	return nil, nil
}

func startUDPServe(t *testing.T, opt UDPOption, h UDPPacketHandler) (*UDPServer, func()) {
	t.Helper()
	var s UDPServer
	s.SetConfig(opt)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, h) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.Addr() != "" {
			return &s, func() {
				cancel()
				select {
				case <-errCh:
				case <-time.After(6 * time.Second):
					t.Error("UDPServer 退出超时")
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("UDPServer 启动超时")
	return nil, nil
}

// --- 问题 1: handler panic 不得击穿进程 ---

type panicAwareTCP struct{}

func (panicAwareTCP) ServeTCP(_ context.Context, conn net.Conn) {
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	if string(buf[:n]) == "boom" {
		panic("tcp handler panic")
	}
	_, _ = conn.Write(buf[:n])
}

func TestTCPServeRecoversHandlerPanic(t *testing.T) {
	s, stop := startTCPServe(t, TCPOption{Addr: "127.0.0.1:0"}, panicAwareTCP{})
	defer stop()

	boom, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := boom.Write([]byte("boom")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	_ = boom.Close()

	good, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatalf("panic 后服务应仍在运行: %v", err)
	}
	defer good.Close()
	if _, err := good.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	_ = good.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(good, buf); err != nil {
		t.Fatalf("panic 后回显失败: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("期望回显 ping,得到 %q", buf)
	}
}

type panicAwareUDP struct{}

func (panicAwareUDP) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	if string(pkt.Data) == "boom" {
		panic("udp handler panic")
	}
	return pkt.Data, nil
}

func TestUDPServeRecoversHandlerPanic(t *testing.T) {
	s, stop := startUDPServe(t, UDPOption{Addr: "127.0.0.1:0"}, panicAwareUDP{})
	defer stop()

	conn, err := net.Dial("udp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("boom")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("panic 后服务应仍在运行: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("期望回显 ping,得到 %q", buf)
	}
}

// --- 问题 2: HTTPServer 超时配置生效 ---

func TestHTTPServerReadHeaderTimeout(t *testing.T) {
	var s HTTPServer
	s.SetConfig(HTTPOption{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 150 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(ctx, http.NewServeMux()) }()
	defer func() {
		cancel()
		<-errCh
	}()
	deadline := time.Now().Add(3 * time.Second)
	for s.Addr() == "" {
		if time.Now().After(deadline) {
			t.Fatal("HTTPServer 启动超时")
		}
		time.Sleep(5 * time.Millisecond)
	}

	conn, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// 只发送半个请求头,不给空行,应在 ReadHeaderTimeout 后被服务端关闭
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 16)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("慢速请求头应在 ReadHeaderTimeout 后被关闭,但收到了响应数据")
	}
}

// --- 问题 3: TCP/UDP 资源上限 ---

func TestTCPServerMaxConns(t *testing.T) {
	s, stop := startTCPServe(t, TCPOption{Addr: "127.0.0.1:0", MaxConns: 1},
		discardTCP{})
	defer stop()

	first, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := first.Write([]byte("hold")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // 等待服务端 accept 并登记

	second, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	_ = second.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if _, err := second.Read(buf); err == nil {
		t.Fatal("超过 MaxConns 的新连接应被立即关闭")
	}
}

type discardTCP struct{}

func (discardTCP) ServeTCP(_ context.Context, conn net.Conn) {
	_, _ = io.Copy(io.Discard, conn)
}

func TestTCPServerConnIdleTimeout(t *testing.T) {
	s, stop := startTCPServe(t, TCPOption{
		Addr:            "127.0.0.1:0",
		ConnIdleTimeout: 150 * time.Millisecond,
	}, discardTCP{})
	defer stop()

	conn, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 保持沉默超过空闲超时后,连接应被服务端关闭
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("空闲超时后连接应被关闭")
	}

	// 活跃连接不受影响:持续读写期间 deadline 自动续期
	active, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	deadline := time.Now().Add(time.Second)
	buf = make([]byte, 1)
	for time.Now().Before(deadline) {
		if _, err := active.Write([]byte("x")); err != nil {
			t.Fatalf("活跃连接不应被空闲超时关闭: %v", err)
		}
		_ = active.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		_, _ = active.Read(buf)
		time.Sleep(50 * time.Millisecond)
	}
}

func TestUDPServerMaxConcurrent(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	h := blockingUDP{calls: &calls, started: started, release: release}

	s, stop := startUDPServe(t, UDPOption{Addr: "127.0.0.1:0", MaxConcurrent: 1}, h)
	defer stop()

	conn, err := net.Dial("udp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("one")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("第一个报文未被处理")
	}

	_, _ = conn.Write([]byte("two"))
	time.Sleep(200 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("MaxConcurrent=1 时第二个报文应被丢弃,实际处理了 %d 个", got)
	}

	close(release)
}

type blockingUDP struct {
	calls   *atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (h blockingUDP) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	h.calls.Add(1)
	select {
	case h.started <- struct{}{}:
	default:
	}
	<-h.release
	return pkt.Data, nil
}
