# Host-app integration API

PG CORE now defines a public, UI-neutral extension contract in
`pkg/integration`. This is the boundary for host applications that embed PG
CORE beside other engines. It does not depend on `internal/quickconnect` or
an engine-specific package, and does not impose a particular UI toolkit.

## Supported entry points

- `integration.EntrySettings`: register the capability under the host app's
  Settings surface.
- `integration.EntryAddConnection`: register it in the host app's `+` /
  Add Connection flow.

A module can request either or both. The host app decides how each action is
rendered and navigated to in its native UI. Registering an action does not
silently switch the active engine or claim that a tunnel is connected.

## Contract sketch

```go
type Module interface {
    Descriptor() integration.Descriptor
    Open(context.Context, integration.EntryPoint, integration.HostContext) error
}

type Registry interface {
    RegisterAction(integration.Action) error
}
```

A host app implements `Registry` as a thin adapter to its navigation system,
then calls `integration.Register(registry, module)` during app initialization.
For a Settings action and an Add Connection action, the module descriptor
declares both entry points. The host can call `integration.Invoke` after the
user chooses an action.

For the standard CDN/SNI entry, the host supplies its native navigation callback:

```go
module := integration.NewFrontingModule(func(
    ctx context.Context,
    entry integration.EntryPoint,
    host integration.HostContext,
) error {
    // Navigate to the app-owned screen; check host.ActiveEngineID against
    // the engine capabilities before enabling connection actions.
    return navigator.OpenFrontingCDN(ctx, entry, host)
})
if err := integration.Register(registry, module); err != nil {
    return err
}
```

The callback above is a host-app adapter point, not a function provided by PG
CORE. It must be implemented by the embedding app. `NewFrontingModule`
declares the stable ID `fronting.cdn-sni` and registers both entry points.

`HostContext` intentionally contains only non-secret app/engine identifiers.
Credentials and private configuration must stay in the host's secure profile
store and must not be passed through UI action metadata.

## Multi-engine rules

- The host owns engine selection and capability negotiation.
- A module must explicitly declare which engine IDs or capabilities it supports
  in its host-specific adapter; this generic registry does not assume that
  native, sing-box, and Xray profiles are interchangeable.
- If the active engine cannot support a module, the host should hide or disable
  that action and explain why. Do not change engines as an implicit side effect
  of opening Settings.
- The module should report unsupported engine/profile states clearly and must
  not claim success before the selected engine's own health check passes.
- Host apps should register stable module IDs and avoid duplicate registration
  during repeated UI initialization.
- UI adapters should not expose secrets in labels, logs, navigation arguments,
  or analytics.

## Scope and limitations

This package provides the **integration seam**, not a native UI implementation
for every host. An Android, Flutter, desktop, or web app still needs a small
adapter that maps `RegisterAction` into its own Settings and Add Connection
navigation. The current Go repository does not contain the source of every
external host app, so their menus cannot be edited from this repository alone.

The existing Quick Connect flow remains available as a standalone loopback web
UI. This integration API does not replace it, does not modify existing engine
behavior, and does not implement an engine-specific transport or network
circumvention mechanism. Existing manual configuration and engine health checks
remain owned by their current packages.
