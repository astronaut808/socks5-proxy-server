package proxy

import (
	"context"
	"io"
	"log"
	"testing"

	"github.com/armon/go-socks5"
)

func TestDestinationPolicy(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		fqdn    string
		want    bool
	}{
		{name: "exact match", pattern: "^example\\.com$", fqdn: "example.com", want: true},
		{name: "subdomain rejected", pattern: "^example\\.com$", fqdn: "api.example.com", want: false},
		{name: "wildcard subdomain", pattern: "^.*\\.example\\.com$", fqdn: "api.example.com", want: true},
		{name: "alternatives", pattern: "^(google\\.com|facebook\\.com)$", fqdn: "google.com", want: true},
		{name: "alternative rejected", pattern: "^(google\\.com|facebook\\.com)$", fqdn: "twitter.com", want: false},
		{name: "empty fqdn rejected", pattern: "^example\\.com$", fqdn: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := mustDestinationPolicy(t, tt.pattern)
			req := &socks5.Request{DestAddr: &socks5.AddrSpec{FQDN: tt.fqdn}}

			_, got := policy.Allow(context.Background(), req)
			if got != tt.want {
				t.Fatalf("Allow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDestinationPolicyRejectsInvalidPattern(t *testing.T) {
	if _, err := NewDestinationPolicy("[", log.New(io.Discard, "", 0)); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestCleanLogValue(t *testing.T) {
	got := cleanLogValue("hello\nworld\r")
	if got != "hello\\nworld\\r" {
		t.Fatalf("cleanLogValue() = %q", got)
	}

	long := cleanLogValue(string(make([]byte, maxLogValueLen+1)))
	if len(long) != maxLogValueLen+3 {
		t.Fatalf("truncated length = %d, want %d", len(long), maxLogValueLen+3)
	}
}

func mustDestinationPolicy(t *testing.T, pattern string) *DestinationPolicy {
	t.Helper()

	policy, err := NewDestinationPolicy(pattern, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new destination policy: %v", err)
	}
	return policy
}
