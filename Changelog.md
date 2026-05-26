# Changelog

All notable changes to this project will be documented in this file.

## [v1.0.0] - 2026-05-26

### Added
- Initial release.
- SOCKS5 proxy with username/password authentication enabled by default.
- Multiple account support through `PROXY_ACCOUNTS`.
- Bcrypt password hash support.
- Authentication failure lockout.
- Source IP allowlisting through `ALLOWED_IPS`.
- Destination FQDN access policy through `ALLOWED_DEST_FQDN`.
- Concurrent connection limits.
- New connection rate limiting.
- Read/write connection deadlines.
- SOCKS5 over TLS.
- Optional mutual TLS client authentication.
- Optional expvar metrics endpoint through `METRICS_ADDR`.
- Environment-based configuration through `cleanenv`.
- Strict startup validation for auth, ports, limits, IPs, TLS files, TLS versions, and metrics address.
- Distroless Docker image.
- CI checks for tests, race detector, `go vet`, `golangci-lint`, Dockerfile linting, and image publishing.
- Dependabot updates for Go modules, GitHub Actions, and Docker.

### Security
- Authentication is required by default.
- TLS 1.0 and TLS 1.1 are not supported.
- Passwords are hashed in memory before comparison.
- Unknown users do not grow authentication failure state.
- SOCKS5 credentials and destination metadata can be protected on the client-to-proxy hop with TLS.
