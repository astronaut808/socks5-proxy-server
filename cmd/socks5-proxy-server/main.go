package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/astronaut808/socks5-proxy-server/internal/proxy"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := proxy.LoadConfig()
	if err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := proxy.Run(ctx, cfg, logger); err != nil {
		logger.Fatalf("Server stopped with error: %v", err)
	}
}
