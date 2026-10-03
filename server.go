package weaver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 服务组件默认优雅关闭等待时间。
const defaultShutdownTimeout = 5 * time.Second

// HTTPServer 超时与请求头默认值;零值字段取默认,显式负值关闭对应超时。
const (
	defaultReadTimeout       = 30 * time.Second
	defaultReadHeaderTimeout = 10 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultMaxHeaderBytes    = 1 << 20

	defaultActiveTimeout = 60 * time.Second
)

// timeoutOr 解析超时配置:0 取默认,负值返回 0(关闭超时),其余原样返回。
func timeoutOr(v, def time.Duration) time.Duration {
	switch {
	case v < 0:
		return 0
	case v == 0:
		return def
	default:
		return v
	}
}

// maxHeaderBytesOr 解析请求头大小上限:<=0 取默认。
func maxHeaderBytesOr(v int) int {
	if v <= 0 {
		return defaultMaxHeaderBytes
	}
	return v
}

// HTTPOption 是 HTTPServer 的配置项,通过 conf tag 注入。
type HTTPOption struct {
	Addr            string        // 监听地址,如 ":8080";Serve 时读取,修改需重启
	ShutdownTimeout time.Duration // 优雅关闭等待时间,缺省 5s
	// 超时防慢速攻击(slowloris):0 取默认,负值显式关闭。
	ReadTimeout       time.Duration // 整个请求读取超时,缺省 30s
	ReadHeaderTimeout time.Duration // 请求头读取超时,缺省 10s
	WriteTimeout      time.Duration // 响应写出超时,缺省 30s(长连接流式响应需显式关闭)
	IdleTimeout       time.Duration // keep-alive 空闲超时,缺省 120s
	MaxHeaderBytes    int           // 请求头大小上限,缺省 1MB
}

// TCPOption 是 TCPServer 的配置项,通过 conf tag 注入。
type TCPOption struct {
	Addr            string
	ShutdownTimeout time.Duration
	MaxConns        int           // 最大并发连接数,超过后立即关闭新连接;<=0 不限
	ConnIdleTimeout time.Duration // 连接空闲超时,每次收发自动续期;<=0 不限
	// ActiveTimeout 连接活跃超时:超过该时长未发生任何成功的收或发
	// (客户端上行数据、服务端推送成功均计入)即判定离线,由后台清扫主动
	// 关闭连接(handler 将收到读错误并退出);0 取默认 60s,负值关闭。
	ActiveTimeout time.Duration
}

// UDPOption 是 UDPServer 的配置项,通过 conf tag 注入。
type UDPOption struct {
	Addr            string
	ShutdownTimeout time.Duration
	MaxConcurrent   int // 单包处理最大并发数,超过后丢弃报文;<=0 不限
}

// TCPHandler 处理单条 TCP 连接,conn 由框架在关闭时负责回收。
type TCPHandler interface {
	ServeTCP(ctx context.Context, conn net.Conn)
}

// TCPSession 是活跃 TCP 连接的快照;Conn 即 handler 收到的同一连接,
// 可用指针比较定位自身,但外部写入请走 Send(自带连接级写锁)。
type TCPSession struct {
	ID         uint64
	RemoteAddr net.Addr
	Conn       net.Conn
	LastActive time.Time // 最近一次成功收/发的时间
}

// UDPPacket 是一次 UDP 收包。
type UDPPacket struct {
	Data []byte
	Addr net.Addr
}

// UDPPacketHandler 处理单个 UDP 报文;返回非 nil 字节将作为回包发往来源地址。
type UDPPacketHandler interface {
	ServeUDP(ctx context.Context, pkt UDPPacket) ([]byte, error)
}

// TCPOnConnect / TCPOnDisconnect 是可选实现的事件回调:TCP handler(通常为
// 组件自身)按需实现,框架在连接建立与断开(含 CloseConn 主动断开、活跃超时
// 踢除)时经类型断言调用;回调不应长时间阻塞,其 panic 被框架隔离。
type TCPOnConnect interface{ OnConnect(TCPSession) }

type TCPOnDisconnect interface{ OnDisconnect(TCPSession) }

// UDPOnConnect / UDPOnDisconnect 是 UDP 对端的事件回调:对端首包登记时触发
// OnPeerConnect;对端超过 udpPeerTTL(5 分钟)未活跃时由后台清扫触发
// OnPeerDisconnect,不依附于后续收包。
type UDPOnConnect interface{ OnPeerConnect(addr net.Addr) }

type UDPOnDisconnect interface{ OnPeerDisconnect(addr net.Addr) }

