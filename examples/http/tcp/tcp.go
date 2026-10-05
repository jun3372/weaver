package tcp

import (
	"bufio"
	"context"
	"net"

	"github.com/jun3372/weaver"
)

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.Listener[weaver.TCPHandler] `conf:"tcp"`
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("tcp echo server init")
	return nil
}

// ServeTCP 实现 echo handler:收到什么就回什么,直到对端关闭;
// 监听、连接管理与优雅关闭由 Listener 负责,无需 Start。
func (i *impl) ServeTCP(_ context.Context, conn net.Conn) {
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		if _, err := conn.Write(append([]byte(sc.Text()), '\n')); err != nil {
			return
		}
	}
}
