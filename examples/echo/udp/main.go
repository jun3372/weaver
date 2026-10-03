package main

import (
	"bytes"
	"context"
	"time"

	"github.com/jun3372/weaver"
)

type T interface{}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[T]
}

// echo 实现 UDPPacketHandler 并内嵌 Listener,无需定义 Start;
// 返回非 nil 字节即作为回包发往来源地址。
type echo struct {
	weaver.Implements[T]
	weaver.Listener[weaver.Handler] `conf:"listener"`
}

func (e *echo) ServeUDP(_ context.Context, pkt weaver.UDPPacket) ([]byte, error) {
	return bytes.ToUpper(bytes.TrimRight(pkt.Data, "\n")), nil
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		e := a.Ref.Get().(*echo)
		a.Logger(ctx).Info("udp echo 已启动", "addr", e.Addr())

		// 定期汇报活跃对端,演示 Peers 对端表;SendTo 可向指定对端定点推送
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
				peers := e.Peers()
				addrs := make([]string, 0, len(peers))
				for _, p := range peers {
					addrs = append(addrs, p.Addr.String())
				}
				a.Logger(ctx).Info("活跃对端", "count", len(addrs), "addrs", addrs)
			}
		}
	}); err != nil {
		panic(err)
	}
}