// safeEvent 调用事件回调并隔离其 panic,避免影响 accept/读循环。
func safeEvent(log *slog.Logger, name string, fn func()) {
	defer func() {
		if e := recover(); e != nil {
			log.Error("服务事件回调 panic", "event", name, "err", e)
		}
	}()
	fn()
}

// serverLog 返回注入的组件 logger;未注入(直接构造服务端、未走 widget 装配)时
// 回退 slog.Default()。
func serverLog(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}

// HTTPServer 内嵌于组件,提供声明式 HTTP 服务:配置注入监听地址,Serve 挂载
// handler 并长驻阻塞,ctx 结束后优雅关闭。同一组件可声明多个具名实例。
type HTTPServer struct {
	mu     sync.RWMutex
	config HTTPOption
	log    *slog.Logger // 框架注入的组件 logger,未注入时回退 slog.Default()

	srv *http.Server
	ln  net.Listener
}

// setLog 注入框架初始化的组件 logger,由 widget 在装配阶段调用。
func (s *HTTPServer) setLog(log *slog.Logger) { s.log = log }

func (s *HTTPServer) Config() HTTPOption {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig 整体替换配置,由框架在配置注入/热更新时通过反射调用。
func (s *HTTPServer) SetConfig(v HTTPOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = v
}

// Addr 返回绑定后的实际监听地址(如 ":0" 场景),未在服务中时返回 ""。
func (s *HTTPServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Serve 在配置的地址上挂载 handler 并长驻阻塞;
// ctx 结束后按 ShutdownTimeout 优雅关闭,完成后返回 nil。
func (s *HTTPServer) Serve(ctx context.Context, h http.Handler) error {
	opt := s.Config()
	if opt.Addr == "" {
		return errors.New("http server: 未配置监听地址,请检查组件的 conf tag")
	}
	s.mu.Lock()
	if s.srv != nil {
		s.mu.Unlock()
		return errors.New("http server: Serve 被重复调用")
	}
	s.mu.Unlock()

	ln, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		return fmt.Errorf("http server: listen %s: %w", opt.Addr, err)
	}

	s.mu.Lock()
	s.ln = ln
	s.srv = &http.Server{
		Handler:           h,
		ReadTimeout:       timeoutOr(opt.ReadTimeout, defaultReadTimeout),
		ReadHeaderTimeout: timeoutOr(opt.ReadHeaderTimeout, defaultReadHeaderTimeout),
		WriteTimeout:      timeoutOr(opt.WriteTimeout, defaultWriteTimeout),
		IdleTimeout:       timeoutOr(opt.IdleTimeout, defaultIdleTimeout),
		MaxHeaderBytes:    maxHeaderBytesOr(opt.MaxHeaderBytes),
	}
	s.mu.Unlock()
	// Serve 返回后清空运行态,Addr() 归零并允许修正配置后重新 Serve
	defer func() {
		s.mu.Lock()
		s.srv = nil
		s.ln = nil
		s.mu.Unlock()
	}()

	timeout := opt.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}

	log := serverLog(s.log)
	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()
	log.Info("http server listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server exited", "err", err)
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown failed", "err", err)
		return err
	}
	log.Info("http server shutdown", "addr", opt.Addr)
	return nil
}

// TCPServer 内嵌于组件,提供声明式 TCP 服务:accept 循环、连接管理与优雅关闭
// 由框架负责,用户只需实现 TCPHandler;支持会话枚举与定点发送(Sessions/Send/CloseConn)。
type TCPServer struct {
	mu     sync.RWMutex
	config TCPOption
	log    *slog.Logger // 框架注入的组件 logger,未注入时回退 slog.Default()

	ln     net.Listener
	nextID uint64
	conns  map[uint64]*tcpConn
	served bool
}

// setLog 注入框架初始化的组件 logger,由 widget 在装配阶段调用。
func (s *TCPServer) setLog(log *slog.Logger) { s.log = log }

