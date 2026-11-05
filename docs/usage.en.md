# Usage and Operations Examples

[Русский](usage.md)

## Restricting Destinations

Allow only specific domains (and their subdomains):

```bash
ALLOWED_DEST_FQDN='^(.*\.)?(example\.com|internal\.local)$'
```

Note: `ALLOWED_DEST_FQDN` filters the destination from the SOCKS5 request, but the server may still send a DNS query for the requested FQDN *before* the policy is evaluated (a limitation of the `things-go/go-socks5` library, which resolves names before applying rules). The policy therefore does not hide rejected names from the server's DNS resolver. Where this matters, use a trusted resolver and restrict the server's DNS egress at the network level (see [SECURITY.md](../SECURITY.md)).

Allow only specific source IPs:

```bash
ALLOWED_IPS=192.168.1.10,10.0.0.5
```

## TLS

Enable SOCKS5 over TLS:

```bash
TLS_ENABLED=true
TLS_PORT=10443
TLS_CERT_FILE=/etc/ssl/certs/proxy.crt
TLS_KEY_FILE=/etc/ssl/private/proxy.key
TLS_MIN_VERSION=1.3
```

Run the container with mounted certificates:

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

## TLS Wrapper via socat

With SOCKS5 over TLS, regular SOCKS5 traffic is sent inside a protected TLS connection. If your SOCKS5 client doesn't support TLS directly, you can run a local TLS wrapper with `socat`.

The correct approach is to verify the server certificate against a CA:

```bash
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,cafile=/path/to/ca.pem
curl --socks5 localhost:1080 -U <user>:<password> https://ipinfo.io
```

The `verify=0` variant disables certificate verification and is suitable only for local testing, not for real use:

```bash
# Local testing only — certificate verification is disabled.
socat TCP-LISTEN:1080,reuseaddr,fork OPENSSL:<server-ip>:10443,verify=0
```

## Metrics

`METRICS_ADDR` enables the standard Go `expvar` endpoint at `/debug/vars`. Bind it to loopback (`127.0.0.1:9090`) or protect it at the network layer (firewall, private interface), since the endpoint requires no authentication.

```bash
METRICS_ADDR=127.0.0.1:9090
curl http://127.0.0.1:9090/debug/vars
```

## Production

Keep authentication enabled, prefer TLS or mTLS for untrusted networks, bind to `127.0.0.1` when exposing the proxy only through a tunnel, and use firewall rules in addition to `ALLOWED_IPS` for public hosts. Avoid broad destination regexes unless intentionally running an open egress proxy. See [SECURITY.md](../SECURITY.md).
