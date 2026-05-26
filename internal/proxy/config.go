package proxy

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	ListenIP        string   `env:"PROXY_LISTEN_IP" env-default:"0.0.0.0"`
	Port            string   `env:"PROXY_PORT" env-default:"1080"`
	User            string   `env:"PROXY_USER"`
	Password        string   `env:"PROXY_PASSWORD"`
	Accounts        []string `env:"PROXY_ACCOUNTS" env-separator:","`
	RequireAuth     bool     `env:"REQUIRE_AUTH" env-default:"true"`
	EnablePlain     bool     `env:"ENABLE_PLAIN" env-default:"false"`
	AllowedFQDN     string   `env:"ALLOWED_DEST_FQDN"`
	AllowedIPs      []string `env:"ALLOWED_IPS" env-separator:","`
	MaxConnections  int      `env:"MAX_CONNECTIONS" env-default:"100"`
	Timeout         int      `env:"TIMEOUT" env-default:"300"`
	MaxAuthFailures int      `env:"MAX_AUTH_FAILURES" env-default:"5"`
	AuthLockout     int      `env:"AUTH_LOCKOUT" env-default:"300"`
	RateLimit       int      `env:"RATE_LIMIT_PER_SEC" env-default:"10"`
	TLS             TLSConfig
	MetricsAddr     string `env:"METRICS_ADDR"`
}

type TLSConfig struct {
	Enabled      bool   `env:"TLS_ENABLED" env-default:"false"`
	Port         string `env:"TLS_PORT" env-default:"10443"`
	CertFile     string `env:"TLS_CERT_FILE"`
	KeyFile      string `env:"TLS_KEY_FILE"`
	MinVersion   string `env:"TLS_MIN_VERSION" env-default:"1.2"`
	ClientAuth   bool   `env:"TLS_CLIENT_AUTH" env-default:"false"`
	ClientCAFile string `env:"TLS_CLIENT_CA_FILE"`
}

type singleUserCredentials struct {
	User     string `env:"PROXY_USER" env-required:"true"`
	Password string `env:"PROXY_PASSWORD" env-required:"true"`
}

type tlsFiles struct {
	CertFile string `env:"TLS_CERT_FILE" env-required:"true"`
	KeyFile  string `env:"TLS_KEY_FILE" env-required:"true"`
}

type tlsClientCA struct {
	ClientCAFile string `env:"TLS_CLIENT_CA_FILE" env-required:"true"`
}

func LoadConfig() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read environment: %w", err)
	}
	if err := loadRequiredEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadRequiredEnv(cfg *Config) error {
	if cfg.RequireAuth && len(cfg.Accounts) == 0 {
		var credentials singleUserCredentials
		if err := cleanenv.ReadEnv(&credentials); err != nil {
			return fmt.Errorf("read required authentication environment: %w", err)
		}
		cfg.User = credentials.User
		cfg.Password = credentials.Password
	}

	if cfg.TLS.Enabled {
		var files tlsFiles
		if err := cleanenv.ReadEnv(&files); err != nil {
			return fmt.Errorf("read required TLS environment: %w", err)
		}
		cfg.TLS.CertFile = files.CertFile
		cfg.TLS.KeyFile = files.KeyFile
	}

	if cfg.TLS.Enabled && cfg.TLS.ClientAuth {
		var ca tlsClientCA
		if err := cleanenv.ReadEnv(&ca); err != nil {
			return fmt.Errorf("read required TLS client CA environment: %w", err)
		}
		cfg.TLS.ClientCAFile = ca.ClientCAFile
	}

	return nil
}

func (c Config) Validate() error {
	if err := validatePort("PROXY_PORT", c.Port); err != nil {
		return err
	}
	if c.ListenIP != "" && net.ParseIP(c.ListenIP) == nil {
		return fmt.Errorf("PROXY_LISTEN_IP must be a valid IP address: %s", c.ListenIP)
	}
	if c.MaxConnections <= 0 {
		return fmt.Errorf("MAX_CONNECTIONS must be greater than 0")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("TIMEOUT must be greater than 0")
	}
	if c.MaxAuthFailures <= 0 {
		return fmt.Errorf("MAX_AUTH_FAILURES must be greater than 0")
	}
	if c.AuthLockout <= 0 {
		return fmt.Errorf("AUTH_LOCKOUT must be greater than 0")
	}
	if c.RateLimit <= 0 {
		return fmt.Errorf("RATE_LIMIT_PER_SEC must be greater than 0")
	}
	if c.MetricsAddr != "" {
		if _, _, err := net.SplitHostPort(c.MetricsAddr); err != nil {
			return fmt.Errorf("METRICS_ADDR must be a host:port address: %w", err)
		}
	}
	for _, ip := range c.AllowedIPs {
		if ip == "" {
			continue
		}
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("ALLOWED_IPS contains invalid IP address: %s", ip)
		}
	}
	if c.RequireAuth && len(c.Accounts) == 0 && (c.User == "" || c.Password == "") {
		return fmt.Errorf("authentication requires PROXY_ACCOUNTS or both PROXY_USER and PROXY_PASSWORD")
	}
	return c.TLS.validate(c.EnablePlain, c.Port)
}

func (c Config) PlainEnabled() bool {
	return !c.TLS.Enabled || c.EnablePlain
}

func (c Config) PlainAddr() string {
	return net.JoinHostPort(c.ListenIP, c.Port)
}

func (c Config) TLSAddr() string {
	return net.JoinHostPort(c.ListenIP, c.TLS.Port)
}

func (c Config) TimeoutDuration() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

func (c Config) LockoutDuration() time.Duration {
	return time.Duration(c.AuthLockout) * time.Second
}

func (t TLSConfig) validate(enablePlain bool, plainPort string) error {
	if !t.Enabled {
		return nil
	}
	if t.CertFile == "" || t.KeyFile == "" {
		return fmt.Errorf("TLS_ENABLED=true requires TLS_CERT_FILE and TLS_KEY_FILE")
	}
	if err := validatePort("TLS_PORT", t.Port); err != nil {
		return err
	}
	if enablePlain && t.Port == plainPort {
		return fmt.Errorf("PROXY_PORT and TLS_PORT must differ when ENABLE_PLAIN=true")
	}
	if _, err := parseTLSVersion(t.MinVersion); err != nil {
		return err
	}
	if err := requireReadableFile("TLS_CERT_FILE", t.CertFile); err != nil {
		return err
	}
	if err := requireReadableFile("TLS_KEY_FILE", t.KeyFile); err != nil {
		return err
	}
	if t.ClientAuth {
		if t.ClientCAFile == "" {
			return fmt.Errorf("TLS_CLIENT_AUTH=true requires TLS_CLIENT_CA_FILE")
		}
		if err := requireReadableFile("TLS_CLIENT_CA_FILE", t.ClientCAFile); err != nil {
			return err
		}
	}
	return nil
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a valid TCP port between 1 and 65535: %s", name, value)
	}
	return nil
}

func requireReadableFile(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s is not readable: %w", name, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must point to a file: %s", name, path)
	}
	return nil
}
