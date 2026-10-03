package weaver

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/jun3372/weaver/internal/reflection"
	"github.com/jun3372/weaver/runtime/codegen"
)

type listenerIntf interface{}

// ---- 测试组件 ----

type httpEchoComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *httpEchoComp) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

// tcpSessComp 读丢弃一切,便于外部 Send/CloseConn 测试。
type tcpSessComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *tcpSessComp) ServeTCP(_ context.Context, conn net.Conn) {
	_, _ = io.Copy(io.Discard, conn)
}

type udpEchoComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *udpEchoComp) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	return append([]byte(nil), pkt.Data...), nil
}

// gatewayComp 三协议同跑:HTTP 走 httpL.Mux() 多路由,TCP/UDP 实现对应接口。
type gatewayComp struct {
	Implements[listenerIntf]
	httpL Listener[http.Handler]     `conf:"http"`
	tcpL  Listener[TCPHandler]       `conf:"tcp"`
	udpL  Listener[UDPPacketHandler] `conf:"udp"`
}

func (c *gatewayComp) Init(context.Context) error {
	c.httpL.Mux().HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("gw-http"))
	})
	return nil
}

func (c *gatewayComp) ServeTCP(_ context.Context, conn net.Conn) {
	_, _ = io.Copy(io.Discard, conn)
}

func (c *gatewayComp) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	return []byte("gw-udp"), nil
}

// muxEchoComp 通过 Listener.Mux() 注册多路由,组件不实现任何 handler 接口。
type muxEchoComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *muxEchoComp) Init(context.Context) error {
	c.Mux().HandleFunc("/hello", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	c.Mux().HandleFunc("/bye", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("bye"))
	})
	return nil
}

// engineComp 模拟接入 gin/echo 等外部框架:Init 中 Set 一个自定义引擎。
type fakeEngine struct{ msg string }

func (e fakeEngine) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(e.msg))
}

type engineComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *engineComp) Init(context.Context) error {
	c.Handler(fakeEngine{msg: "engine"})
	return nil
}

// handlerlessComp 未实现任何 handler 接口也未 Set,start 应报错。
type handlerlessComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

// badHTTPComp 声明具体接口 H 但既未实现也未 Set,start 应报错。
type badHTTPComp struct {
	Implements[listenerIntf]
	Listener[http.Handler] `conf:"listener"`
}

// ambiguousComp 实现了两种 handler 接口却只用聚合 Listener。
type ambiguousComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *ambiguousComp) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("x"))
}

func (c *ambiguousComp) ServeTCP(_ context.Context, conn net.Conn) {}

