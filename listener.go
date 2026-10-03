package weaver

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/pkg/errors"
)

// Handler 是 HTTP/TCP/UDP 三种服务 handler 的聚合接口;
// 组件实现其中任意一个即可被 Listener 自动 Serve。
type Handler interface {
	http.Handler
	TCPHandler
	UDPPacketHandler
}

// Listener 聚合服务组件:内部持有 HTTPServer/TCPServer/UDPServer 之一,
// 通过 conf tag 注入配置,启动阶段自动 Serve,无需 Start。
//
// 单协议组件声明 Listener[weaver.Handler] 自动探测;组件实现多个 handler
// 接口时,为每个协议声明具名 Listener 并以具体接口作 H,如:
//
//	type gateway struct {
//	    weaver.Implements[T]
//	    httpL weaver.Listener[http.Handler]            `conf:"http"`
//	    tcpL  weaver.Listener[weaver.TCPHandler]       `conf:"tcp"`
//	    udpL  weaver.Listener[weaver.UDPPacketHandler] `conf:"udp"`
//	}
//
// HTTP 服务的 handler 有三种来源,优先级从高到低:
//  1. Handler 注册的外部框架引擎(gin/echo 等一切实现了 http.Handler 的类型);
//  2. Mux() 惰性创建的标准库 ServeMux(多路由便捷方式);
//  3. 组件自身实现的 ServeHTTP。
type Listener[H any] struct {
	mu      sync.RWMutex
	server  any          // *HTTPServer / *TCPServer / *UDPServer,init 时创建
	handler http.Handler // Handler/Mux 注册的外部 HTTP handler,优先于组件自身

	armed bool // 配置注入成功后为 true,start 阶段仅自动启动已就绪的 listener
}

var (
	httpHandlerType = reflect.TypeFor[http.Handler]()
	tcpHandlerType  = reflect.TypeFor[TCPHandler]()
	udpHandlerType  = reflect.TypeFor[UDPPacketHandler]()
)

// Handler 注册外部框架的 HTTP handler(gin.Engine、echo.Echo 等一切实现了
// http.Handler 的类型),Serve 时优先于组件自身;须在 Init 中调用。
func (l *Listener[H]) Handler(handler http.Handler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handler = handler
}

// Mux 返回惰性创建的 *http.ServeMux 并自动 Set,便于直接注册多路由。
func (l *Listener[H]) Mux() *http.ServeMux {
	l.mu.Lock()
	defer l.mu.Unlock()
	if m, ok := l.handler.(*http.ServeMux); ok {
		return m
	}
	m := http.NewServeMux()
	l.handler = m
	return m
}

// handlerProtocols 返回 t 实现的三种 handler 接口(0~3 个)。
func handlerProtocols(t reflect.Type) []reflect.Type {
	var matched []reflect.Type
	for _, iface := range [...]reflect.Type{httpHandlerType, tcpHandlerType, udpHandlerType} {
		if t.Implements(iface) {
			matched = append(matched, iface)
		}
	}
	return matched
}

func newListenerServer(t reflect.Type) any {
	switch t {
	case httpHandlerType:
		return new(HTTPServer)
	case tcpHandlerType:
		return new(TCPServer)
	case udpHandlerType:
		return new(UDPServer)
	}
	return nil
}

// init 按组件实现与 H 的类型创建内部服务端;由 widget 在装配阶段调用。
// 组件未实现任何 handler 接口时预留 HTTP,待 Init 中 Handler/Mux 注入外部 handler。
func (l *Listener[H]) init(impl any) error {
	h := reflect.TypeFor[H]()
	implType := reflect.TypeOf(impl)

	matched := handlerProtocols(h)
	switch {
	case len(matched) == 0:
		return errors.Errorf("listener: 类型参数 %s 未实现 http.Handler/TCPHandler/UDPPacketHandler 中任一接口", h)
	case len(matched) == 1:
		// 固定协议;组件是否提供 handler 在 serve 时结合 Handler/Mux 校验
	case len(matched) == 3:
		// 聚合类型:自动探测组件实现的接口,多于一个则歧义
		found := handlerProtocols(implType)
		switch len(found) {
		case 0:
			// 未实现任何接口:预留 HTTP,待 Init 中 Handler/Mux 注入外部 handler
			matched = []reflect.Type{httpHandlerType}
		case 1:
			matched = found
		default:
			return errors.Errorf("listener: 组件 %s 同时实现了 %d 种 handler 接口,请为每个协议声明具名 Listener 并以具体接口作类型参数", implType, len(found))
		}
	default:
		return errors.Errorf("listener: 类型参数 %s 实现了 %d 种 handler 接口,请改用具体接口或聚合类型 weaver.Handler", h, len(matched))
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.server != nil {
		return nil // 幂等
	}
	l.server = newListenerServer(matched[0])
	return nil
}

// enabled 报告 listener 是否已注入配置,未就绪的在 start 阶段跳过。
func (l *Listener[H]) enabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.armed
}

