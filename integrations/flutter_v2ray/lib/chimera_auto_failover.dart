import 'chimera_core_bridge.dart';
import 'chimera_flutter_v2ray.dart';

/// One serialized failover operation joining Go policy and the plugin session.
///
/// [verifyConnectivity] must be an app-owned, bounded end-to-end check through
/// the newly connected session; a plugin "connected" status alone is insufficient.
class ChimeraAutoFailover {
  ChimeraAutoFailover({
    required this.core,
    required this.engine,
    required this.verifyConnectivity,
  });

  final ChimeraCoreBridge core;
  final ChimeraFlutterV2Ray engine;
  final Future<bool> Function(ChimeraV2RayCandidate candidate)
      verifyConnectivity;

  Future<bool> _running = Future<bool>.value(false);
  bool _busy = false;

  /// Samples the explicitly supplied candidates and executes at most one switch.
  ///
  /// Returns true only when a new candidate was switched to and verified.
  /// Concurrent calls are rejected to prevent overlapping stop/start operations.
  Future<bool> runOnce(List<ChimeraV2RayCandidate> candidates) async {
    if (_busy) {
      throw StateError('A Chimera failover operation is already running.');
    }
    _busy = true;
    try {
      final current = engine.activeCandidate;
      if (current == null) {
        throw StateError('Connect a candidate before running auto-failover.');
      }
      if (candidates.isEmpty || candidates.length > 64) {
        throw ArgumentError('Provide between 1 and 64 explicit candidates.');
      }
      if (!candidates.any((candidate) => candidate.id == current.id)) {
        throw ArgumentError('Candidate set must include the active candidate.');
      }

      final probes = await engine.probeCandidates(candidates);
      final samples = probes
          .map(
            (probe) => ChimeraHealthCandidate(
              id: probe.candidate.id,
              subscriptionId: probe.candidate.subscriptionId,
              status: probe.isAvailable ? 'healthy' : 'failed',
              latencyMs: probe.isAvailable ? probe.delayMs : -1,
            ),
          )
          .toList(growable: false);

      final decision = await core.evaluate(
        currentId: current.id,
        candidates: samples,
      );
      if (!decision.shouldSwitch || decision.targetId == null) return false;

      final target = candidates.firstWhere(
        (candidate) => candidate.id == decision.targetId,
        orElse: () => throw StateError('Policy returned an unknown target ID.'),
      );
      try {
        await engine.switchTo(target);
        if (!await verifyConnectivity(target)) {
          throw StateError('Replacement session failed connectivity verification.');
        }
        await core.confirmSwitch(target.id);
        return true;
      } catch (_) {
        // Best-effort restoration. Never confirm a candidate that failed verification.
        try {
          if (engine.activeCandidate?.id != current.id) {
            await engine.switchTo(current);
          }
        } finally {
          await core.cancelSwitch();
        }
        rethrow;
      }
    } finally {
      _busy = false;
    }
  }
}
