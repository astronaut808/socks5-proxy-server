# Security Policy

## Supported Versions

Security fixes are applied to the `master` branch and published through the `latest` container tag unless a release branch is explicitly announced.

## Reporting a Vulnerability

Please report security issues privately through GitHub security advisories when available. If advisories are not available, contact the repository owner directly instead of opening a public issue with exploit details.

## Deployment Guidance

Run with authentication enabled, use strong unique passwords or bcrypt hashes in `PROXY_ACCOUNTS`, and prefer TLS or mutual TLS on untrusted networks. Combine application allowlists with host firewall rules when exposing the proxy publicly.
