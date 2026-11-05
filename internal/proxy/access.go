package proxy

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	socks5 "github.com/things-go/go-socks5"
	"github.com/things-go/go-socks5/statute"
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
	fqdn := ""
	if req.DestAddr != nil {
		fqdn = req.DestAddr.FQDN
	}
	if p.pattern.MatchString(fqdn) {
		return ctx, true
	}
	if p.logger != nil {
		p.logger.Printf("Rejected destination %s", cleanLogValue(fqdn))
	}
	return ctx, false
}

// commandPolicy gates SOCKS5 commands before delegating to the optional
// destination policy. UDP ASSOCIATE is rejected unless explicitly enabled
// via ENABLE_UDP_ASSOCIATE.
type commandPolicy struct {
	allowUDP bool
	next     socks5.RuleSet
	logger   *log.Logger
}

func (p commandPolicy) Allow(ctx context.Context, req *socks5.Request) (context.Context, bool) {
	if req.Command == statute.CommandAssociate && !p.allowUDP {
		if p.logger != nil {
			p.logger.Println("Rejected UDP ASSOCIATE request (ENABLE_UDP_ASSOCIATE=false)")
		}
		return ctx, false
	}
	if p.next != nil {
		return p.next.Allow(ctx, req)
	}
	return ctx, true
}

func cleanLogValue(value string) string {
	if len(value) > maxLogValueLen {
		value = value[:maxLogValueLen] + "..."
	}
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\r", "\\r")
}