// startAndListenerComp 同时定义 Start 与 Listener。
type startAndListenerComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`
}

func (c *startAndListenerComp) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("x"))
}

func (c *startAndListenerComp) Start(context.Context) error { return nil }

// noTagComp 无 conf tag,应跳过注入与自动启动。
type noTagComp struct {
	Implements[listenerIntf]
	Listener[Handler]
}

func (c *noTagComp) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("x"))
}

// ---- 测试辅助 ----

func newListenerWidget(t testing.TB, yamlConf string, regs ...*codegen.Registration) (context.Context, context.CancelFunc, *widget) {
	t.Helper()

	confFile := filepath.Join(t.TempDir(), "weaver.yaml")
	if err := os.WriteFile(confFile, []byte(yamlConf), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := viper.New()
	conf.SetConfigFile(confFile)
	if err := conf.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return ctx, cancel, newWidget(ctx, cancel, conf, regs)
}

func listenerReg(t testing.TB, impl any) *codegen.Registration {
	t.Helper()
	return &codegen.Registration{
		Name:      "test/" + reflect.TypeOf(impl).Elem().Name(),
		Interface: reflection.Type[listenerIntf](),
		Impl:      reflect.TypeOf(impl).Elem(),
	}
}

// ---- 用例 ----

func TestListenerAutoServeHTTP(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &httpEchoComp{}))

	c, err := w.getImpl(reflect.TypeFor[httpEchoComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	addr := waitAddr(t, c.(*httpEchoComp).Addr)
	resp, err := http.Get("http://" + addr)
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("响应体 = %q, want %q", body, "ok")
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerTCPSessionsAndSend(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n  connidletimeout: 30s\n",
		listenerReg(t, &tcpSessComp{}))

	c, err := w.getImpl(reflect.TypeFor[tcpSessComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*tcpSessComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, comp.Addr)

	connA, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close()
	connB, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close()

	deadline := time.Now().Add(3 * time.Second)
	var idA, idB uint64
	for {
		if ss := comp.Sessions(); len(ss) == 2 {
			// 服务端视角的 RemoteAddr 即客户端的 LocalAddr
			for _, s := range ss {
				if s.RemoteAddr.String() == connA.LocalAddr().String() {
					idA = s.ID
				} else {
					idB = s.ID
				}
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("3 秒内未登记 2 个会话,当前 %d", len(comp.Sessions()))
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 定点发送:与 handler(io.Copy 只读)无写竞争
	_ = connA.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := comp.Send(idA, []byte("push")); err != nil {
		t.Fatalf("send: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(connA, buf); err != nil || string(buf) != "push" {
		t.Fatalf("A 收到 %q, err=%v, want push", buf, err)
	}

	// 主动断开 B
	if err := comp.CloseConn(idB); err != nil {
		t.Fatalf("close conn: %v", err)
	}
	_ = connB.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := connB.Read(buf); err == nil {
		t.Fatal("B 被服务端关闭后读取应报错")
	}

	// 不存在的会话
	if err := comp.Send(9999, []byte("x")); err == nil {
		t.Fatal("Send 不存在的会话应报错")
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerUDPPeersAndSendTo(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &udpEchoComp{}))

	c, err := w.getImpl(reflect.TypeFor[udpEchoComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*udpEchoComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, comp.Addr)

	client, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	// 回显
	if _, err := client.Write([]byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 16)
	n, err := client.Read(buf)
	if err != nil || string(buf[:n]) != "hi" {
		t.Fatalf("回显 = %q, err=%v, want hi", buf[:n], err)
	}

	// 对端表
	deadline := time.Now().Add(3 * time.Second)
	for {
		if ps := comp.Peers(); len(ps) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("3 秒内未登记对端,当前 %d", len(comp.Peers()))
		}
		time.Sleep(5 * time.Millisecond)
	}

	// 定点发送
	if err := comp.SendTo(comp.Peers()[0].Addr, []byte("push")); err != nil {
		t.Fatalf("sendto: %v", err)
	}
	n, err = client.Read(buf)
	if err != nil || string(buf[:n]) != "push" {
		t.Fatalf("定点发送收到 %q, err=%v, want push", buf[:n], err)
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerAllThreeProtocols(t *testing.T) {
	yaml := "http:\n  addr: \"127.0.0.1:0\"\n" +
		"tcp:\n  addr: \"127.0.0.1:0\"\n" +
		"udp:\n  addr: \"127.0.0.1:0\"\n"
	ctx, cancel, w := newListenerWidget(t, yaml, listenerReg(t, &gatewayComp{}))

	c, err := w.getImpl(reflect.TypeFor[gatewayComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*gatewayComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	httpAddr := waitAddr(t, comp.httpL.Addr)
	tcpAddr := waitAddr(t, comp.tcpL.Addr)
	udpAddr := waitAddr(t, comp.udpL.Addr)

	resp, err := http.Get("http://" + httpAddr)
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "gw-http" {
		t.Fatalf("http 响应 = %q, want gw-http", body)
	}

	tc, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("tcp dial: %v", err)
	}
	tc.Close()

	uc, err := net.Dial("udp", udpAddr)
	if err != nil {
		t.Fatalf("udp dial: %v", err)
	}
	if _, err := uc.Write([]byte("x")); err != nil {
		t.Fatalf("udp write: %v", err)
	}
	uc.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 8)
	n, err := uc.Read(buf)
	if err != nil || string(buf[:n]) != "gw-udp" {
		t.Fatalf("udp 响应 = %q, err=%v, want gw-udp", buf[:n], err)
	}
	uc.Close()

	cancel()
	w.shutdown(context.Background())
}

func TestListenerServeMux(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &muxEchoComp{}))

	c, err := w.getImpl(reflect.TypeFor[muxEchoComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, c.(*muxEchoComp).Addr)

	for path, want := range map[string]string{"/hello": "hello", "/bye": "bye"} {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != want {
			t.Fatalf("%s 响应 = %q, want %q", path, body, want)
		}
	}

	// 未注册路由由 http.ServeMux 默认返回 404
	resp, err := http.Get("http://" + addr + "/none")
	if err != nil {
		t.Fatalf("get /none: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/none 状态码 = %d, want 404", resp.StatusCode)
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerSetExternalEngine(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &engineComp{}))

	c, err := w.getImpl(reflect.TypeFor[engineComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, c.(*engineComp).Addr)

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "engine" {
		t.Fatalf("响应 = %q, want %q", body, "engine")
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerRejectsHandlerless(t *testing.T) {
	ctx, _, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &handlerlessComp{}))

	if _, err := w.getImpl(reflect.TypeFor[handlerlessComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err != nil {
		if !strings.Contains(err.Error(), "Handler/Mux") {
			t.Fatalf("错误信息应包含 Handler/Mux,得到 %v", err)
		}
	}
	// serve 失败经 cancel 异步上报
	awaitCancel(t, ctx, "未提供任何 handler")
	w.shutdown(context.Background())
}

func TestListenerRejectsUnimplementedHandler(t *testing.T) {
	ctx, _, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &badHTTPComp{}))

	if _, err := w.getImpl(reflect.TypeFor[badHTTPComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}
	// 声明 http.Handler 但既未实现也未 Set → serve 阶段快速失败并 cancel
	if err := w.start(ctx); err != nil {
		if !strings.Contains(err.Error(), "Handler/Mux") {
			t.Fatalf("错误信息应包含 Handler/Mux,得到 %v", err)
		}
	}
	awaitCancel(t, ctx, "声明 http.Handler 但未提供 handler")
	w.shutdown(context.Background())
}

func TestListenerRejectsAmbiguousProtocols(t *testing.T) {
	_, _, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &ambiguousComp{}))

	if _, err := w.getImpl(reflect.TypeFor[ambiguousComp]()); err == nil {
		t.Fatal("组件实现多种 handler 接口却只用聚合 Listener,装配应报错")
	}
}

func TestListenerConflictsWithStart(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &startAndListenerComp{}))

	if _, err := w.getImpl(reflect.TypeFor[startAndListenerComp]()); err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err == nil {
		t.Fatal("同时定义 Start 与 Listener,start 应报错")
	}
	cancel()
	w.shutdown(context.Background())
}

func TestListenerWithoutTagSkipped(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "other:\n  foo: 1\n", listenerReg(t, &noTagComp{}))

	c, err := w.getImpl(reflect.TypeFor[noTagComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if addr := c.(*noTagComp).Addr(); addr != "" {
		t.Fatalf("无配置标签的 listener 不应启动,addr = %q", addr)
	}
	cancel()
	w.shutdown(context.Background())
}

// tcpEventComp 记录 OnConnect/OnDisconnect 事件。
// 注意:widget 反射创建的是全新实例,事件状态必须能在零值上工作。
type tcpEventComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`

	mu      sync.Mutex
	Online  []TCPSession
	Offline []TCPSession
}

