package weaver

import (
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

// 新功能性能基准:TCP 会话管理(Send/Sessions)、UDP 对端表(recordPeer/prunePeers)
// 的开销。

// benchAddr 是基准用的 net.Addr 桩,避免真实拨号。
type benchAddr string

func (a benchAddr) Network() string { return "udp" }
func (a benchAddr) String() string  { return string(a) }

// startBenchTCP 启动一个 TCP listener 组件并接入一条客户端连接,返回组件、会话 ID 与连接。
// 客户端持续读取推送数据并以 100ms 心跳保活,模拟真实在线客户端。
func startBenchTCP(b *testing.B) (*tcpSessComp, uint64, net.Conn) {
	b.Helper()
	ctx, cancel, w := newListenerWidget(b, "listener:\n  addr: \"127.0.0.1:0\"\n",
		listenerReg(b, &tcpSessComp{}))
	b.Cleanup(cancel)

	c, err := w.getImpl(reflect.TypeFor[tcpSessComp]())
	if err != nil {
		b.Fatalf("create component: %v", err)
	}
	comp := c.(*tcpSessComp)
	if err := w.start(ctx); err != nil {
		b.Fatalf("start: %v", err)
	}
	addr := waitAddr(b, comp.Addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	b.Cleanup(func() { conn.Close() })

	// 心跳 + 排水:客户端不发数据会被活跃超时判定离线(预期行为)
	go func() {
		_, _ = io.Copy(io.Discard, conn)
	}()
	heartbeat := time.NewTicker(100 * time.Millisecond)
	go func() {
		for range heartbeat.C {
			if _, err := conn.Write([]byte{0}); err != nil {
				heartbeat.Stop()
				return
			}
		}
	}()
	b.Cleanup(heartbeat.Stop)

	deadline := time.Now().Add(3 * time.Second)
	for {
		if ss := comp.Sessions(); len(ss) == 1 {
			return comp, ss[0].ID, conn
		}
		if time.Now().After(deadline) {
			b.Fatal("3 秒内未登记会话")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// BenchmarkTCPSend 衡量外部定点推送的单次开销:handler 侧 io.Copy 消费,
// 路径为连接级写锁 + 一次 write 系统调用 + 原子活跃时间戳。
func BenchmarkTCPSend(b *testing.B) {
	comp, id, conn := startBenchTCP(b)
	defer conn.Close()
	data := make([]byte, 64)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := comp.Send(id, data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTCPSessions 衡量会话快照(拷贝 + 排序)的开销。
func BenchmarkTCPSessions(b *testing.B) {
	comp, _, conn := startBenchTCP(b)
	defer conn.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = comp.Sessions()
	}
}

// benchPrimePeers 构造一个带 n 个对端的 udpServer。
func benchPrimePeers(b *testing.B, n int, stale bool) *udpServer {
	b.Helper()
	var s udpServer
	s.peerTable = make(map[string]*udpPeer, n)
	lastSeen := time.Now()
	if stale {
		lastSeen = lastSeen.Add(-2 * udpPeerTTL)
	}
	for i := 0; i < n; i++ {
		a := benchAddr("192.0.2.1:10000")
		p := &udpPeer{addr: a, lastSeen: lastSeen}
		s.peerTable[a.String()] = p
	}
	return &s
}

// BenchmarkUDPPeerRecord 衡量大表(1000 对端)下单包登记的开销,应为 O(1)。
func BenchmarkUDPPeerRecord(b *testing.B) {
	s := benchPrimePeers(b, 1000, false)
	addr := benchAddr("192.0.2.1:20000")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.recordPeer(addr)
	}
}

// BenchmarkUDPPeerPrune 衡量清扫对 1000 个全部过期的对端做一轮清理的总开销,
// 实际运行频率为 udpPeerTTL/4(默认 5 分钟跑一次)。
func BenchmarkUDPPeerPrune(b *testing.B) {
	s := benchPrimePeers(b, 1000, true)
	now := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s.peerTable = make(map[string]*udpPeer, 1000)
		lastSeen := now.Add(-2 * udpPeerTTL)
		for j := 0; j < 1000; j++ {
			a := benchAddr("192.0.2.1:10000")
			s.peerTable[a.String()] = &udpPeer{addr: a, lastSeen: lastSeen}
		}
		b.StartTimer()
		s.prunePeers(now)
	}
}
