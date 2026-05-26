package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func buildTLSConfig(cfg TLSConfig) (*tls.Config, error) {
	minVersion, err := parseTLSVersion(cfg.MinVersion)
	if err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS key pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:           []tls.Certificate{cert},
		MinVersion:             minVersion,
		ClientAuth:             tls.NoClientCert,
		SessionTicketsDisabled: true,
		NextProtos:             []string{"h2", "http/1.1"},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	if cfg.ClientAuth {
		pool, err := loadClientCAPool(cfg.ClientCAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func parseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("TLS_MIN_VERSION must be 1.2 or 1.3")
	}
}

func loadClientCAPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read TLS client CA file: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("parse TLS client CA file: %s", path)
	}
	return pool, nil
}