// confType 返回内部服务端的配置类型,供 widget 构造注入目标。
func (l *Listener[H]) confType() reflect.Type {
	switch l.server.(type) {
	case *HTTPServer:
		return reflect.TypeFor[HTTPOption]()
	case *TCPServer:
		return reflect.TypeFor[TCPOption]()
	case *UDPServer:
		return reflect.TypeFor[UDPOption]()
	}
	return nil
}

// setConf 整体替换内部服务端配置,由 widget 在配置注入/热更新时调用。
func (l *Listener[H]) setConf(v any) {
	switch s := l.server.(type) {
	case *HTTPServer:
		s.SetConfig(v.(HTTPOption))
	case *TCPServer:
		s.SetConfig(v.(TCPOption))
	case *UDPServer:
		s.SetConfig(v.(UDPOption))
	}
}

// arm 标记配置已注入。
func (l *Listener[H]) arm() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.armed = true
}

// protocol 返回绑定的协议名("http"/"tcp"/"udp"),未初始化返回 ""。
func (l *Listener[H]) protocol() string {
	switch l.server.(type) {
	case *HTTPServer:
		return "http"
	case *TCPServer:
		return "tcp"
	case *UDPServer:
		return "udp"
	}
	return ""
}

// setLog 将组件 logger 透传给内部服务端,由 widget 在装配阶段调用。
func (l *Listener[H]) setLog(log *slog.Logger) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	switch s := l.server.(type) {
	case *HTTPServer:
		s.setLog(log)
	case *TCPServer:
		s.setLog(log)
	case *UDPServer:
		s.setLog(log)
	}
}

// serve 以注册的外部 handler(优先)或组件自身为 handler 长驻启动内部
// 服务端;ctx 结束后优雅关闭。
func (l *Listener[H]) serve(ctx context.Context, impl any) error {
	switch s := l.server.(type) {
	case *HTTPServer:
		l.mu.RLock()
		handler := l.handler
		l.mu.RUnlock()
		if handler == nil {
			h, ok := impl.(http.Handler)
			if !ok {
				return errors.Errorf("listener: 组件 %s 未实现 http.Handler,且未通过 Handler/Mux 注册 handler", reflect.TypeOf(impl))
			}
			handler = h
		}
		return s.Serve(ctx, handler)
	case *TCPServer:
		return s.Serve(ctx, impl.(TCPHandler))
	case *UDPServer:
		return s.Serve(ctx, impl.(UDPPacketHandler))
	}
	return errors.New("listener: 服务端未初始化")
}

// Config 返回注入的服务端配置快照,类型为 HTTPOption/TCPOption/UDPOption 之一
// (与绑定协议对应),经类型断言取用;服务端未初始化(未启用)时返回 nil。
// 配置热更新后返回最新值。
func (l *Listener[H]) Config() any {
	l.mu.RLock()
	defer l.mu.RUnlock()

	switch s := l.server.(type) {
	case *HTTPServer:
		return s.Config()
	case *TCPServer:
		return s.Config()
	case *UDPServer:
		return s.Config()
	}
	return nil
}

// Addr 返回绑定后的实际监听地址,未初始化或未在服务中时返回 ""。
func (l *Listener[H]) Addr() string {
	switch s := l.server.(type) {
	case *HTTPServer:
		return s.Addr()
	case *TCPServer:
		return s.Addr()
	case *UDPServer:
		return s.Addr()
	}
	return ""
}

func (l *Listener[H]) tcpServer() (*TCPServer, error) {
	s, ok := l.server.(*TCPServer)
	if !ok {
		return nil, errors.New("listener: 未绑定 tcp 服务")
	}
	return s, nil
}

func (l *Listener[H]) udpServer() (*UDPServer, error) {
	s, ok := l.server.(*UDPServer)
	if !ok {
		return nil, errors.New("listener: 未绑定 udp 服务")
	}
	return s, nil
}

// Sessions 返回活跃 TCP 会话快照;非 tcp 绑定时返回 nil(错误语义见 Send)。
func (l *Listener[H]) Sessions() []TCPSession {
	s, err := l.tcpServer()
	if err != nil {
		return nil
	}
	return s.Sessions()
}

// Send 向指定 TCP 会话定点发送数据,与 handler 并发写安全。
func (l *Listener[H]) Send(id uint64, data []byte) error {
	s, err := l.tcpServer()
	if err != nil {
		return err
	}
	return s.Send(id, data)
}

// CloseConn 主动断开指定 TCP 会话。
func (l *Listener[H]) CloseConn(id uint64) error {
	s, err := l.tcpServer()
	if err != nil {
		return err
	}
	return s.CloseConn(id)
}

// Peers 返回活跃 UDP 对端快照;非 udp 绑定时返回 nil(错误语义见 SendTo)。
func (l *Listener[H]) Peers() []UDPPeer {
	s, err := l.udpServer()
	if err != nil {
		return nil
	}
	return s.Peers()
}

// SendTo 向指定 UDP 对端定点发送报文。
func (l *Listener[H]) SendTo(addr net.Addr, data []byte) error {
	s, err := l.udpServer()
	if err != nil {
		return err
	}
	return s.SendTo(addr, data)
}

var _ listenerAPI = (*Listener[Handler])(nil)
