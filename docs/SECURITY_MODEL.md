# Core Security Model

## Goals

Chimera Core uses defense in depth. No encoding, signature, or single check makes a device "unhackable". Security controls should run by default and fail closed where continuing could expose traffic or secrets.

## Automatic controls currently present

- Strict JSON configuration parsing rejects unknown fields, unsupported schema versions, duplicate engines, and multiple JSON values.
- Configuration input is capped at 1 MiB before decoding to bound memory use from malformed or hostile files.
- Engine adapters receive cancellation-aware contexts and must not log credentials or raw secret-bearing configuration.
- Integrity helpers use HMAC-SHA-256 with constant-time verification. The host must provide keys from an OS-backed secret store; keys must not be hard-coded, committed, or logged.
- Malware scanning is bounded to explicitly authorized directories, skips symlinks and oversized files, and can quarantine exact signature matches rather than permanently deleting them.
- CI runs formatting, `go vet`, Go unit tests, and Go race tests, plus the Python, Java, and Kotlin client tests.

## Binary and Morse

Binary (`0101...`) and Morse are encodings for diagnostics only. They do not encrypt, authenticate, hide, or protect data. Never encode passwords, tokens, private keys, or personal data into logs using these formats. HMAC is for integrity/authentication when a secret key is safely managed; it is not encryption.

## Platform boundary

A normal Android app cannot inspect other apps' private sandboxes, read arbitrary system memory, or guarantee removal of every malicious process. CPU utilization is not a malware signature. Device-wide protection requires OS-supported permissions and platform integration, and some checks require user/administrator consent. The core must not claim full-device antivirus until a signed signature feed, Android WorkManager integration, safe quarantine/recovery, and real-device tests exist.

## Required release hardening

- Store keys in Android Keystore or an equivalent OS-backed secret store; never generate a universal embedded key.
- Verify update/signature-feed authenticity and freshness before applying changes.
- Keep network TLS certificate validation enabled; do not bypass trust checks.
- Limit retries, input sizes, scan duration, and resource use.
- Redact secrets from all logs and diagnostics.
- Run dependency vulnerability scanning and test release artifacts on supported OS versions.
- Test actual engine startup, routes, DNS, cancellation, recovery, and shutdown on devices before describing the app as production-ready.

## Fail-closed behavior

If a required integrity check, signature verification, or security policy fails, do not silently continue with an untrusted configuration. Return a stable error code, emit a redacted diagnostic, and let the platform host decide whether recovery or user consent is required.
