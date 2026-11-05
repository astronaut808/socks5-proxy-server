package proxy

import (
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

func TestLimitedConnCloseIsIdempotent(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()

	released := make(chan struct{}, 1)
	conn := &limitedConn{
		Conn: client,
		release: func() {
			released <- struct{}{}
		},
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	_ = conn.Close()

	select {
	case <-released:
	default:
		t.Fatal("release was not called")
	}
	select {
	case <-released:
		t.Fatal("release called twice")
	default:
	}
}

func TestLimitedListenerCloseUnblocksAccept(t *testing.T) {
	for _, blockedBy := range []string{"connections", "rate"} {
		t.Run(blockedBy, func(t *testing.T) {
			ln := newLimitedListener(&pipeListener{once: true}, 1, 1, time.Second, nil)
			if blockedBy == "connections" {
				ln.limit <- struct{}{}
			} else {
				ln.rateLimiter.AllowN(time.Now(), 2)
			}
			done := make(chan error, 1)
			go func() { _, err := ln.Accept(); done <- err }()
			if err := ln.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, net.ErrClosed) {
					t.Fatalf("Accept: %v", err)
				}
			case <-time.After(200 * time.Millisecond):
				if blockedBy == "connections" {
					<-ln.limit
				}
				t.Fatalf("Close did not unblock Accept (Accept returned: %v)", <-done)
			}
		})
	}
}

func TestDeadlineConnReadDeadline(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	conn := deadlineConn{Conn: client, timeout: 10 * time.Millisecond}
	done := make(chan error, 1)
	go func() {
		_, err := conn.Read([]byte{0})
		done <- err
	}()

	select {
	case err := <-done:
		var netErr net.Error
		ok := errors.As(err, &netErr)
		if !ok || !netErr.Timeout() {
			t.Fatalf("error = %v, want timeout", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("read did not time out")
	}
}

func TestLimitedListenerAcceptWrapsConnection(t *testing.T) {
	stats := &fakeStats{}
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	listener := &pipeListener{conn: server}
	limited := newLimitedListener(listener, 1, 100, time.Second, stats)

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := limited.Accept()
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		accepted <- conn
	}()

	conn := <-accepted
	if stats.accepted != 1 {
		t.Fatalf("accepted stats = %d, want 1", stats.accepted)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close accepted conn: %v", err)
	}
	if stats.closed != 1 {
		t.Fatalf("closed stats = %d, want 1", stats.closed)
	}
}

type remoteAddrConn struct {
	net.Conn
	remote net.Addr
	closed chan struct{}
}

func (c *remoteAddrConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *remoteAddrConn) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return c.Conn.Close()
}

type queueListener struct {
	conns []net.Conn
}

func (l *queueListener) Accept() (net.Conn, error) {
	if len(l.conns) == 0 {
		return nil, net.ErrClosed
	}
	conn := l.conns[0]
	l.conns = l.conns[1:]
	return conn, nil
}

func (l *queueListener) Close() error {
	return nil
}

func (l *queueListener) Addr() net.Addr {
	return pipeAddr("queue")
}

func newRemoteAddrConn(t *testing.T, remote net.Addr) *remoteAddrConn {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	return &remoteAddrConn{
		Conn:   client,
		remote: remote,
		closed: make(chan struct{}),
	}
}

func TestAllowedIPsListenerAcceptsAllowedIP(t *testing.T) {
	conn := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1234})
	listener := newAllowedIPsListener(
		&queueListener{conns: []net.Conn{conn}},
		[]net.IP{net.ParseIP("192.0.2.1")},
		log.New(io.Discard, "", 0),
	)

	accepted, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted != conn {
		t.Fatal("accepted connection is not the queued one")
	}
	select {
	case <-conn.closed:
		t.Fatal("allowed connection was closed")
	default:
	}
}

func TestAllowedIPsListenerRejectsDisallowedIP(t *testing.T) {
	rejected := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("198.51.100.1"), Port: 1})
	allowed := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 2})
	listener := newAllowedIPsListener(
		&queueListener{conns: []net.Conn{rejected, allowed}},
		[]net.IP{net.ParseIP("192.0.2.1")},
		log.New(io.Discard, "", 0),
	)

	accepted, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted != allowed {
		t.Fatal("accepted connection is not the allowed one")
	}
	select {
	case <-rejected.closed:
	case <-time.After(time.Second):
		t.Fatal("rejected connection was not closed")
	}
}

