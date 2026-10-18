# Capability adoption: bring useful engine features into Chimera

Chimera can discover a feature manifest from an engine adapter and assess whether
that feature can be used by the host. The `capability` package provides the
first catalog and assessment layer.

## Adoption paths

- **Native implementation:** implement the behavior in Chimera's own code when
  the design is understood and tested, or reuse source only after its license,
  attribution, provenance, and compatibility have been reviewed.
- **Adapter/delegation:** use the upstream engine's supported API or adapter
  instead of copying its source. This is the default.
- **Unavailable:** the host lacks a required capability, so the feature cannot
  be enabled safely.

## Suggested workflow

1. Discover documented capabilities through an engine adapter or an explicit
   manifest. Do not run untrusted engine binaries or repositories merely to
   inspect them.
2. Record upstream project, version, feature, license, and source URL.
3. Assess API/platform requirements and conflicts with existing engine ownership.
4. Choose native reimplementation, license-approved source reuse, or adapter
   delegation.
5. Add tests that establish behavioral compatibility and failure handling.
6. Keep attribution and license notices with any reused code, and pin the
   upstream revision for reproducibility.

## What this does not do

It does not automatically scrape, copy, or merge arbitrary upstream source code.
A capability is not the same thing as its implementation, and a declared
feature does not prove correctness or security. Some engines have incompatible
configuration models, platform assumptions, or exclusive access to resources
such as VPN/TUN. Chimera must not silently activate conflicting data planes.
The catalog currently provides metadata and assessment only; it does not yet
implement automatic code generation or install features at runtime.
