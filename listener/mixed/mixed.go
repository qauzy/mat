package mixed

import (
	"net"

	"github.com/qauzy/mat/adapter/inbound"
	"github.com/qauzy/mat/common/lru"
	N "github.com/qauzy/mat/common/net"
	C "github.com/qauzy/mat/constant"
	"github.com/qauzy/mat/listener/http"
	"github.com/qauzy/mat/listener/socks"
	"github.com/qauzy/mat/transport/socks4"
	"github.com/qauzy/mat/transport/socks5"
)

type Listener struct {
	listener net.Listener
	addr     string
	cache    *lru.LruCache[string, bool]
	closed   bool
}

// RawAddress implements C.Listener
func (l *Listener) RawAddress() string {
	return l.addr
}

// Address implements C.Listener
func (l *Listener) Address() string {
	return l.listener.Addr().String()
}

// Close implements C.Listener
func (l *Listener) Close() error {
	l.closed = true
	return l.listener.Close()
}

func New(addr string, tunnel C.Tunnel, additions ...inbound.Addition) (*Listener, error) {
	isDefault := false
	if len(additions) == 0 {
		isDefault = true
		additions = []inbound.Addition{
			inbound.WithInName("DEFAULT-MIXED"),
			inbound.WithSpecialRules(""),
		}
	}
	l, err := inbound.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	ml := &Listener{
		listener: l,
		addr:     addr,
		cache:    lru.New[string, bool](lru.WithAge[string, bool](30)),
	}
	go func() {
		for {
			c, err := ml.listener.Accept()
			if err != nil {
				if ml.closed {
					break
				}
				continue
			}
			if isDefault { // only apply on default listener
				if !inbound.IsRemoteAddrDisAllowed(c.RemoteAddr()) {
					_ = c.Close()
					continue
				}
			}
			go handleConn(c, tunnel, ml.cache, additions...)
		}
	}()

	return ml, nil
}

func handleConn(conn net.Conn, tunnel C.Tunnel, cache *lru.LruCache[string, bool], additions ...inbound.Addition) {
	N.TCPKeepAlive(conn)

	bufConn := N.NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		return
	}

	switch head[0] {
	case socks4.Version:
		socks.HandleSocks4(bufConn, tunnel, additions...)
	case socks5.Version:
		socks.HandleSocks5(bufConn, tunnel, additions...)
	default:
		http.HandleConn(bufConn, tunnel, cache, additions...)
	}
}