func (c *tcpEventComp) ServeTCP(_ context.Context, conn net.Conn) {
	_, _ = io.Copy(io.Discard, conn)
}

func (c *tcpEventComp) OnConnect(s TCPSession) {
	c.mu.Lock()
	c.Online = append(c.Online, s)
	c.mu.Unlock()
}

func (c *tcpEventComp) OnDisconnect(s TCPSession) {
	c.mu.Lock()
	c.Offline = append(c.Offline, s)
	c.mu.Unlock()
}

func (c *tcpEventComp) events() (online, offline []TCPSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Online, c.Offline
}

// udpEventComp 记录 OnPeerConnect/OnPeerDisconnect 事件(零值可用)。
type udpEventComp struct {
	Implements[listenerIntf]
	Listener[Handler] `conf:"listener"`

	mu     sync.Mutex
	joined []net.Addr
	left   []net.Addr
}

func (c *udpEventComp) ServeUDP(_ context.Context, pkt UDPPacket) ([]byte, error) {
	return append([]byte(nil), pkt.Data...), nil
}

func (c *udpEventComp) OnPeerConnect(addr net.Addr) {
	c.mu.Lock()
	c.joined = append(c.joined, addr)
	c.mu.Unlock()
}

func (c *udpEventComp) OnPeerDisconnect(addr net.Addr) {
	c.mu.Lock()
	c.left = append(c.left, addr)
	c.mu.Unlock()
}

