package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/jun3372/weaver"
)

type T interface{}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[T]
}

// gateway 同时支持三种协议:TCP/UDP 直接实现对应 handler 接口;
// HTTP 通过具名 Listener 的 Mux() 注册多路由,无需实现 ServeHTTP 与 Start。
type gateway struct {
	weaver.Implements[T]
	httpL weaver.Listener[http.Handler]            `conf:"http"`
	tcpL  weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
	udpL  weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

func (g *gateway) Init(ctx context.Context) error {
	g.httpL.Mux().HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintln(w, "http ok, tcp/udp 也在线")
	})
	g.httpL.Mux().HandleFunc("GET /sessions", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "当前 TCP 会话数: %d\n", len(g.tcpL.Sessions()))
	})
	return nil
}

func (g *gateway) ServeTCP(_ context.Context, conn net.Conn) {
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		if _, err := conn.Write(append([]byte(sc.Text()), '\n')); err != nil {
			return
		}
	}
}

func (g *gateway) ServeUDP(_ context.Context, pkt weaver.UDPPacket) ([]byte, error) {
	return bytes.ToUpper(bytes.TrimRight(pkt.Data, "\n")), nil
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("三协议网关已启动,监听 :8080(http) / :8081(tcp) / :8082(udp)")
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
