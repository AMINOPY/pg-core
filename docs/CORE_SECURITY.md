# Core security and diagnostic encodings

## Real integrity protection

The `internal/security` package provides HMAC-SHA-256 tags for authenticating
small byte sequences and constant-time tag verification. Callers must supply
a high-entropy secret from an OS/platform secret store. The core must not
hard-code the key, place it in source control, or print it to logs. Key
rotation, secure storage, and ownership are responsibilities of the host app
until a platform key-provider integration is implemented.

An HMAC detects unauthorized changes only when the secret key remains secret.
It does not encrypt data, prove that a binary is malware-free, or protect a
compromised process that can access the key.

## Binary and Morse

`Binary` displays bytes as eight `0`/`1` digits. `Morse` converts ASCII
letters and digits to dots and dashes. These formats can be used for compact
diagnostic representations or human-readable incident codes.

**Neither binary nor Morse is a security mechanism.** They are reversible
encodings and provide no secrecy, authenticity, or tamper resistance. Never
use them instead of TLS, authenticated encryption, HMAC, secure storage, or
platform permissions. Do not encode credentials or private keys for display.

## Scope and remaining work

This package is a tested security primitive, not a complete core security
audit. Before production, integrate authenticated audit events at the relevant
call sites, obtain and rotate keys through Android Keystore or another
appropriate OS secret store, review dependency and build provenance, fuzz
untrusted parsers, and test Android lifecycle/network behavior on physical
devices. The Go control-plane tests do not validate a real VPN data plane.
