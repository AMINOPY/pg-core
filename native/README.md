# Native C boundary

This directory contains a small, dependency-free C ABI primitive for bounded
SNI hostname syntax validation. It is intentionally not an independent tunnel
engine and does not own connections, secrets, or process lifecycle.

## Build and test

From the repository root, with a C11 compiler:

```sh
cc -std=c11 -Wall -Wextra -Werror -pedantic native/cdn_validation.c native/test_cdn_validation.c -o /tmp/pg-core-c-test
/tmp/pg-core-c-test
```

The function uses an explicit pointer and length; the input need not be
NUL-terminated. It rejects malformed DNS label syntax and IP literals. It
does not perform IDNA conversion, DNS resolution, certificate checks, or prove
that a hostname is controlled by the caller. Keep the existing Go validation
as the authoritative validation path until cross-language parity tests are
wired into CI.
