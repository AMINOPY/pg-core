# Android skills integration for PG Core

PG Core can use Google's [android/skills](https://github.com/android/skills) repository as **development-agent guidance**. These are modular `SKILL.md` instructions and supporting references/scripts, not runtime libraries: installing them does not add VPN, TUN, routing, DNS, or Android UI behavior to the compiled core.

## Install or refresh every upstream skill

From the repository root, run:

```bash
bash scripts/install-android-skills.sh
```

The script clones the current upstream default branch and copies every directory containing a `SKILL.md`—including that skill's local references and resources—into `.agents/skills/android-upstream/`. It also copies the upstream Apache-2.0 license. Re-running it refreshes only this managed destination; it does not modify the Go/Kotlin/C/Python implementation or delete other agent skills.

Review upstream changes before relying on them in production. The upstream repository evolves independently, so the installed content is a snapshot of the branch at installation time. For reproducible team builds, pin a reviewed upstream commit before making the sync part of a release workflow.

## Skills most relevant to this repository

Use whichever installed skills match the task, particularly:

- Android build-system and Android CLI workflows when changing build configuration.
- Android security guidance when reviewing VPN permission boundaries, exported components, intent handling, secrets, and IPC.
- Android performance/profiling workflows when measuring startup, memory, CPU, or battery impact.
- Android testing setup for unit, instrumentation, and device-level test strategy.
- Navigation/UI skills only when an actual host application UI is in scope.

Other upstream skills are installed too, as requested, but should only be applied when relevant. Camera, TV, automotive, XR, billing, or Play-specific guidance is not evidence that PG Core implements those product features.

## PG Core-specific acceptance rules

For Android engine work:

1. Preserve existing APIs and behavior unless the change explicitly requires otherwise.
2. Keep VPN ownership in one clearly defined host `VpnService`; do not let multiple adapters independently own the system TUN, routes, or DNS.
3. Treat engine-process liveness as weaker than data-plane connectivity. Test a real, permitted end-to-end connection.
4. On stop, startup failure, permission revocation, and network change, verify cleanup and restoration of resources owned by the host.
5. Keep secrets out of logs and generated reports; bound network probes, response bodies, and timeouts.
6. Run Go tests and the available Kotlin checks, then add emulator/device tests for behavior that cannot be proven by host-side unit tests.
7. Do not claim production readiness until Android VPN lifecycle, IPv4/IPv6, DNS, network switching, reconnect, and cleanup have been verified on supported devices.

See [Android's official skill documentation](https://developer.android.com/tools/agents/android-skills) and the upstream [Apache-2.0 license](https://github.com/android/skills/blob/main/LICENSE.txt).
