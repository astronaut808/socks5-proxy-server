package proxy

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("PROXY_USER", "user")
	t.Setenv("PROXY_PASSWORD", "password")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Port != "1080" {
		t.Fatalf("port = %q, want 1080", cfg.Port)
	}
	if cfg.ListenIP != "0.0.0.0" {
		t.Fatalf("listen IP = %q, want 0.0.0.0", cfg.ListenIP)
	}
	if !cfg.RequireAuth {
		t.Fatal("auth should be required by default")
	}
	if cfg.TLS.MinVersion != "1.2" {
		t.Fatalf("TLS min version = %q, want 1.2", cfg.TLS.MinVersion)
	}
}

func TestLoadConfigLists(t *testing.T) {
	t.Setenv("PROXY_ACCOUNTS", "alice:secret,bob:supersecret")
	t.Setenv("ALLOWED_IPS", "192.168.1.1,10.0.0.1")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := len(cfg.Accounts); got != 2 {
		t.Fatalf("accounts length = %d, want 2", got)
	}
	if got := len(cfg.AllowedIPs); got != 2 {
		t.Fatalf("allowed IPs length = %d, want 2", got)
	}
}

func TestLoadConfigRequiresCredentialsWhenAuthEnabled(t *testing.T) {
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing credentials error")
	}
}

func TestLoadConfigRequiresTLSFilesWhenTLSEnabled(t *testing.T) {
	t.Setenv("PROXY_USER", "user")
	t.Setenv("PROXY_PASSWORD", "password")
	t.Setenv("TLS_ENABLED", "true")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing TLS files error")
	}
}

func TestLoadConfigRequiresClientCAWhenClientAuthEnabled(t *testing.T) {
	cert := writeTempFile(t, "cert.pem", "cert")
	key := writeTempFile(t, "key.pem", "key")

	t.Setenv("PROXY_USER", "user")
	t.Setenv("PROXY_PASSWORD", "password")
	t.Setenv("TLS_ENABLED", "true")
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)
	t.Setenv("TLS_CLIENT_AUTH", "true")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected missing TLS client CA error")
	}
}

func TestValidateConfigRejectsInvalidValues(t *testing.T) {
	base := validConfig()

	tests := map[string]func(*Config){
		"auth without credentials": func(cfg *Config) {
			cfg.User = ""
			cfg.Password = ""
			cfg.Accounts = nil
		},
		"bad listen IP":        func(cfg *Config) { cfg.ListenIP = "nope" },
		"bad plain port":       func(cfg *Config) { cfg.Port = "70000" },
		"zero connections":     func(cfg *Config) { cfg.MaxConnections = 0 },
		"zero timeout":         func(cfg *Config) { cfg.Timeout = 0 },
		"zero auth failures":   func(cfg *Config) { cfg.MaxAuthFailures = 0 },
		"zero lockout":         func(cfg *Config) { cfg.AuthLockout = 0 },
		"zero rate limit":      func(cfg *Config) { cfg.RateLimit = 0 },
		"bad allowed IP":       func(cfg *Config) { cfg.AllowedIPs = []string{"127.0.0.1", "bad"} },
		"bad metrics address":  func(cfg *Config) { cfg.MetricsAddr = "127.0.0.1" },
		"legacy TLS version":   func(cfg *Config) { cfg.TLS.Enabled = true; cfg.TLS.MinVersion = "1.1" },
		"same plain TLS ports": func(cfg *Config) { cfg.TLS.Enabled = true; cfg.EnablePlain = true; cfg.TLS.Port = cfg.Port },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateTLSFiles(t *testing.T) {
	cert := writeTempFile(t, "cert.pem", "cert")
	key := writeTempFile(t, "key.pem", "key")

	cfg := validConfig()
	cfg.TLS.Enabled = true
	cfg.TLS.CertFile = cert
	cfg.TLS.KeyFile = key

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}
}

func validConfig() Config {
	return Config{
		ListenIP:        "127.0.0.1",
		Port:            "1080",
		User:            "user",
		Password:        "password",
		RequireAuth:     true,
		MaxConnections:  100,
		Timeout:         300,
		MaxAuthFailures: 5,
		AuthLockout:     300,
		RateLimit:       10,
		TLS: TLSConfig{
			Port:       "10443",
			MinVersion: "1.2",
		},
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	path := t.TempDir() + string(os.PathSeparator) + name
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
