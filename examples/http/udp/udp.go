package udp

import (
	"bytes"
	"context"

	"github.com/jun3372/weaver"
)

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.UDPServer `conf:"udp"`
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("udp echo server init")
	return nil
}

// Start 挂载 echo handler:返回非 nil 字节即作为回包发往来源地址;
// 读循环、派发与回包由 weaver.UDPServer 负责。
func (i *impl) Start(ctx context.Context) error {
	return i.UDPServer.Serve(ctx, echoHandler{})
}

type echoHandler struct{}

func (echoHandler) ServeUDP(_ context.Context, pkt weaver.UDPPacket) ([]byte, error) {
	return bytes.ToUpper(bytes.TrimRight(pkt.Data, "\n")), nil
}
