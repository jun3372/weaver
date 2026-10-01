package weaver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// 服务组件默认优雅关闭等待时间。
const defaultShutdownTimeout = 5 * time.Second

// HTTPOption 是 HTTPServer 的配置项,通过 conf tag 注入。
type HTTPOption struct {
	Addr            string        // 监听地址,如 ":8080";Serve 时读取,修改需重启
	ShutdownTimeout time.Duration // 优雅关闭等待时间,缺省 5s
}

// TCPOption 是 TCPServer 的配置项,通过 conf tag 注入。
type TCPOption struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// UDPOption 是 UDPServer 的配置项,通过 conf tag 注入。
type UDPOption struct {
	Addr            string
	ShutdownTimeout time.Duration
}

// TCPHandler 处理单条 TCP 连接,conn 由框架在关闭时负责回收。
type TCPHandler interface {
	ServeTCP(ctx context.Context, conn net.Conn)
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

// HTTPServer 内嵌于组件,提供声明式 HTTP 服务:配置注入监听地址,Serve 挂载
// handler 并长驻阻塞,ctx 结束后优雅关闭。同一组件可声明多个具名实例。
type HTTPServer struct {
	mu     sync.RWMutex
	config HTTPOption

	srv *http.Server
	ln  net.Listener
}

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
	s.srv = &http.Server{Handler: h}
	s.mu.Unlock()

	timeout := opt.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()
	slog.Default().Info("http server listening", "addr", ln.Addr().String())

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Default().Error("http server exited", "err", err)
			return err
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		slog.Default().Error("http server shutdown failed", "err", err)
		return err
	}
	slog.Default().Info("http server shutdown", "addr", opt.Addr)
	return nil
}

// TCPServer 内嵌于组件,提供声明式 TCP 服务:accept 循环、连接管理与优雅关闭
// 由框架负责,用户只需实现 TCPHandler。
type TCPServer struct {
	mu     sync.RWMutex
	config TCPOption

	ln     net.Listener
	conns  map[net.Conn]struct{}
	served bool
}

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
	s.conns = make(map[net.Conn]struct{})
	s.mu.Unlock()

	ln, err := net.Listen("tcp", opt.Addr)
	if err != nil {
		s.mu.Lock()
		s.served = false // 允许修正配置后重试
		s.mu.Unlock()
		return fmt.Errorf("tcp server: listen %s: %w", opt.Addr, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	slog.Default().Info("tcp server listening", "addr", ln.Addr().String())

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				s.waitConns(&wg, s.conns, opt.ShutdownTimeout)
				slog.Default().Info("tcp server shutdown", "addr", ln.Addr().String())
				return nil
			default:
			}
			slog.Default().Error("tcp server accept failed", "err", err)
			return fmt.Errorf("tcp server: accept: %w", err)
		}

		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				_ = conn.Close()
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
			}()
			h.ServeTCP(ctx, conn)
		}()
	}
}

// waitConns 关闭全部活跃连接以解除 handler 阻塞,并限时等待其退出。
func (s *TCPServer) waitConns(wg *sync.WaitGroup, conns map[net.Conn]struct{}, timeout time.Duration) {
	s.mu.Lock()
	for conn := range conns {
		_ = conn.Close()
	}
	s.mu.Unlock()

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
		slog.Default().Warn("tcp server shutdown timeout,部分连接未能及时退出")
	}
}

// UDPServer 内嵌于组件,提供声明式 UDP 服务:读循环、派发与回包由框架负责,
// 用户只需实现 UDPPacketHandler。
type UDPServer struct {
	mu     sync.RWMutex
	config UDPOption

	conn   net.PacketConn
	served bool
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
	s.mu.Unlock()

	conn, err := net.ListenPacket("udp", opt.Addr)
	if err != nil {
		s.mu.Lock()
		s.served = false // 允许修正配置后重试
		s.mu.Unlock()
		return fmt.Errorf("udp server: listen %s: %w", opt.Addr, err)
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	slog.Default().Info("udp server listening", "addr", conn.LocalAddr().String())

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	var wg sync.WaitGroup
	buf := make([]byte, 65535)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				s.waitUDP(&wg, opt.ShutdownTimeout)
				slog.Default().Info("udp server shutdown", "addr", conn.LocalAddr().String())
				return nil
			default:
			}
			slog.Default().Error("udp server read failed", "err", err)
			return fmt.Errorf("udp server: read: %w", err)
		}

		pkt := UDPPacket{Data: append([]byte(nil), buf[:n]...), Addr: addr}
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := h.ServeUDP(ctx, pkt)
			if err != nil {
				slog.Default().Error("udp server handler failed", "err", err)
				return
			}
			if resp != nil {
				if _, err := conn.WriteTo(resp, pkt.Addr); err != nil {
					slog.Default().Error("udp server write failed", "err", err)
				}
			}
		}()
	}
}

func (s *UDPServer) waitUDP(wg *sync.WaitGroup, timeout time.Duration) {
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
		slog.Default().Warn("udp server shutdown timeout,部分报文处理未完成")
	}
}
