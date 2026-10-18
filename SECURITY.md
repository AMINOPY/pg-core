# Security policy

## Current status

This repository is an early scaffold and is not suitable for production or protecting sensitive traffic. It does not yet implement a tunnel or integrate a network engine.

## Reporting

Use GitHub's private vulnerability reporting feature when available. Do not publish secrets, private keys, access tokens, or sensitive connection profiles in issues.

## Engineering requirements

- Keep TLS certificate validation enabled.
- Do not log credentials, private keys, tokens, or raw configuration objects.
- Parse configuration strictly and reject unknown fields until schema migration exists.
- Bound network checks, retries, and subprocess execution.
- Avoid direct-network fallback when a profile requires a tunnel.
- Pin and review dependencies; document upstream licenses and advisories.
- Test cleanup after startup failures and process termination.

## Limitations

No claim is made that this project defeats DPI, provides anonymity, or guarantees connectivity. Security and compatibility claims require repeatable tests and independent review.
