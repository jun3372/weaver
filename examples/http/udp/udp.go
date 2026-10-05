package udp

import (
	"bytes"
	"context"

	"github.com/jun3372/weaver"
)

type T interface{}

type impl struct {
	weaver.Implements[T]
	weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

func (i *impl) Init(ctx context.Context) error {
	i.Logger(ctx).Info("udp echo server init")
	return nil
}

// ServeUDP 实现 echo handler:返回非 nil 字节即作为回包发往来源地址;
// 监听、读循环、派发与回包由 Listener 负责,无需 Start。
func (i *impl) ServeUDP(_ context.Context, pkt weaver.UDPPacket) ([]byte, error) {
	return bytes.ToUpper(bytes.TrimRight(pkt.Data, "\n")), nil
}
