# Security Policy

## Supported Versions

Security fixes are applied to the `master` branch and published through the `latest` container tag unless a release branch is explicitly announced.

## Reporting a Vulnerability

Please report security issues privately through GitHub security advisories when available. If advisories are not available, contact the repository owner directly instead of opening a public issue with exploit details.

## Deployment Guidance

Run with authentication enabled, use strong unique passwords or bcrypt hashes in `PROXY_ACCOUNTS`, and prefer TLS or mutual TLS on untrusted networks. Combine application allowlists with host firewall rules when exposing the proxy publicly.

## Known Limitations

### `ALLOWED_DEST_FQDN` does not prevent DNS lookups

`ALLOWED_DEST_FQDN` filters the destination requested in the SOCKS5 request, but due to a limitation of the underlying `things-go/go-socks5` library, the server resolves the requested FQDN via DNS *before* the rule set is evaluated. This means a client can trigger a DNS query for a domain that the policy will ultimately reject, and that query is visible to the server's DNS resolver and the network path to it (a DNS leak relative to the policy decision).

Mitigation: where this matters, run the server with a trusted DNS resolver and restrict its DNS egress at the network level (for example, allow outbound UDP/TCP port 53 only to an internal or otherwise trusted resolver, or use a resolver with its own filtering). Treat `ALLOWED_DEST_FQDN` as a connection-level filter, not as a way to hide which names clients ask the proxy to resolve.
