import 'dart:convert' as convert;

import 'package:flutter/services.dart';

/// Sanitized health sample sent from Dart to the Go failover policy.
/// Never include share links, credentials, or raw engine configuration here.
class ChimeraHealthCandidate {
  const ChimeraHealthCandidate({
    required this.id,
    required this.subscriptionId,
    required this.status,
    required this.latencyMs,
  });

  final String id;
  final String subscriptionId;
  final String status;
  final int latencyMs;

  Map<String, Object?> toJson() => {
        'id': id,
        'subscription_id': subscriptionId,
        'status': status,
        'latency_ms': latencyMs,
      };
}

class ChimeraFailoverDecision {
  const ChimeraFailoverDecision({
    required this.currentId,
    required this.shouldSwitch,
    required this.reason,
    this.targetId,
    this.targetLatencyMs,
  });

  final String currentId;
  final bool shouldSwitch;
  final String reason;
  final String? targetId;
  final int? targetLatencyMs;

  factory ChimeraFailoverDecision.fromJson(Map<Object?, Object?> json) {
    final currentId = json['current_id'];
    final shouldSwitch = json['switch'];
    final reason = json['reason'];
    if (currentId is! String || shouldSwitch is! bool || reason is! String) {
      throw const FormatException('Invalid Chimera decision response.');
    }
    final targetId = json['target_id'];
    final latency = json['target_latency_ms'];
    if (targetId != null && targetId is! String) {
      throw const FormatException('Invalid target_id in Chimera response.');
    }
    if (latency != null && latency is! int) {
      throw const FormatException('Invalid target latency in Chimera response.');
    }
    if (shouldSwitch && (targetId == null || targetId.isEmpty)) {
      throw const FormatException('Switch decision did not include target_id.');
    }
    return ChimeraFailoverDecision(
      currentId: currentId,
      shouldSwitch: shouldSwitch,
      reason: reason,
      targetId: targetId as String?,
      targetLatencyMs: latency as int?,
    );
  }
}

/// A verified, sanitized outcome. Failed observations must use latencyMs=-1.
class ChimeraConnectorOutcome {
  const ChimeraConnectorOutcome({
    required this.connectorId,
    required this.version,
    required this.success,
    required this.latencyMs,
  });

  final String connectorId;
  final String version;
  final bool success;
  final int latencyMs;

  Map<String, Object?> toJson() => {
        'connector_id': connectorId,
        'version': version,
        'success': success,
        'latency_ms': latencyMs,
      };
}

