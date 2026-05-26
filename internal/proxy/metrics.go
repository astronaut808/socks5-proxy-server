package proxy

import (
	"context"
	"errors"
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

type Metrics struct {
	accepted *expvar.Int
	active   *expvar.Int
}

var defaultMetrics = &Metrics{
	accepted: expvar.NewInt("socks5_accepted_connections_total"),
	active:   expvar.NewInt("socks5_active_connections"),
}

func (m *Metrics) Accepted() {
	m.accepted.Add(1)
	m.active.Add(1)
}

func (m *Metrics) Closed() {
	m.active.Add(-1)
}

func startMetrics(ctx context.Context, addr string, logger *log.Logger) error {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("METRICS_ADDR must be a host:port address: %w", err)
	}

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on metrics address %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("Metrics shutdown error: %v", err)
		}
	}()

	go func() {
		logger.Printf("Metrics listening on %s", addr)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("Metrics server error: %v", err)
		}
	}()

	return nil
}
