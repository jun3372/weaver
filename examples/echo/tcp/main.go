package main

import (
	"bufio"
	"context"
	"fmt"
	"net"

	"github.com/jun3372/weaver"
)

type T interface{}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[T]
}

// echo 实现 TCPHandler 并内嵌 Listener,无需定义 Start;
// 每收到一行,除回显外还向其他会话广播,演示 Sessions/Send 定点推送。
type echo struct {
	weaver.Implements[T]
	weaver.Listener[weaver.Handler] `conf:"listener"`
}

func (e *echo) ServeTCP(_ context.Context, conn net.Conn) {
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		line := append([]byte(sc.Text()), '\n')
		if _, err := conn.Write(line); err != nil {
			return
		}

		// 定点推送:连接级写锁保证与各 handler 的写入互斥
		for _, s := range e.Sessions() {
			if s.Conn == conn {
				continue
			}
			msg := fmt.Sprintf("[broadcast from %s] %s\n", conn.RemoteAddr(), sc.Text())
			if err := e.Send(s.ID, []byte(msg)); err != nil {
				fmt.Println("send failed:", err)
			}
		}
	}
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("tcp echo 已启动,监听 :8081")
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