/// Dart side of the Android MethodChannel bridge to Chimera's Go policy API.
///
/// This bridge evaluates policy only. The app must use its single
/// ChimeraFlutterV2Ray session to perform a proposed switch, verify actual
/// connectivity, and then call confirmSwitch; on failure it should rollback
/// when possible and call cancelSwitch.
class ChimeraCoreBridge {
  ChimeraCoreBridge({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(_channelName);

  static const String _channelName = 'com.aminopy.chimera/core';
  final MethodChannel _channel;
  bool _initialized = false;

  Future<void> initialize({
    int failureThreshold = 2,
    int maxLatencyMs = 1500,
    int cooldownMs = 10000,
    int minImprovementMs = 100,
  }) async {
    if (failureThreshold < 1 ||
        failureThreshold > 100 ||
        maxLatencyMs < 1 ||
        cooldownMs < 0 ||
        minImprovementMs < 0) {
      throw ArgumentError('Invalid Chimera failover configuration.');
    }
    await _channel.invokeMethod<Object?>('initialize', {
      'config': _encodeConfig(
        failureThreshold: failureThreshold,
        maxLatencyMs: maxLatencyMs,
        cooldownMs: cooldownMs,
        minImprovementMs: minImprovementMs,
      ),
    });
    _initialized = true;
  }

  Future<ChimeraFailoverDecision> evaluate({
    required String currentId,
    required List<ChimeraHealthCandidate> candidates,
  }) async {
    _requireInitialized();
    if (currentId.trim().isEmpty || candidates.isEmpty || candidates.length > 64) {
      throw ArgumentError('Provide a current ID and 1 to 64 candidates.');
    }
    final ids = <String>{};
    const allowedStatuses = <String>{'healthy', 'degraded', 'failed', 'stopped'};
    for (final candidate in candidates) {
      if (candidate.id.trim().isEmpty ||
          candidate.id.length > 256 ||
          !ids.add(candidate.id) ||
          candidate.subscriptionId.length > 256 ||
          !allowedStatuses.contains(candidate.status) ||
          candidate.latencyMs < -1 ||
          candidate.latencyMs > 604800000) {
        throw ArgumentError('Invalid or duplicate candidate data.');
      }
    }
    if (!ids.contains(currentId)) {
      throw ArgumentError('currentId must match one candidate ID.');
    }
    final sample = <String, Object?>{
      'current_id': currentId,
      'candidates': candidates.map((candidate) => candidate.toJson()).toList(),
    };
    final response = await _channel.invokeMapMethod<Object?, Object?>(
      'evaluate',
      {'sample': _encode(sample)},
    );
    if (response == null) {
      throw const FormatException('Empty Chimera decision response.');
    }
    return ChimeraFailoverDecision.fromJson(response);
  }

  /// Record only after the host independently verifies the outcome.
  Future<void> recordOutcome(ChimeraConnectorOutcome outcome) async {
    if (outcome.connectorId.trim().isEmpty ||
        outcome.connectorId.length > 256 ||
        outcome.version.trim().isEmpty ||
        outcome.version.length > 256 ||
        outcome.latencyMs < -1 ||
        outcome.latencyMs > 604800000 ||
        (!outcome.success && outcome.latencyMs != -1)) {
      throw ArgumentError('Invalid connector outcome.');
    }
    await _channel.invokeMethod<Object?>('recordOutcome', {
      'observation': _encode(outcome.toJson()),
    });
  }

  /// Returns sanitized learning records, not profiles or credentials.
  Future<List<Map<String, Object?>>> learningSnapshot() async {
    final raw = await _channel.invokeMethod<String>('learningSnapshot');
    if (raw == null) {
      throw const FormatException('Empty learning snapshot.');
    }
    final decoded = convert.jsonDecode(raw);
    if (decoded is! List) {
      throw const FormatException('Invalid learning snapshot.');
    }
    return decoded.map((item) {
      if (item is! Map) {
        throw const FormatException('Invalid learning record.');
      }
      return Map<String, Object?>.from(item);
    }).toList(growable: false);
  }

  /// Export this string into platform-protected storage if persistence is needed.
  Future<String> exportLearning() async {
    final raw = await _channel.invokeMethod<String>('exportLearning');
    if (raw == null) {
      throw const FormatException('Empty learning export.');
    }
    return raw;
  }

  /// Restore a previously exported, versioned snapshot.
  Future<void> importLearning(String snapshotJson) async {
    if (snapshotJson.isEmpty || snapshotJson.length > 1024 * 1024) {
      throw ArgumentError('Invalid learning snapshot size.');
    }
    await _channel.invokeMethod<Object?>(
      'importLearning',
      {'snapshot': snapshotJson},
    );
  }

  Future<void> confirmSwitch(String targetId) async {
    _requireInitialized();
    if (targetId.trim().isEmpty || targetId.length > 256) {
      throw ArgumentError.value(targetId, 'targetId');
    }
    await _channel.invokeMethod<Object?>(
      'confirmSwitch',
      {'targetId': targetId},
    );
  }

  Future<void> cancelSwitch() async {
    _requireInitialized();
    await _channel.invokeMethod<Object?>('cancelSwitch');
  }

  void _requireInitialized() {
    if (!_initialized) {
      throw StateError('Call ChimeraCoreBridge.initialize() first.');
    }
  }

  String _encodeConfig({
    required int failureThreshold,
    required int maxLatencyMs,
    required int cooldownMs,
    required int minImprovementMs,
  }) =>
      _encode({
        'failure_threshold': failureThreshold,
        'max_latency_ms': maxLatencyMs,
        'cooldown_ms': cooldownMs,
        'min_improvement_ms': minImprovementMs,
      });

  String _encode(Map<String, Object?> value) => convert.jsonEncode(value);
}
