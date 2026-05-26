# SOCKS5 Proxy Server

![Build and Push Docker image](https://github.com/astronaut808/socks5-proxy-server/actions/workflows/docker.yml/badge.svg)

Small SOCKS5 proxy built on `go-socks5` and Go 1.26.3. It supports password authentication, multiple accounts, source IP allowlists, destination FQDN access policies, connection limits, rate limiting, plain SOCKS5, SOCKS5 over TLS, and optional mutual TLS.

## Quick Start

Run with one username and password:

```bash
docker run -d --name socks5 \
  -p 1080:1080 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Run on a custom container port:

```bash
docker run -d --name socks5 \
  -p 1090:9090 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  -e PROXY_PORT=9090 \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Test through the proxy:

```bash
curl --socks5 <proxy-host>:1080 -U <user>:<password> https://ipinfo.io
```

## Configuration

| ENV variable | Type | Default | Required | Description |
| --- | --- | --- | --- | --- |
| `REQUIRE_AUTH` | Boolean | `true` | no | Require SOCKS5 username/password authentication. Disabling this is not recommended unless another control protects the proxy. |
| `PROXY_USER` | String | empty | conditional | Single proxy username, required with `PROXY_PASSWORD` when `REQUIRE_AUTH=true` and `PROXY_ACCOUNTS` is empty. |
| `PROXY_PASSWORD` | String | empty | conditional | Single proxy password, required with `PROXY_USER` when `REQUIRE_AUTH=true` and `PROXY_ACCOUNTS` is empty. The plaintext value is hashed in memory at startup. |
| `PROXY_ACCOUNTS` | String | empty | conditional | Comma-separated `user:password` or `user:bcrypt-hash` entries. Required when `REQUIRE_AUTH=true` and single-user credentials are not set. |
| `ENABLE_PLAIN` | Boolean | `false` | no | Keep the plain SOCKS5 listener enabled when TLS is enabled. |
| `PROXY_PORT` | String | `1080` | no | Plain SOCKS5 listen port. Must be 1-65535. |
| `PROXY_LISTEN_IP` | String | `0.0.0.0` | no | Listen address. Use `127.0.0.1` for local-only access. |
| `ALLOWED_DEST_FQDN` | String | empty | no | Destination FQDN regular expression. Empty allows all destinations. Prefer anchored regexes such as `^api\.example\.com$`. |
| `ALLOWED_IPS` | String | empty | no | Comma-separated source IP allowlist. |
| `MAX_CONNECTIONS` | Int | `100` | no | Maximum concurrent client connections. Must be greater than zero. |
| `TIMEOUT` | Int | `300` | no | Read/write timeout in seconds. Must be greater than zero. |
| `MAX_AUTH_FAILURES` | Int | `5` | no | Failed authentication attempts before temporary lockout. Must be greater than zero. |
| `AUTH_LOCKOUT` | Int | `300` | no | Lockout duration in seconds. Must be greater than zero. |
| `RATE_LIMIT_PER_SEC` | Int | `10` | no | New connection rate limit. Burst limit is 2x this value. Must be greater than zero. |
| `TLS_ENABLED` | Boolean | `false` | no | Enable SOCKS5 over TLS. |
| `TLS_PORT` | String | `10443` | no | TLS listener port. Must be 1-65535. |
| `TLS_CERT_FILE` | String | empty | conditional | TLS certificate file path in PEM format. Required when `TLS_ENABLED=true`. |
| `TLS_KEY_FILE` | String | empty | conditional | TLS private key file path in PEM format. Required when `TLS_ENABLED=true`. |
| `TLS_CLIENT_AUTH` | Boolean | `false` | no | Require client certificate authentication. |
| `TLS_CLIENT_CA_FILE` | String | empty | conditional | PEM file with CA certificates used to validate client certificates. Required when `TLS_CLIENT_AUTH=true`. |
| `TLS_MIN_VERSION` | String | `1.2` | no | Minimum TLS version. Supported values: `1.2`, `1.3`. |
| `METRICS_ADDR` | String | empty | no | Optional `host:port` address for the standard Go `expvar` endpoint at `/debug/vars`. Empty disables metrics. |

## Examples

Restrict destinations to specific domains:

```bash
ALLOWED_DEST_FQDN='^.*\.(example\.com|internal\.local)$'
```

Allow only specific client IPs:

```bash
ALLOWED_IPS=192.168.1.10,10.0.0.5
```

Enable TLS:

```bash
TLS_ENABLED=true
TLS_PORT=10443
TLS_CERT_FILE=/etc/ssl/certs/proxy.crt
TLS_KEY_FILE=/etc/ssl/private/proxy.key
TLS_MIN_VERSION=1.3
```

Run with TLS certificates mounted:

```bash
docker run -d --name socks5-tls \
  -p 10443:10443 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  -e TLS_ENABLED=true \
  -e TLS_CERT_FILE=/certs/cert.pem \
  -e TLS_KEY_FILE=/certs/key.pem \
  -v /path/to/certs:/certs:ro \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

With SOCKS5 over TLS, regular SOCKS5 traffic is sent inside a protected TLS connection. If your SOCKS5 client doesn't support TLS directly, you can use a local TLS wrapper such as `socat`:

```bash
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,verify=0
curl --socks5 localhost:1080 -U <user>:<password> https://ipinfo.io
```

## Development

Project layout:

```text
cmd/socks5-proxy-server/    application entrypoint
internal/proxy/       config, auth, access policy, listeners, TLS, metrics, server orchestration
.github/workflows/    CI and image publishing
```

Run local checks:

```bash
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Build a local binary:

```bash
go build ./cmd/socks5-proxy-server
```

Enable local metrics while developing:

```bash
METRICS_ADDR=127.0.0.1:9090 go run ./cmd/socks5-proxy-server
curl http://127.0.0.1:9090/debug/vars
```

Build and run with Docker Compose:

```bash
cp .env.example .env
docker compose -f docker-compose.build.yml up -d --build
```

## Production Notes

Keep authentication enabled, prefer TLS or mTLS for untrusted networks, bind to `127.0.0.1` when exposing the proxy only through a tunnel, and use firewall rules in addition to `ALLOWED_IPS` for public hosts. Avoid broad destination regexes unless intentionally running an open egress proxy.
