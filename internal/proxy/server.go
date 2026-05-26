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

	"github.com/armon/go-socks5"
)

func Run(ctx context.Context, cfg Config, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if cfg.MetricsAddr != "" {
		if err := startMetrics(ctx, cfg.MetricsAddr, logger); err != nil {
			return err
		}
	}

	server, err := buildServer(cfg, logger)
	if err != nil {
		return err
	}

	listeners, err := openListeners(cfg, defaultMetrics)
	if err != nil {
		return err
	}
	defer closeAll(listeners)

	return serve(ctx, server, listeners, logger)
}

func buildServer(cfg Config, logger *log.Logger) (*socks5.Server, error) {
	socksConfig := &socks5.Config{
		Logger: log.New(logger.Writer(), "[SOCKS5] ", logger.Flags()),
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
		socksConfig.AuthMethods = []socks5.Authenticator{
			socks5.UserPassAuthenticator{Credentials: store},
		}
	} else {
		logger.Println("Warning: proxy running without authentication")
	}

	if cfg.AllowedFQDN != "" {
		policy, err := NewDestinationPolicy(cfg.AllowedFQDN, logger)
		if err != nil {
			return nil, fmt.Errorf("configure destination policy: %w", err)
		}
		socksConfig.Rules = policy
	}

	server, err := socks5.New(socksConfig)
	if err != nil {
		return nil, fmt.Errorf("create SOCKS5 server: %w", err)
	}
	if len(cfg.AllowedIPs) > 0 {
		server.SetIPWhitelist(parseIPs(cfg.AllowedIPs))
	}
	return server, nil
}

func openListeners(cfg Config, stats listenerStats) ([]net.Listener, error) {
	var listeners []net.Listener

	if cfg.PlainEnabled() {
		listener, err := net.Listen("tcp", cfg.PlainAddr())
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", cfg.PlainAddr(), err)
		}
		listeners = append(listeners, newLimitedListener(
			listener,
			cfg.MaxConnections,
			cfg.RateLimit,
			cfg.TimeoutDuration(),
			stats,
		))
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
		listeners = append(listeners, newLimitedListener(
			listener,
			cfg.MaxConnections,
			cfg.RateLimit,
			cfg.TimeoutDuration(),
			stats,
		))
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("no listeners enabled")
	}
	return listeners, nil
}

func serve(ctx context.Context, server *socks5.Server, listeners []net.Listener, logger *log.Logger) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(listeners))
	stop := sync.OnceFunc(func() {
		closeAll(listeners)
	})

	go func() {
		<-ctx.Done()
		stop()
	}()

	for _, listener := range listeners {
		wg.Add(1)
		go func(listener net.Listener) {
			defer wg.Done()
			if err := server.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
				errs <- err
				stop()
			}
		}(listener)
	}

	wg.Wait()
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
