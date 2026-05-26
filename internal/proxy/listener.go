package proxy

import (
	"context"
	"fmt"
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
}

func newLimitedListener(ln net.Listener, maxConnections, rateLimit int, timeout time.Duration, stats listenerStats) *limitedListener {
	return &limitedListener{
		Listener:    ln,
		limit:       make(chan struct{}, maxConnections),
		timeout:     timeout,
		rateLimiter: rate.NewLimiter(rate.Limit(rateLimit), rateLimit*2),
		stats:       stats,
	}
}

func (l *limitedListener) Accept() (net.Conn, error) {
	if err := l.rateLimiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("wait for rate limit: %w", err)
	}

	l.limit <- struct{}{}
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

type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c deadlineConn) Read(b []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(c.timeout))
	return c.Conn.Read(b)
}

func (c deadlineConn) Write(b []byte) (int, error) {
	_ = c.SetWriteDeadline(time.Now().Add(c.timeout))
	return c.Conn.Write(b)
}

type limitedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *limitedConn) Close() error {
	c.once.Do(c.release)
	return c.Conn.Close()
}