func (c *udpEventComp) events() (joined, left []net.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.joined, c.left
}

func TestListenerActiveTimeout(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t,
		"listener:\n  addr: \"127.0.0.1:0\"\n  activetimeout: 300ms\n",
		listenerReg(t, &tcpSessComp{}))

	c, err := w.getImpl(reflect.TypeFor[tcpSessComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*tcpSessComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, comp.Addr)

	// 1) 纯接收客户端(只读不写):服务端每 100ms 推送成功即刷新活跃时间,应一直存活
	receiver, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial receiver: %v", err)
	}
	defer receiver.Close()
	go func() { _, _ = io.Copy(io.Discard, receiver) }() // 只排水,从不上行

	deadline := time.Now().Add(3 * time.Second)
	for {
		if ss := comp.Sessions(); len(ss) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("3 秒内未登记会话")
		}
		time.Sleep(5 * time.Millisecond)
	}
	receiverID := comp.Sessions()[0].ID

	// 持续推送(跨越 3+ 个超时窗口):推送成功即刷新活跃时间,纯接收客户端应一直存活
	stopPush := make(chan struct{})
	pushDone := make(chan struct{})
	go func() {
		defer close(pushDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopPush:
				return
			case <-ticker.C:
				if err := comp.Send(receiverID, []byte("push")); err != nil {
					return
				}
			}
		}
	}()

	// 1 秒推送期(窗口 300ms)后仍在线,且 LastActive 是最后一次成功推送
	time.Sleep(time.Second)
	ss := comp.Sessions()
	if len(ss) != 1 || ss[0].ID != receiverID {
		close(stopPush)
		<-pushDone
		t.Fatalf("纯接收客户端应存活,当前会话数 %d", len(ss))
	}
	if time.Since(ss[0].LastActive) > 300*time.Millisecond {
		close(stopPush)
		<-pushDone
		t.Fatalf("LastActive 未被成功推送刷新: %v", ss[0].LastActive)
	}

	// 2) 双向静默客户端:服务端不向它推送、它也不上行,超过窗口应被踢除;
	// 与此同时纯接收客户端因持续推送成功而不受影响
	silent, err := net.Dial("tcp", addr)
	if err != nil {
		close(stopPush)
		<-pushDone
		t.Fatalf("dial silent: %v", err)
	}
	defer silent.Close()

	deadline = time.Now().Add(3 * time.Second)
	for {
		if _, err := silent.Read(make([]byte, 1)); err != nil {
			break // 被服务端关闭
		}
		if time.Now().After(deadline) {
			close(stopPush)
			<-pushDone
			t.Fatal("静默客户端 3 秒内未被踢除")
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(stopPush)
	<-pushDone

	if len(comp.Sessions()) != 1 || comp.Sessions()[0].ID != receiverID {
		t.Fatalf("纯接收客户端不应被误踢,当前会话数 %d", len(comp.Sessions()))
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerTCPEvents(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &tcpEventComp{}))

	c, err := w.getImpl(reflect.TypeFor[tcpEventComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*tcpEventComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, comp.Addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// OnConnect 触发
	deadline := time.Now().Add(3 * time.Second)
	for {
		if online, _ := comp.events(); len(online) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("3 秒内未收到 OnConnect 事件")
		}
		time.Sleep(10 * time.Millisecond)
	}
	online, _ := comp.events()
	if online[0].RemoteAddr.String() != conn.LocalAddr().String() {
		t.Fatalf("OnConnect 会话地址 = %v, want %v", online[0].RemoteAddr, conn.LocalAddr())
	}

	// 服务端主动断开 → OnDisconnect 触发,会话已注销
	if err := comp.CloseConn(comp.Sessions()[0].ID); err != nil {
		t.Fatalf("close conn: %v", err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		_, offline := comp.events()
		if len(offline) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("3 秒内未收到 OnDisconnect 事件")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, offline := comp.events()
	if offline[0].ID != online[0].ID {
		t.Fatalf("离线会话 ID = %d, want %d", offline[0].ID, online[0].ID)
	}
	deadline = time.Now().Add(3 * time.Second)
	for len(comp.Sessions()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("断开后会话未注销")
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn.Close()
	cancel()
	w.shutdown(context.Background())
}

func TestListenerUDPEvents(t *testing.T) {
	oldTTL := udpPeerTTL
	udpPeerTTL = 200 * time.Millisecond
	t.Cleanup(func() { udpPeerTTL = oldTTL })

	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &udpEventComp{}))

	c, err := w.getImpl(reflect.TypeFor[udpEventComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*udpEventComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	addr := waitAddr(t, comp.Addr)

	client, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	if _, err := client.Write([]byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// OnPeerConnect 触发
	deadline := time.Now().Add(3 * time.Second)
	for {
		if joined, _ := comp.events(); len(joined) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("3 秒内未收到 OnPeerConnect 事件")
		}
		time.Sleep(10 * time.Millisecond)
	}
	joined, _ := comp.events()
	if joined[0].String() != client.LocalAddr().String() {
		t.Fatalf("上线对端地址 = %v, want %v", joined[0], client.LocalAddr())
	}

	// 对端静默超过 TTL 后,后台清扫直接触发 OnPeerDisconnect(无需后续收包)
	deadline = time.Now().Add(3 * time.Second)
	for {
		_, left := comp.events()
		if len(left) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("3 秒内未收到 OnPeerDisconnect 事件")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, left := comp.events()
	if left[0].String() != client.LocalAddr().String() {
		t.Fatalf("离线对端地址 = %v, want %v", left[0], client.LocalAddr())
	}
	if peers := comp.Peers(); len(peers) != 0 {
		t.Fatalf("静默对端应被清扫,当前 %d 个", len(peers))
	}

	cancel()
	w.shutdown(context.Background())
}

func TestListenerWrongProtocolManagement(t *testing.T) {
	ctx, cancel, w := newListenerWidget(t, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(t, &tcpSessComp{}))

	c, err := w.getImpl(reflect.TypeFor[tcpSessComp]())
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	comp := c.(*tcpSessComp)
	if err := w.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitAddr(t, comp.Addr)

	if peers := comp.Peers(); peers != nil {
		t.Fatalf("tcp 绑定的 listener 调 Peers 应返回 nil,得到 %v", peers)
	}
	if err := comp.SendTo(nil, []byte("x")); err == nil || !strings.Contains(err.Error(), "udp") {
		t.Fatalf("tcp 绑定的 listener 调 SendTo 应报 udp 错误,得到 %v", err)
	}

	cancel()
	w.shutdown(context.Background())
}
