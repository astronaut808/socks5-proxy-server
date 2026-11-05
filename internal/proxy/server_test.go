package proxy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"strconv"
	"testing"
	"time"
)

type testProxy struct {
	listeners []net.Listener
	cancel    context.CancelFunc
	done      chan struct{}
}

func startTestProxy(t *testing.T, cfg Config) *testProxy {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	logger := log.New(io.Discard, "", 0)
	server, err := buildServer(ctx, cfg, logger)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	listeners, err := openListeners(cfg, nil, logger)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	p := &testProxy{listeners: listeners, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(p.done)
		if err := serve(ctx, server, listeners, logger); err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-p.done:
		case <-time.After(8 * time.Second):
			t.Error("server shutdown timed out")
		}
	})
	return p
}

func testClient(t *testing.T, addr string, tlsConfig *tls.Config) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if tlsConfig != nil {
		secured := tls.Client(conn, tlsConfig)
		if err := secured.Handshake(); err != nil {
			t.Fatal(err)
		}
		conn = secured
	}
	return conn
}

func authenticateTestClient(t *testing.T, conn net.Conn, user, password string) byte {
	t.Helper()
	method := byte(0)
	if user != "" {
		method = 2
	}
	writeTestBytes(t, conn, []byte{5, 1, method})
	reply := readTestBytes(t, conn, 2)
	if reply[0] != 5 || reply[1] != method {
		t.Fatalf("auth method reply: %v", reply)
	}
	if method == 0 {
		return 0
	}
	request := append([]byte{1, byte(len(user))}, []byte(user)...)
	request = append(request, byte(len(password)))
	request = append(request, []byte(password)...)
	writeTestBytes(t, conn, request)
	return readTestBytes(t, conn, 2)[1]
}

func connectTestClient(t *testing.T, conn net.Conn, target string, domain bool) byte {
	t.Helper()
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	request := []byte{5, 1, 0}
	if domain {
		request = append(request, 3, byte(len(host)))
		request = append(request, []byte(host)...)
	} else {
		request = append(request, 1)
		request = append(request, net.ParseIP(host).To4()...)
	}
	request = binary.BigEndian.AppendUint16(request, uint16(port))
	writeTestBytes(t, conn, request)
	reply := readTestBytes(t, conn, 4)
	switch reply[3] {
	case 1:
		readTestBytes(t, conn, 6)
	case 4:
		readTestBytes(t, conn, 18)
	default:
		t.Fatalf("unexpected reply address: %v", reply)
	}
	return reply[1]
}

func writeTestBytes(t *testing.T, conn net.Conn, data []byte) {
	t.Helper()
	if _, err := conn.Write(data); err != nil {
		t.Fatal(err)
	}
}

func readTestBytes(t *testing.T, conn net.Conn, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if _, err := io.ReadFull(conn, data); err != nil {
		t.Fatal(err)
	}
	return data
}

func testUpstream(t *testing.T, handle func(net.Conn)) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		handle(conn)
	}()
	t.Cleanup(func() { _ = ln.Close(); <-done })
	return ln
}

func TestSOCKS5Transports(t *testing.T) {
	for _, mode := range []string{"plain", "authenticated", "tls12", "tls13", "mtls"} {
		t.Run(mode, func(t *testing.T) {
			cfg := validConfig()
			cfg.Port = "0"
			cfg.RequireAuth = mode != "plain"
			var clientTLS *tls.Config
			if mode == "tls12" || mode == "tls13" || mode == "mtls" {
				var cert tls.Certificate
				var roots *x509.CertPool
				cfg.TLS, cert, roots = testCertificates(t)
				version := uint16(tls.VersionTLS13)
				if mode == "tls12" {
					version = tls.VersionTLS12
				}
				clientTLS = &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: version, MaxVersion: version}
				if mode == "mtls" {
					cfg.TLS.ClientAuth = true
					clientTLS.Certificates = []tls.Certificate{cert}
				}
			}
			upstream := testUpstream(t, func(conn net.Conn) { _, _ = io.Copy(conn, conn) })
			p := startTestProxy(t, cfg)
			conn := testClient(t, p.listeners[0].Addr().String(), clientTLS)
			user, password := "", ""
			if cfg.RequireAuth {
				user, password = cfg.User, cfg.Password
			}
			if status := authenticateTestClient(t, conn, user, password); status != 0 {
				t.Fatalf("auth: %d", status)
			}
			if status := connectTestClient(t, conn, upstream.Addr().String(), false); status != 0 {
				t.Fatalf("connect: %d", status)
			}
			payload := []byte("socks5 regression test")
			writeTestBytes(t, conn, payload)
			if got := readTestBytes(t, conn, len(payload)); !bytes.Equal(got, payload) {
				t.Fatalf("echo: %q", got)
			}
		})
	}
}

