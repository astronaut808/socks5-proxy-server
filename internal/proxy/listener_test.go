package proxy

import (
	"errors"
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
