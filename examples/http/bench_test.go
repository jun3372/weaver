package main

// 框架性能基准:在进程内启动完整 http 示例(HTTP/TCP/UDP 全部监听 :0),
// 与原生 net/http 基线对比,量化框架在请求路径上的开销。

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jun3372/weaver"
)

var (
	httpAddr string
	tcpAddr  string
	udpAddr  string
	runDone  chan error
)

func addrOf(v any) string {
	type addrer interface{ Addr() string }
	if a, ok := v.(addrer); ok {
		return a.Addr()
	}
	return ""
}

func waitAddr(t testing.TB, get func() string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a := get(); a != "" {
			return a
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待监听地址超时")
	return ""
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "weaver-bench")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	conf := filepath.Join(dir, "weaver.yaml")
	cfg := []byte(`
app:
  Name: bench
http:
  Addr: ":0"
  Message: "stress-A"
tcp:
  Addr: ":0"
udp:
  Addr: ":0"
weaver:
  Logger:
    Level: "error"
    Type: "json"
`)
	if err := os.WriteFile(conf, cfg, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("SERVICE_CONFIG", conf)

	// 提前解析 flags,避免与 weaver.Run 内部的 flag 注册/解析并发竞争
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

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		a := appPtr
		mu.Unlock()
		if a != nil {
			if h := addrOf(a.Ref.Get()); h != "" {
				httpAddr = h
				tcpAddr = addrOf(a.tcp.Get())
				udpAddr = addrOf(a.udp.Get())
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if httpAddr == "" {
		fmt.Fprintln(os.Stderr, "示例启动超时")
		cancel()
		os.Exit(1)
	}

	code := m.Run()
	cancel()
	<-runDone
	os.Exit(code)
}

// --- HTTP ---

// 框架路径:请求经过 weaver 示例的 mux handler(每次请求读取热更新配置)。
func BenchmarkHTTP_Echo(b *testing.B) {
	benchHTTP(b, "http://"+httpAddr+"/")
}

// 原生基线:等价的 net/http handler,量化框架层开销。
func BenchmarkHTTP_RawBaseline(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "%s\n", "bench")
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	b.ResetTimer()
	benchHTTP(b, "http://"+ln.Addr().String()+"/")
}

func benchHTTP(b *testing.B, url string) {
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConnsPerHost: 8,
	}}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(url)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// 框架路径并发吞吐。
func BenchmarkHTTP_Echo_Parallel(b *testing.B) {
	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 256,
	}}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get("http://" + httpAddr + "/")
			if err != nil {
				b.Fatal(err)
			}
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// 配置读取微基准:WithConfig 每次读取都要走 RLock + 值拷贝(Listener 内部
// 同机制,已收敛不再单独导出)。
func BenchmarkConfigRead(b *testing.B) {
	var c weaver.WithConfig[weaver.HTTPOption]
	c.SetConfig(weaver.HTTPOption{Addr: ":8080"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Config()
	}
}

// --- TCP ---

// 单连接串行回显往返,衡量每请求框架转发开销(accept/goroutine 摊销后)。
func BenchmarkTCP_Echo(b *testing.B) {
	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	msg := []byte("ping\n")
	buf := make([]byte, len(msg))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(msg); err != nil {
			b.Fatal(err)
		}
		if _, err := io.ReadFull(r, buf); err != nil {
			b.Fatal(err)
		}
	}
}

// 多连接并发回显吞吐。
func BenchmarkTCP_Echo_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("tcp", tcpAddr)
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		msg := []byte("ping\n")
		buf := make([]byte, len(msg))
		for pb.Next() {
			if _, err := conn.Write(msg); err != nil {
				b.Fatal(err)
			}
			if _, err := io.ReadFull(r, buf); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- UDP ---

// 串行收发往返:每包含框架侧的拷贝与独立 goroutine 派发开销。
func BenchmarkUDP_Echo(b *testing.B) {
	conn, err := net.Dial("udp", udpAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	msg := []byte("ping")
	buf := make([]byte, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := conn.Write(msg); err != nil {
			b.Fatal(err)
		}
		if _, err := conn.Read(buf); err != nil {
			b.Fatal(err)
		}
	}
}
