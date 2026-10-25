# Persistent manifest rollback protection

`transportprofile.FetchManifest` accepts a minimum sequence from its caller. That protects a single invocation only unless the caller persists the last accepted value. `FileSequenceStore` provides a local store and `FetchManifestWithStore` wires the read/verify/accept sequence together.

```go
store, err := transportprofile.NewFileSequenceStore("/path/owned-by-the-app/manifest-sequences.json")
if err != nil {
    return err
}
manifest, err := transportprofile.FetchManifestWithStore(ctx, "https://tunnel.example.com", trustedKeys, time.Now(), store)
if err != nil {
    return err
}
_ = manifest // use profiles only after this call succeeds
```

## Security and operational notes

- The store is keyed by canonical discovery hostname and persists only sequence numbers, not credentials.
- A sequence is advanced only after signature, hostname, validity window, and profile validation pass.
- Lower sequences are rejected; equal sequences are allowed for re-fetching the same signed manifest.
- Writes use a same-directory temporary file, restrictive file mode where supported, file sync, and atomic rename. The parent directory must be private to the application.
- Operations are serialized through one store instance. There is no cross-process lock, platform secure-storage integration, protection against a local attacker replacing the store, or key-rotation management. Apps should use private application data storage and consider OS-provided secure storage for higher assurance.
- A manifest sequence is not a substitute for expiry checks, trusted signing keys, or a valid transport implementation. This feature does not itself create a tunnel.
