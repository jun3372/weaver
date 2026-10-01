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
	weaver.TCPServer `conf:"tcp"`
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("tcp echo server init")
	return nil
}

// Start 挂载 echo handler:收到什么就回什么,直到对端关闭;
// 连接管理与优雅关闭由 weaver.TCPServer 负责。
func (i *impl) Start(ctx context.Context) error {
	return i.TCPServer.Serve(ctx, echoHandler{})
}

type echoHandler struct{}

func (echoHandler) ServeTCP(_ context.Context, conn net.Conn) {
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		if _, err := conn.Write(append([]byte(sc.Text()), '\n')); err != nil {
			return
		}
	}
}
