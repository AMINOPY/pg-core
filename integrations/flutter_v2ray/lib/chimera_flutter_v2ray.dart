import 'package:flutter_v2ray/flutter_v2ray.dart';

/// A profile explicitly imported or supplied by the application.
class ChimeraV2RayCandidate {
  const ChimeraV2RayCandidate({
    required this.id,
    required this.subscriptionId,
    required this.shareLink,
  });

  final String id;
  final String subscriptionId;
  final String shareLink;
}

class ChimeraV2RayProbeResult {
  const ChimeraV2RayProbeResult({
    required this.candidate,
    required this.delayMs,
    this.error,
  });

  final ChimeraV2RayCandidate candidate;

  /// Negative values (including -1) mean the probe failed or is unavailable.
  final int delayMs;
  final Object? error;

  bool get isAvailable => delayMs >= 0;
}

/// App-side adapter around blueboy-tm/flutter_v2ray.
///
/// This class owns the plugin's single V2Ray/VPN session. Do not start another
/// TUN owner alongside it for the same traffic path. This is not Go FFI.
class ChimeraFlutterV2Ray {
  ChimeraFlutterV2Ray({required void Function(V2RayStatus) onStatusChanged})
      : _plugin = FlutterV2ray(onStatusChanged: onStatusChanged);

  final FlutterV2ray _plugin;
  bool _initialized = false;
  ChimeraV2RayCandidate? _active;

  ChimeraV2RayCandidate? get activeCandidate => _active;

  Future<void> initialize() async {
    if (_initialized) return;
    await _plugin.initializeV2Ray();
    _initialized = true;
  }

  Future<ChimeraV2RayProbeResult> probe(
    ChimeraV2RayCandidate candidate, {
    String testUrl = 'https://www.google.com/generate_204',
  }) async {
    try {
      final parsed = FlutterV2ray.parseFromURL(candidate.shareLink);
      final delay = await _plugin.getServerDelay(
        config: parsed.getFullConfiguration(),
        url: testUrl,
      );
      return ChimeraV2RayProbeResult(
        candidate: candidate,
        delayMs: delay < 0 ? -1 : delay,
      );
    } catch (error) {
      return ChimeraV2RayProbeResult(
        candidate: candidate,
        delayMs: -1,
        error: error,
      );
    }
  }

  /// Sequentially probes a bounded set of explicitly supplied candidates.
  Future<List<ChimeraV2RayProbeResult>> probeCandidates(
    List<ChimeraV2RayCandidate> candidates,
  ) async {
    if (candidates.isEmpty || candidates.length > 64) {
      throw ArgumentError('Provide between 1 and 64 explicit candidates.');
    }
    final ids = <String>{};
    for (final candidate in candidates) {
      if (candidate.id.trim().isEmpty || !ids.add(candidate.id)) {
        throw ArgumentError('Candidate IDs must be non-empty and unique.');
      }
    }
    final results = <ChimeraV2RayProbeResult>[];
    for (final candidate in candidates) {
      results.add(await probe(candidate));
    }
    results.sort((a, b) {
      final aDelay = a.isAvailable ? a.delayMs : 0x7fffffff;
      final bDelay = b.isAvailable ? b.delayMs : 0x7fffffff;
      final byDelay = aDelay.compareTo(bDelay);
      if (byDelay != 0) return byDelay;
      final bySubscription =
          a.candidate.subscriptionId.compareTo(b.candidate.subscriptionId);
      if (bySubscription != 0) return bySubscription;
      return a.candidate.id.compareTo(b.candidate.id);
    });
    return results;
  }

  /// Connects to one candidate, requesting VPN permission when needed.
  Future<void> connect(
    ChimeraV2RayCandidate candidate, {
    bool proxyOnly = false,
  }) async {
    await initialize();
    final parsed = FlutterV2ray.parseFromURL(candidate.shareLink);
    if (!proxyOnly && !await _plugin.requestPermission()) {
      throw StateError('VPN permission was denied.');
    }
    await _plugin.startV2Ray(
      remark: parsed.remark,
      config: parsed.getFullConfiguration(),
      proxyOnly: proxyOnly,
    );
    _active = candidate;
  }

  /// Replaces the active plugin session and attempts to restore the old profile
  /// if startup fails. This is best-effort, not a seamless handover.
  Future<void> switchTo(
    ChimeraV2RayCandidate candidate, {
    bool proxyOnly = false,
  }) async {
    final previous = _active;
    await initialize();
    if (previous == null) {
      await connect(candidate, proxyOnly: proxyOnly);
      return;
    }
    await _plugin.stopV2Ray();
    _active = null;
    try {
      await connect(candidate, proxyOnly: proxyOnly);
    } catch (error) {
      try {
        await connect(previous, proxyOnly: proxyOnly);
      } catch (rollbackError) {
        throw StateError(
          'Candidate startup failed ($error); restoring previous profile '
          'also failed ($rollbackError).',
        );
      }
      rethrow;
    }
  }

  Future<void> disconnect() async {
    await _plugin.stopV2Ray();
    _active = null;
  }

  Future<int> connectedDelay() async {
    await initialize();
    final delay = await _plugin.getConnectedServerDelay();
    return delay < 0 ? -1 : delay;
  }
}