func TestAllowedIPsListenerRejectsMalformedRemoteAddr(t *testing.T) {
	rejected := newRemoteAddrConn(t, pipeAddr("not-an-ip"))
	allowed := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 3})
	listener := newAllowedIPsListener(
		&queueListener{conns: []net.Conn{rejected, allowed}},
		[]net.IP{net.ParseIP("192.0.2.1")},
		log.New(io.Discard, "", 0),
	)

	accepted, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted != allowed {
		t.Fatal("accepted connection is not the allowed one")
	}
	select {
	case <-rejected.closed:
	case <-time.After(time.Second):
		t.Fatal("malformed connection was not closed")
	}
}

func TestAllowedIPsListenerWithoutLogger(t *testing.T) {
	rejected := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("198.51.100.1"), Port: 1})
	listener := newAllowedIPsListener(
		&queueListener{conns: []net.Conn{rejected}},
		[]net.IP{net.ParseIP("192.0.2.1")},
		nil,
	)
	if _, err := listener.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("accept: %v, want %v", err, net.ErrClosed)
	}
	select {
	case <-rejected.closed:
	case <-time.After(time.Second):
		t.Fatal("rejected connection was not closed")
	}
}

// TestWrapListenerFiltersDisallowedIPsBeforeRateLimit proves the ALLOWED_IPS
// filter runs inside the rate/connection limiter: rejected source IPs must
// not consume rate-limit tokens or connection slots and must not increment
// accepted-connection metrics, while allowed IPs still do.
func TestWrapListenerFiltersDisallowedIPsBeforeRateLimit(t *testing.T) {
	rejected1 := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("198.51.100.1"), Port: 1})
	rejected2 := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("198.51.100.2"), Port: 2})
	allowed := newRemoteAddrConn(t, &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 3})
	stats := &fakeStats{}
	cfg := Config{
		AllowedIPs:     []string{"192.0.2.1"},
		MaxConnections: 10,
		Timeout:        1,
		RateLimit:      1,
	}
	listener := wrapListener(
		&queueListener{conns: []net.Conn{rejected1, rejected2, allowed}},
		cfg, stats, log.New(io.Discard, "", 0),
	)

	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if conn.RemoteAddr() != allowed.remote {
		t.Fatalf("accepted remote = %v, want %v", conn.RemoteAddr(), allowed.remote)
	}
	if stats.accepted != 1 {
		t.Fatalf("accepted stats = %d, want 1 (disallowed IPs must not count)", stats.accepted)
	}
	for _, rejected := range []*remoteAddrConn{rejected1, rejected2} {
		select {
		case <-rejected.closed:
		default:
			t.Fatalf("rejected connection from %s was not closed", rejected.remote)
		}
	}
	limited, ok := listener.(*limitedListener)
	if !ok {
		t.Fatalf("wrapListener returned %T, want *limitedListener", listener)
	}
	if got := len(limited.limit); got != 1 {
		t.Fatalf("connection slots held = %d, want 1 (only the allowed connection)", got)
	}
	// Burst is RateLimit*2 = 2; exactly one token may be consumed per Accept
	// call regardless of how many disallowed IPs were rejected inside it.
	if tokens := limited.rateLimiter.Tokens(); tokens < 0.99 {
		t.Fatalf("rate-limit tokens = %f, want >= 0.99 (rejected IPs must not burn tokens)", tokens)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close accepted conn: %v", err)
	}
	if stats.closed != 1 {
		t.Fatalf("closed stats = %d, want 1", stats.closed)
	}
	if got := len(limited.limit); got != 0 {
		t.Fatalf("connection slots held after close = %d, want 0", got)
	}
}

type fakeStats struct {
	accepted int
	closed   int
}

func (s *fakeStats) Accepted() {
	s.accepted++
}

func (s *fakeStats) Closed() {
	s.closed++
}

type pipeListener struct {
	conn net.Conn
	once bool
}

func (l *pipeListener) Accept() (net.Conn, error) {
	if l.once {
		return nil, net.ErrClosed
	}
	l.once = true
	return l.conn, nil
}

func (l *pipeListener) Close() error {
	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return pipeAddr("pipe")
}

type pipeAddr string

func (a pipeAddr) Network() string {
	return string(a)
}

func (a pipeAddr) String() string {
	return string(a)
}
