package proxy

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/armon/go-socks5"
)

const maxLogValueLen = 128

type DestinationPolicy struct {
	pattern *regexp.Regexp
	logger  *log.Logger
}

func NewDestinationPolicy(pattern string, logger *log.Logger) (*DestinationPolicy, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile destination pattern: %w", err)
	}
	return &DestinationPolicy{pattern: compiled, logger: logger}, nil
}

func (p *DestinationPolicy) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	if p.pattern.MatchString(req.DestAddr.FQDN) {
		return ctx, true
	}
	if p.logger != nil {
		p.logger.Printf("Rejected destination %s", cleanLogValue(req.DestAddr.FQDN))
	}
	return ctx, false
}

func cleanLogValue(value string) string {
	if len(value) > maxLogValueLen {
		value = value[:maxLogValueLen] + "..."
	}
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\r", "\\r")
}
