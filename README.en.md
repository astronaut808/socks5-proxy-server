[Русский](README.md) | **English**

# SOCKS5 Proxy Server

![Build and Push Docker image](https://github.com/astronaut808/socks5-proxy-server/actions/workflows/docker.yml/badge.svg)

Small SOCKS5 proxy built on `go-socks5` and modern Go (see `go.mod` for the required version). It supports password authentication, multiple accounts, source IP allowlists, destination FQDN access policies, connection limits, rate limiting, plain SOCKS5, SOCKS5 over TLS, and optional mutual TLS.

## Quick Start

```bash
docker run -d --name socks5 \
  -p 1080:1080 \
  -e PROXY_USER=<user> \
  -e PROXY_PASSWORD=<password> \
  ghcr.io/astronaut808/socks5-proxy-server:latest
```

Test through the proxy:

```bash
curl --socks5 <proxy-host>:1080 -U <user>:<password> https://ipinfo.io
```

Docker Compose is also supported: copy `.env.example` to `.env` and run `docker compose up -d` (see `docker-compose.yml` and `docker-compose.build.yml`).

## Configuration

| ENV variable | Type | Default | Required | Description |
| --- | --- | --- | --- | --- |
| `REQUIRE_AUTH` | Boolean | `true` | no | Require SOCKS5 username/password authentication. Disabling this is not recommended unless another control protects the proxy. |
| `PROXY_USER` | String | empty | conditional | Single proxy username, required with `PROXY_PASSWORD` when `REQUIRE_AUTH=true` and `PROXY_ACCOUNTS` is empty. |
| `PROXY_PASSWORD` | String | empty | conditional | Single proxy password, required with `PROXY_USER` when `REQUIRE_AUTH=true` and `PROXY_ACCOUNTS` is empty. The plaintext value is hashed in memory at startup. |
| `PROXY_ACCOUNTS` | String | empty | conditional | Comma-separated `user:password` or `user:bcrypt-hash` entries. Required when `REQUIRE_AUTH=true` and single-user credentials are not set. |
| `ENABLE_PLAIN` | Boolean | `false` | no | The plain SOCKS5 listener is enabled by default. When TLS is enabled, the plain listener is disabled; set `true` only if you intentionally want both listeners (plain and TLS). |
| `PROXY_PORT` | String | `1080` | no | Plain SOCKS5 listen port. Must be 1-65535. |
| `PROXY_LISTEN_IP` | String | `0.0.0.0` | no | Listen address. Use `127.0.0.1` for local-only access. |
| `ALLOWED_DEST_FQDN` | String | empty | no | Destination FQDN regular expression. Empty allows all destinations. Prefer anchored regexes such as `^(.*\.)?(example\.com\|internal\.local)$`. The pattern applies to FQDNs only: destinations given as IP literals are rejected when a pattern is set. |
| `ALLOWED_IPS` | String | empty | no | Comma-separated source IP allowlist. |
| `ENABLE_UDP_ASSOCIATE` | Boolean | `false` | no | Allow the SOCKS5 UDP ASSOCIATE command. Rejected at the policy layer by default: most deployments only need TCP CONNECT, and UDP increases the attack surface. Enable only if clients actually need UDP. |
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
| `METRICS_ADDR` | String | empty | no | Optional `host:port` address for the standard Go `expvar` endpoint at `/debug/vars`. Empty disables metrics. Prefer binding to loopback (`127.0.0.1:9090`) or protecting it at the network layer. |

## Documentation

- [Usage and operations examples](docs/usage.en.md) — TLS, socat wrapper, metrics, production notes.
- [Development](docs/development.md) — project layout, tests, build. (In Russian.)
- [Security](SECURITY.md) — policy and vulnerability reporting.
