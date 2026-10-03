// 私有协议解析示例:TCP 演示 SOCKS5(RFC 1928)方法协商的字节流分帧解析,
// UDP 演示 magic+len 二进制帧的 datagram 解析。解析函数独立于网络,可单独单测。
package main

import (
	"context"
	"encoding/binary"
	"io"
	"net"

	"github.com/pkg/errors"

	"github.com/jun3372/weaver"
)

type T interface{}

type app struct {
	weaver.Implements[weaver.Main]
	weaver.Ref[T]
}

// proto 同时实现两种 handler 接口,以具体接口作 H 各绑一个 Listener。
type proto struct {
	weaver.Implements[T]
	tcpL weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
	udpL weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
}

// negotiate 解析 SOCKS5 方法协商请求(VER NMETHODS METHODS)并应答
// "不接受任何方法"。TCP 是字节流,会粘包/拆包,必须用 io.ReadFull 分帧。
func negotiate(r io.Reader, w io.Writer) error {
	var head [2]byte // VER(1) NMETHODS(1)
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return errors.Wrap(err, "read socks5 header")
	}
	if head[0] != 5 {
		return errors.Errorf("unsupported socks version %d", head[0])
	}
	methods := make([]byte, head[1])
	if _, err := io.ReadFull(r, methods); err != nil {
		return errors.Wrap(err, "read socks5 methods")
	}
	_, err := w.Write([]byte{5, 0xFF}) // VER=5, METHOD=0xFF(no acceptable methods)
	return err
}

func (p *proto) ServeTCP(ctx context.Context, conn net.Conn) {
	if err := negotiate(conn, conn); err != nil {
		p.Logger(ctx).Error("socks5 negotiate failed", "remote", conn.RemoteAddr().String(), "err", err)
	}
}

// decode 解析 magic(2) | len(2) | payload(len) 帧,返回 payload。
// UDP 天然按 datagram 边界,无粘包,直接 binary 解析即可。
func decode(pkt []byte) ([]byte, error) {
	if len(pkt) < 4 {
		return nil, errors.New("short packet")
	}
	const magic = 0xCAFE
	if m := binary.BigEndian.Uint16(pkt[0:2]); m != magic {
		return nil, errors.Errorf("bad magic %#x", m)
	}
	n := int(binary.BigEndian.Uint16(pkt[2:4]))
	if n != len(pkt)-4 {
		return nil, errors.Errorf("len mismatch: header says %d, packet has %d", n, len(pkt)-4)
	}
	return pkt[4:], nil
}

func encode(payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint16(out[0:2], 0xCAFE)
	binary.BigEndian.PutUint16(out[2:4], uint16(len(payload)))
	copy(out[4:], payload)
	return out
}

func (p *proto) ServeUDP(_ context.Context, pkt weaver.UDPPacket) ([]byte, error) {
	payload, err := decode(pkt.Data)
	if err != nil {
		return nil, err
	}
	return encode(append([]byte("echo:"), payload...)), nil
}

func main() {
	if err := weaver.Run(context.Background(), func(ctx context.Context, a *app) error {
		a.Logger(ctx).Info("protocol 示例已启动,监听 :8083(tcp) / :8084(udp)")
		<-ctx.Done()
		return nil
	}); err != nil {
		panic(err)
	}
}