func TestSOCKS5HalfClose(t *testing.T) {
	upstream := testUpstream(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("response"))
		_ = conn.(*net.TCPConn).CloseWrite()
		data, err := io.ReadAll(conn)
		if err != nil || string(data) != "after EOF" {
			t.Errorf("reverse stream: %q, %v", data, err)
		}
	})
	cfg := validConfig()
	cfg.Port = "0"
	cfg.RequireAuth = false
	p := startTestProxy(t, cfg)
	conn := testClient(t, p.listeners[0].Addr().String(), nil)
	authenticateTestClient(t, conn, "", "")
	if status := connectTestClient(t, conn, upstream.Addr().String(), false); status != 0 {
		t.Fatalf("connect: %d", status)
	}
	data, err := io.ReadAll(conn)
	if err != nil || string(data) != "response" {
		t.Fatalf("forward EOF: %q, %v", data, err)
	}
	writeTestBytes(t, conn, []byte("after EOF"))
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatal(err)
	}
}

func TestSOCKS5RejectsBadPassword(t *testing.T) {
	cfg := validConfig()
	cfg.Port = "0"
	p := startTestProxy(t, cfg)
	conn := testClient(t, p.listeners[0].Addr().String(), nil)
	if authenticateTestClient(t, conn, cfg.User, "wrong") == 0 {
		t.Fatal("bad password accepted")
	}
}

func TestSOCKS5DomainPolicy(t *testing.T) {
	for _, allow := range []bool{true, false} {
		t.Run(strconv.FormatBool(allow), func(t *testing.T) {
			cfg := validConfig()
			cfg.Port = "0"
			cfg.RequireAuth = false
			cfg.AllowedFQDN = "^denied\\.example$"
			if allow {
				cfg.AllowedFQDN = "^localhost$"
			}
			upstream := testUpstream(t, func(conn net.Conn) {})
			_, port, _ := net.SplitHostPort(upstream.Addr().String())
			p := startTestProxy(t, cfg)
			conn := testClient(t, p.listeners[0].Addr().String(), nil)
			authenticateTestClient(t, conn, "", "")
			status := connectTestClient(t, conn, net.JoinHostPort("localhost", port), true)
			if (status == 0) != allow {
				t.Fatalf("status: %d, allow: %v", status, allow)
			}
		})
	}
}

func sendTestCommand(t *testing.T, conn net.Conn, command byte) byte {
	t.Helper()
	// Minimal request: IPv4 destination 0.0.0.0:0.
	writeTestBytes(t, conn, []byte{5, command, 0, 1, 0, 0, 0, 0, 0, 0})
	reply := readTestBytes(t, conn, 4)
	if reply[0] != 5 {
		t.Fatalf("unexpected reply version: %v", reply)
	}
	if reply[3] == 1 {
		readTestBytes(t, conn, 6)
	}
	return reply[1]
}

func TestSOCKS5UDPAssociate(t *testing.T) {
	const (
		success     = 0
		ruleFailure = 2
	)
	tests := []struct {
		name      string
		enableUDP bool
		want      byte
	}{
		// Rejected by the rule layer when disabled (the default).
		{name: "disabled", enableUDP: false, want: ruleFailure},
		// Allowed by the rule layer when enabled; things-go/go-socks5
		// implements UDP ASSOCIATE and replies with the bound UDP address.
		{name: "enabled", enableUDP: true, want: success},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Port = "0"
			cfg.RequireAuth = false
			cfg.EnableUDP = tt.enableUDP
			p := startTestProxy(t, cfg)
			conn := testClient(t, p.listeners[0].Addr().String(), nil)
			authenticateTestClient(t, conn, "", "")
			if status := sendTestCommand(t, conn, 3); status != tt.want {
				t.Fatalf("udp associate status: %d, want %d", status, tt.want)
			}
		})
	}
}

func TestRunClosesMetricsOnStartupError(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()
	cfg := validConfig()
	cfg.MetricsAddr = addr
	cfg.AllowedFQDN = "["
	cfg.RequireAuth = false
	if err := Run(context.Background(), cfg, log.New(io.Discard, "", 0)); err == nil {
		t.Fatal("expected startup error")
	}
	probe, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("metrics port leaked: %v", err)
	}
	_ = probe.Close()
}

func TestUpstreamCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u := upstreamNetwork{ctx: ctx, timeout: time.Second}
	if _, err := u.Dial(context.Background(), "tcp", "127.0.0.1:1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("dial: %v", err)
	}
	if _, _, err := u.Resolve(context.Background(), "cancelled.invalid"); !errors.Is(err, context.Canceled) {
		t.Fatalf("resolve: %v", err)
	}
}

func testCertificates(t *testing.T) (TLSConfig, tls.Certificate, *x509.CertPool) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("load test CA")
	}
	cfg := TLSConfig{Enabled: true, Port: "0", MinVersion: "1.2", CertFile: writeTempFile(t, "cert.pem", string(certPEM)), KeyFile: writeTempFile(t, "key.pem", string(keyPEM))}
	cfg.ClientCAFile = cfg.CertFile
	return cfg, cert, roots
}
