# Safe PG CORE update model

PG CORE updates are delivered as immutable, versioned releases. A change to the core repository must never rewrite source files, dependencies, engine registrations, or configuration in an unrelated consumer project at runtime.

## Contract

- Consumers pin a released version (Go module tag or a release artifact); do not track a moving branch in production.
- A new core release is the update signal. Consumer repositories may use Dependabot/Renovate or a repository-specific workflow to open an update pull request.
- Update automation must create a branch/PR, not push directly to a consumer's default branch.
- The consumer project's CI must pass before merge. A failed or incompatible update leaves the existing version intact.
- Each consumer chooses its own rollout time. Updating one project must not mutate any other project or the PG CORE source tree.
- Runtime self-modification of source code is explicitly unsupported. Android applications cannot safely replace their own installed native code without a signed app update; use the platform package/update channel instead.
- Do not auto-restart a live VPN tunnel to install an unverified engine. Validate signatures/provenance, checksums, supported ABI and version compatibility before a staged update; retain the previous known-good version for rollback.

## Enabling dependency update PRs in a consumer

In each consuming repository, add a Dependabot or Renovate rule for the PG CORE Go module and configure it to open PRs. Example Dependabot configuration (adjust the directory if the module is not at the repository root):

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
```

This is intentionally configured in each consumer repository: a central core repository cannot safely change another repository unless that repository explicitly installs an updater and grants narrowly scoped permissions.

## Current scope

This policy documents the safe update boundary. It does not claim that arbitrary external projects are already registered, automatically updated, or have been tested against every release. The release and consumer CI pipeline must be enabled and verified before describing updates as automatic.
