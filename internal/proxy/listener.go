package proxy

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type listenerStats interface {
	Accepted()
	Closed()
}

type limitedListener struct {
	net.Listener
	limit       chan struct{}
	timeout     time.Duration
	rateLimiter *rate.Limiter
	stats       listenerStats
	ctx         context.Context
	cancel      context.CancelFunc
}

func newLimitedListener(ln net.Listener, maxConnections, rateLimit int, timeout time.Duration, stats listenerStats) *limitedListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &limitedListener{
		Listener:    ln,
		limit:       make(chan struct{}, maxConnections),
		timeout:     timeout,
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimit), rateLimit*2),
		stats:       stats,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (l *limitedListener) Accept() (net.Conn, error) {
	if err := l.rateLimiter.Wait(l.ctx); err != nil {
		return nil, net.ErrClosed
	}

	select {
	case l.limit <- struct{}{}:
	case <-l.ctx.Done():
		return nil, net.ErrClosed
	}
	if l.ctx.Err() != nil {
		<-l.limit
		return nil, net.ErrClosed
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		<-l.limit
		return nil, err
	}

	if l.stats != nil {
		l.stats.Accepted()
	}
	return &limitedConn{
		Conn: deadlineConn{Conn: conn, timeout: l.timeout},
		release: func() {
			if l.stats != nil {
				l.stats.Closed()
			}
			<-l.limit
		},
	}, nil
}

func (l *limitedListener) Close() error {
	l.cancel()
	return l.Listener.Close()
}

// allowedIPsListener rejects connections from source IPs outside the
// ALLOWED_IPS list before the SOCKS5 handshake begins, replacing the
// SetIPWhitelist hook of the previous go-socks5 library. It is the
// innermost listener wrapper so rejected connections never consume
// rate-limit tokens, connection slots, or accepted-connection metrics.
type allowedIPsListener struct {
	net.Listener
	allowed []net.IP
	logger  *log.Logger
}

func newAllowedIPsListener(ln net.Listener, allowed []net.IP, logger *log.Logger) *allowedIPsListener {
	return &allowedIPsListener{Listener: ln, allowed: allowed, logger: logger}
}

func (l *allowedIPsListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		remote := "<unknown>"
		if addr := conn.RemoteAddr(); addr != nil {
			remote = addr.String()
		}
		host, _, err := net.SplitHostPort(remote)
		if err != nil {
			host = remote
		}
		ip := net.ParseIP(host)
		if ip != nil && l.isAllowed(ip) {
			return conn, nil
		}
		if l.logger != nil {
			l.logger.Printf("Rejected connection from disallowed IP: %s", cleanLogValue(remote))
		}
		_ = conn.Close()
	}
}

func (l *allowedIPsListener) isAllowed(ip net.IP) bool {
	for _, allowed := range l.allowed {
		if ip.Equal(allowed) {
			return true
		}
	}
	return false
}

type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c deadlineConn) Read(b []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(b)
}

func (c deadlineConn) Write(b []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(b)
}

func (c deadlineConn) CloseWrite() error {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	return closeWrite(c.Conn)
}

func closeWrite(conn net.Conn) error {
	if conn, ok := conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}

type limitedConn struct {
	net.Conn
	release  func()
	once     sync.Once
	closeErr error
}

func (c *limitedConn) Close() error {
	c.once.Do(func() {
		c.closeErr = c.Conn.Close()
		c.release()
	})
	return c.closeErr
}

func (c *limitedConn) CloseWrite() error {
	return closeWrite(c.Conn)
}