func (s *TCPServer) Config() TCPOption {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig 整体替换配置,由框架在配置注入/热更新时通过反射调用。
func (s *TCPServer) SetConfig(v TCPOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = v
}

// Addr 返回绑定后的实际监听地址,未在服务中时返回 ""。
func (s *TCPServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Serve 在配置的地址上开始 accept 并长驻阻塞;每个连接由独立 goroutine 调用
// handler。ctx 结束后停止 accept、关闭全部连接并限时等待退出。
func (s *TCPServer) Serve(ctx context.Context, h TCPHandler) error {
	opt := s.Config()
	if opt.Addr == "" {
		return errors.New("tcp server: 未配置监听地址,请检查组件的 conf tag")
	}

	s.mu.Lock()
	if s.served {
		s.mu.Unlock()
		return errors.New("tcp server: Serve 被重复调用")
	}
	s.served = true
	s.conns = make(map[uint64]*tcpConn)
	s.mu.Unlock()
	// Serve 返回后清空运行态,Addr() 归零并允许修正配置后重新 Serve
	defer func() {
		s.mu.Lock()
		s.ln = nil
		s.conns = nil
		s.served = false
		s.mu.Unlock()
	}()

	ln, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		return fmt.Errorf("tcp server: listen %s: %w", opt.Addr, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	log := serverLog(s.log)
	log.Info("tcp server listening", "addr", ln.Addr().String())

	// 事件回调:handler 按需实现 OnConnect/OnDisconnect,未实现则跳过
	onConnect, _ := h.(TCPOnConnect)
	onDisconnect, _ := h.(TCPOnDisconnect)

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	// 活跃超时清扫:定期关闭未在窗口内收到客户端数据的连接(判定离线)
	if active := timeoutOr(opt.ActiveTimeout, defaultActiveTimeout); active > 0 {
		go s.reapIdle(ctx, active, log)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				s.waitConns(&wg, opt.ShutdownTimeout, log)
				log.Info("tcp server shutdown", "addr", ln.Addr().String())
				return nil
			default:
			}
			log.Error("tcp server accept failed", "err", err)
			return fmt.Errorf("tcp server: accept: %w", err)
		}

		// 会话登记:分配递增 ID,写锁包装保证 handler 与外部 Send 并发写安全
		s.mu.Lock()
		s.nextID++
		tc := &tcpConn{Conn: conn, id: s.nextID, idle: opt.ConnIdleTimeout}
		if full := opt.MaxConns > 0 && len(s.conns) >= opt.MaxConns; full {
			s.mu.Unlock()
			log.Warn("tcp server 连接数已达上限,拒绝新连接",
				"max", opt.MaxConns, "remote", conn.RemoteAddr().String())
			_ = conn.Close()
			continue
		}
		s.conns[tc.id] = tc
		s.mu.Unlock()
		tc.touch()

		sess := TCPSession{
			ID:         tc.id,
			RemoteAddr: conn.RemoteAddr(),
			Conn:       tc,
			LastActive: time.Unix(0, tc.lastActive.Load()),
		}
		if onConnect != nil {
			safeEvent(log, "OnConnect", func() { onConnect.OnConnect(sess) })
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if e := recover(); e != nil {
					log.Error("tcp server handler panic",
						"err", e, "remote", conn.RemoteAddr().String())
				}
			}()
			defer func() {
				_ = conn.Close()
				s.mu.Lock()
				delete(s.conns, tc.id)
				s.mu.Unlock()
				if onDisconnect != nil {
					disconnected := sess
					disconnected.LastActive = time.Unix(0, tc.lastActive.Load())
					safeEvent(log, "OnDisconnect", func() { onDisconnect.OnDisconnect(disconnected) })
				}
			}()
			h.ServeTCP(ctx, tc)
		}()
	}
}

// tcpConn 包装 net.Conn:连接级写锁保证 handler 与 Send 并发写安全;
// lastActive 由收/发两个方向成功后刷新(任一方向成功即视为连接活跃),
// 供活跃超时清扫判定离线;每次读写前重置绝对 deadline,实现空闲超时。
type tcpConn struct {
	net.Conn
	id         uint64
	wmu        sync.Mutex
	idle       time.Duration
	lastActive atomic.Int64 // UnixNano
}

func (c *tcpConn) touch() { c.lastActive.Store(time.Now().UnixNano()) }

func (c *tcpConn) Read(p []byte) (int, error) {
	if c.idle > 0 {
		_ = c.SetReadDeadline(time.Now().Add(c.idle))
	}
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.touch()
	}
	return n, err
}

func (c *tcpConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.idle > 0 {
		_ = c.SetWriteDeadline(time.Now().Add(c.idle))
	}
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.touch()
	}
	return n, err
}

// reapIdle 定期关闭活跃超时的连接;handler 将因读错误退出并自行注销会话。
func (s *TCPServer) reapIdle(ctx context.Context, active time.Duration, log *slog.Logger) {
	interval := active / 4
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			deadline := time.Now().Add(-active)
			s.mu.RLock()
			var stale []*tcpConn
			for _, c := range s.conns {
				if time.Unix(0, c.lastActive.Load()).Before(deadline) {
					stale = append(stale, c)
				}
			}
			s.mu.RUnlock()

			for _, c := range stale {
				log.Warn("tcp server 连接活跃超时,判定离线并踢除",
					"id", c.id, "remote", c.RemoteAddr().String(), "timeout", active)
				_ = c.Close()
			}
		}
	}
}

