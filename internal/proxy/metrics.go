package proxy

import (
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

func startMetrics(addr string, logger *log.Logger) (func(), error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("METRICS_ADDR must be a host:port address: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/debug/vars", expvar.Handler())
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on metrics address %s: %w", addr, err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		logger.Printf("Metrics listening on %s", addr)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("Metrics server error: %v", err)
		}
	}()

	return func() {
		_ = server.Close()
		<-done
	}, nil
}
