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

## Multi-language fusion frame ABI

The PGF1 framing codec is implemented by `fusion_frame.c/.h`, with regression tests in `test_fusion_frame.c`. Run both native test programs with:

```sh
cc -std=c11 -Wall -Wextra -Werror -pedantic native/cdn_validation.c native/test_cdn_validation.c -o /tmp/pg-core-cdn-test
/tmp/pg-core-cdn-test
cc -std=c11 -Wall -Wextra -Werror -pedantic native/fusion_frame.c native/test_fusion_frame.c -o /tmp/pg-core-fusion-test
/tmp/pg-core-fusion-test
```

The matching Go and Kotlin codecs are documented in `docs/MULTILANG_FUSION.md`. The framing format is not authenticated and must not be exposed directly to untrusted networks.