// Sessions 返回活跃会话快照,按 ID 升序。
func (s *TCPServer) Sessions() []TCPSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TCPSession, 0, len(s.conns))
	for _, c := range s.conns {
		out = append(out, TCPSession{
			ID:         c.id,
			RemoteAddr: c.RemoteAddr(),
			Conn:       c,
			LastActive: time.Unix(0, c.lastActive.Load()),
		})
	}
	slices.SortFunc(out, func(a, b TCPSession) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// Send 向指定会话写入数据;连接级写锁保证与 handler 的写入互斥。
func (s *TCPServer) Send(id uint64, data []byte) error {
	s.mu.RLock()
	c := s.conns[id]
	s.mu.RUnlock()
	if c == nil {
		return fmt.Errorf("tcp server: 会话 %d 不存在", id)
	}
	_, err := c.Write(data)
	return err
}

// CloseConn 主动断开指定会话;handler 将收到读取错误并退出。
func (s *TCPServer) CloseConn(id uint64) error {
	s.mu.RLock()
	c := s.conns[id]
	s.mu.RUnlock()
	if c == nil {
		return fmt.Errorf("tcp server: 会话 %d 不存在", id)
	}
	return c.Close()
}

// waitConns 关闭全部活跃连接以解除 handler 阻塞,并限时等待其退出。
func (s *TCPServer) waitConns(wg *sync.WaitGroup, timeout time.Duration, log *slog.Logger) {
	s.mu.RLock()
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.mu.RUnlock()

	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Warn("tcp server shutdown timeout,部分连接未能及时退出")
	}
}

// UDPServer 内嵌于组件,提供声明式 UDP 服务:读循环、派发与回包由框架负责,
// 用户只需实现 UDPPacketHandler;支持对端表与定点发送(Peers/SendTo)。
type UDPServer struct {
	mu     sync.RWMutex
	config UDPOption
	log    *slog.Logger // 框架注入的组件 logger,未注入时回退 slog.Default()

	conn   net.PacketConn
	peers  map[string]*udpPeer
	served bool
}

// setLog 注入框架初始化的组件 logger,由 widget 在装配阶段调用。
func (s *UDPServer) setLog(log *slog.Logger) { s.log = log }

// udpPeerTTL 内未再收包的对端在下次收包时被惰性清理。
var udpPeerTTL = 5 * time.Minute

type udpPeer struct {
	addr     net.Addr
	lastSeen time.Time
}

// UDPPeer 是活跃 UDP 对端的快照。
type UDPPeer struct {
	Addr     net.Addr
	LastSeen time.Time
}

// recordPeer 登记对端并刷新活跃时间;若该对端自身已超 TTL(清扫尚未跑到),
// 先判定离线再作为新对端登记,保证离线/上线事件成对触发。
// 仅做 O(1) 的单键检查,全表清理由 reapIdlePeers 负责。
func (s *UDPServer) recordPeer(addr net.Addr) (added bool, pruned []net.Addr) {
	now := time.Now()
	key := addr.String()

	s.mu.Lock()
	p, ok := s.peers[key]
	if ok && now.Sub(p.lastSeen) > udpPeerTTL {
		delete(s.peers, key)
		pruned = append(pruned, p.addr)
		ok = false
	}
	if !ok {
		added = true
	}
	s.peers[key] = &udpPeer{addr: addr, lastSeen: now}
	s.mu.Unlock()
	return added, pruned
}

// reapIdlePeers 定期清理超过 udpPeerTTL 未活跃的静默对端并触发离线事件。
func (s *UDPServer) reapIdlePeers(ctx context.Context, onDisconnect UDPOnDisconnect, log *slog.Logger) {
	interval := udpPeerTTL / 4
	if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for _, pa := range s.prunePeers(time.Now()) {
				if onDisconnect != nil {
					safeEvent(log, "OnPeerDisconnect", func() { onDisconnect.OnPeerDisconnect(pa) })
				}
			}
		}
	}
}

// prunePeers 清理超过 udpPeerTTL 未活跃的对端,返回被清理的对端列表。
func (s *UDPServer) prunePeers(now time.Time) []net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pruned []net.Addr
	for k, p := range s.peers {
		if now.Sub(p.lastSeen) > udpPeerTTL {
			pruned = append(pruned, p.addr)
			delete(s.peers, k)
		}
	}
	return pruned
}

