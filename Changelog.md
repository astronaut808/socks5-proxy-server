# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Opt-in `ENABLE_UDP_ASSOCIATE` setting (default `false`). With the new SOCKS5 library, enabling it provides full UDP ASSOCIATE relay support; existing configurations without the variable keep working with UDP disabled.

### Changed
- Update the Go requirement and Docker build toolchain from 1.26.3 to 1.27.3.
- Migrated the SOCKS5 dependency from `github.com/armon/go-socks5` with a `serjs` fork `replace` directive to `github.com/things-go/go-socks5` v0.1.4, used directly.
- Moved `AllowedIPs` filtering to the listener layer.
- Update module dependencies.

### Removed
- The stale `replace github.com/armon/go-socks5 => github.com/serjs/go-socks5` workaround in `go.mod`.
- Stale upstream funding links from `.github/FUNDING.yml`.

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
