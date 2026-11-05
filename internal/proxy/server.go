package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	socks5 "github.com/things-go/go-socks5"
)

func Run(ctx context.Context, cfg Config, logger *log.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if cfg.MetricsAddr != "" {
		stopMetrics, err := startMetrics(cfg.MetricsAddr, logger)
		if err != nil {
			return err
		}
		defer stopMetrics()
	}

	server, err := buildServer(ctx, cfg, logger)
	if err != nil {
		return err
	}

	listeners, err := openListeners(cfg, defaultMetrics, logger)
	if err != nil {
		return err
	}
	defer closeAll(listeners)

	return serve(ctx, server, listeners, logger)
}

func buildServer(ctx context.Context, cfg Config, logger *log.Logger) (*socks5.Server, error) {
	upstream := upstreamNetwork{ctx: ctx, timeout: cfg.TimeoutDuration()}
	options := []socks5.Option{
		socks5.WithLogger(socks5.NewLogger(log.New(logger.Writer(), "[SOCKS5] ", logger.Flags()))),
		socks5.WithDial(upstream.Dial),
		socks5.WithResolver(upstream),
	}

	if cfg.RequireAuth {
		store, err := NewAccountStore(
			cfg.Accounts,
			cfg.User,
			cfg.Password,
			cfg.MaxAuthFailures,
			cfg.LockoutDuration(),
		)
		if err != nil {
			return nil, fmt.Errorf("configure authentication: %w", err)
		}
		options = append(options, socks5.WithCredential(store))
	} else {
		logger.Println("Warning: proxy running without authentication")
	}

	var rules socks5.RuleSet
	if cfg.AllowedFQDN != "" {
		policy, err := NewDestinationPolicy(cfg.AllowedFQDN, logger)
		if err != nil {
			return nil, fmt.Errorf("configure destination policy: %w", err)
		}
		rules = policy
	}
	options = append(options, socks5.WithRule(commandPolicy{
		allowUDP: cfg.EnableUDP,
		next:     rules,
		logger:   logger,
	}))

	return socks5.NewServer(options...), nil
}

func openListeners(cfg Config, stats listenerStats, logger *log.Logger) ([]net.Listener, error) {
	var listeners []net.Listener

	if cfg.PlainEnabled() {
		listener, err := net.Listen("tcp", cfg.PlainAddr())
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", cfg.PlainAddr(), err)
		}
		listeners = append(listeners, wrapListener(listener, cfg, stats, logger))
	}

	if cfg.TLS.Enabled {
		tlsConfig, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			closeAll(listeners)
			return nil, err
		}

		listener, err := tls.Listen("tcp", cfg.TLSAddr(), tlsConfig)
		if err != nil {
			closeAll(listeners)
			return nil, fmt.Errorf("listen on TLS %s: %w", cfg.TLSAddr(), err)
		}
		listeners = append(listeners, wrapListener(listener, cfg, stats, logger))
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("no listeners enabled")
	}
	return listeners, nil
}

// wrapListener applies the ALLOWED_IPS source filter as the innermost layer,
// then wraps it with connection limits and rate limiting. Filtering source
// IPs first ensures rejected connections never consume rate-limit tokens or
// connection slots and are not counted as accepted connections. The filter
// replaces the IP whitelist hook of the previous go-socks5 library, which
// has no equivalent in things-go/go-socks5.
func wrapListener(listener net.Listener, cfg Config, stats listenerStats, logger *log.Logger) net.Listener {
	if len(cfg.AllowedIPs) > 0 {
		listener = newAllowedIPsListener(listener, parseIPs(cfg.AllowedIPs), logger)
	}
	return newLimitedListener(
		listener,
		cfg.MaxConnections,
		cfg.RateLimit,
		cfg.TimeoutDuration(),
		stats,
	)
}

func serve(ctx context.Context, server *socks5.Server, listeners []net.Listener, logger *log.Logger) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	connections := make(map[net.Conn]struct{})
	changed := make(chan struct{}, 1)
	errs := make(chan error, len(listeners))
	stop := sync.OnceFunc(func() {
		closeAll(listeners)
	})

	stopCancellation := context.AfterFunc(ctx, stop)
	defer stopCancellation()
	defer stop()

	for _, listener := range listeners {
		wg.Add(1)
		go func(listener net.Listener) {
			defer wg.Done()
			for {
				conn, err := listener.Accept()
				if err != nil {
					if !errors.Is(err, net.ErrClosed) {
						errs <- err
					}
					stop()
					return
				}
				mu.Lock()
				connections[conn] = struct{}{}
				mu.Unlock()
				go func() {
					// ServeConn closes the connection itself (defer in the
					// library); limitedConn.Close stays idempotent regardless.
					_ = server.ServeConn(conn)
					mu.Lock()
					delete(connections, conn)
					mu.Unlock()
					select {
					case changed <- struct{}{}:
					default:
					}
				}()
			}
		}(listener)
	}

	wg.Wait()
	// Give established streams time to finish, then release all client sockets.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
drain:
	for {
		mu.Lock()
		remaining := len(connections)
		mu.Unlock()
		if remaining == 0 {
			break
		}
		select {
		case <-changed:
		case <-deadline.C:
			mu.Lock()
			for conn := range connections {
				_ = conn.Close()
			}
			mu.Unlock()
			break drain
		}
	}
	close(errs)
	for err := range errs {
		return fmt.Errorf("serve SOCKS5: %w", err)
	}
	logger.Println("Server stopped")
	return nil
}

func parseIPs(values []string) []net.IP {
	ips := make([]net.IP, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		ips = append(ips, net.ParseIP(value))
	}
	return ips
}

func closeAll(listeners []net.Listener) {
	for _, listener := range listeners {
		_ = listener.Close()
	}
}