// Peers 返回最近活跃的对端快照,按地址字符串升序。
func (s *UDPServer) Peers() []UDPPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UDPPeer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, UDPPeer{Addr: p.addr, LastSeen: p.lastSeen})
	}
	slices.SortFunc(out, func(a, b UDPPeer) int { return strings.Compare(a.Addr.String(), b.Addr.String()) })
	return out
}

// SendTo 向指定对端发送报文;可在 handler 外主动推送。
func (s *UDPServer) SendTo(addr net.Addr, data []byte) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()
	if conn == nil {
		return errors.New("udp server: 未在服务中,无法发送")
	}
	_, err := conn.WriteTo(data, addr)
	return err
}

func (s *UDPServer) Config() UDPOption {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig 整体替换配置,由框架在配置注入/热更新时通过反射调用。
func (s *UDPServer) SetConfig(v UDPOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = v
}

// Addr 返回绑定后的实际监听地址,未在服务中时返回 ""。
func (s *UDPServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.conn == nil {
		return ""
	}
	return s.conn.LocalAddr().String()
}

// Serve 在配置的地址上开始收包并长驻阻塞;每个报文由独立 goroutine 调用
// handler,handler 返回的非 nil 字节作为回包发往来源地址。
func (s *UDPServer) Serve(ctx context.Context, h UDPPacketHandler) error {
	opt := s.Config()
	if opt.Addr == "" {
		return errors.New("udp server: 未配置监听地址,请检查组件的 conf tag")
	}

	s.mu.Lock()
	if s.served {
		s.mu.Unlock()
		return errors.New("udp server: Serve 被重复调用")
	}
	s.served = true
	s.peers = make(map[string]*udpPeer)
	s.mu.Unlock()
	// Serve 返回后清空运行态,Addr() 归零并允许修正配置后重新 Serve
	defer func() {
		s.mu.Lock()
		s.conn = nil
		s.peers = nil
		s.served = false
		s.mu.Unlock()
	}()

	conn, err := net.ListenPacket("udp", opt.Addr)
	if err != nil {
		return fmt.Errorf("udp server: listen %s: %w", opt.Addr, err)
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	log := serverLog(s.log)
	log.Info("udp server listening", "addr", conn.LocalAddr().String())

	// 事件回调:handler 按需实现 OnPeerConnect/OnPeerDisconnect,未实现则跳过
	onConnect, _ := h.(UDPOnConnect)
	onDisconnect, _ := h.(UDPOnDisconnect)

	// 静默对端清扫:定期清理超 TTL 未活跃的对端并触发离线事件,
	// 使完全静默的对端也能被感知,不依附于后续收包
	go s.reapIdlePeers(ctx, onDisconnect, log)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	var wg sync.WaitGroup
	buf := make([]byte, 65535)
	var sem chan struct{}
	if opt.MaxConcurrent > 0 {
		sem = make(chan struct{}, opt.MaxConcurrent)
	}
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				s.waitUDP(&wg, opt.ShutdownTimeout, log)
				log.Info("udp server shutdown", "addr", conn.LocalAddr().String())
				return nil
			default:
			}
			log.Error("udp server read failed", "err", err)
			return fmt.Errorf("udp server: read: %w", err)
		}

		if sem != nil {
			select {
			case sem <- struct{}{}:
			default:
				log.Warn("udp server 并发已达上限,丢弃报文",
					"max", opt.MaxConcurrent, "remote", addr.String())
				continue
			}
		}

		added, pruned := s.recordPeer(addr)
		for _, pa := range pruned {
			if onDisconnect != nil {
				safeEvent(log, "OnPeerDisconnect", func() { onDisconnect.OnPeerDisconnect(pa) })
			}
		}
		if added && onConnect != nil {
			safeEvent(log, "OnPeerConnect", func() { onConnect.OnPeerConnect(addr) })
		}

		pkt := UDPPacket{Data: append([]byte(nil), buf[:n]...), Addr: addr}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}
			defer func() {
				if e := recover(); e != nil {
					log.Error("udp server handler panic", "err", e, "remote", pkt.Addr.String())
				}
			}()
			resp, err := h.ServeUDP(ctx, pkt)
			if err != nil {
				log.Error("udp server handler failed", "err", err)
				return
			}
			if resp != nil {
				if _, err := conn.WriteTo(resp, pkt.Addr); err != nil {
					log.Error("udp server write failed", "err", err)
				}
			}
		}()
	}
}

func (s *UDPServer) waitUDP(wg *sync.WaitGroup, timeout time.Duration, log *slog.Logger) {
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Warn("udp server shutdown timeout,部分报文处理未完成")
	}
}
