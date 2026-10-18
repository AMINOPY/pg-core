import 'dart:convert' as convert;

import 'package:flutter/services.dart';

/// Sanitized, user-approved measurements from a speed-test and connectivity checks.
/// Never include phone numbers, SIM serials, IMSI/ICCID, precise location, or credentials.
class ChimeraNetworkReport {
  const ChimeraNetworkReport({
    required this.networkType,
    required this.downloadMbps,
    required this.uploadMbps,
    required this.latencyMs,
    required this.jitterMs,
    required this.packetLossPct,
    required this.dnsLatencyMs,
    this.cellularGeneration,
    this.carrier,
    this.simState,
    this.roaming = false,
    this.signalStrengthDbm,
    this.checks = const [],
  });

  final String networkType;
  final String? cellularGeneration;
  final String? carrier;
  final String? simState;
  final bool roaming;
  final double downloadMbps;
  final double uploadMbps;
  final double latencyMs;
  final double jitterMs;
  final double packetLossPct;
  final double dnsLatencyMs;
  final int? signalStrengthDbm;
  final List<Map<String, Object?>> checks;

  Map<String, Object?> toJson() => {
        'network_type': networkType,
        if (cellularGeneration != null) 'cellular_generation': cellularGeneration,
        if (carrier != null) 'carrier': carrier,
        if (simState != null) 'sim_state': simState,
        'roaming': roaming,
        'download_mbps': downloadMbps,
        'upload_mbps': uploadMbps,
        'latency_ms': latencyMs,
        'jitter_ms': jitterMs,
        'packet_loss_pct': packetLossPct,
        'dns_latency_ms': dnsLatencyMs,
        if (signalStrengthDbm != null) 'signal_strength_dbm': signalStrengthDbm,
        if (checks.isNotEmpty) 'checks': checks,
      };
}

class ChimeraAIFinding {
  const ChimeraAIFinding({
    required this.code,
    required this.severity,
    required this.confidence,
    required this.summary,
    required this.advice,
  });

  final String code;
  final String severity;
  final double confidence;
  final String summary;
  final String advice;

  factory ChimeraAIFinding.fromJson(Map<Object?, Object?> json) {
    final code = json['code'];
    final severity = json['severity'];
    final confidence = json['confidence'];
    final summary = json['summary'];
    final advice = json['advice'];
    if (code is! String ||
        severity is! String ||
        confidence is! num ||
        summary is! String ||
        advice is! String) {
      throw const FormatException('Invalid AI finding.');
    }
    return ChimeraAIFinding(
      code: code,
      severity: severity,
      confidence: confidence.toDouble(),
      summary: summary,
      advice: advice,
    );
  }
}

class ChimeraAIAnalysis {
  const ChimeraAIAnalysis({
    required this.score,
    required this.grade,
    required this.findings,
    required this.limitations,
  });

  final int score;
  final String grade;
  final List<ChimeraAIFinding> findings;
  final List<String> limitations;

  factory ChimeraAIAnalysis.fromJson(Map<Object?, Object?> json) {
    final schemaVersion = json['schema_version'];
    final score = json['network_score'];
    final grade = json['grade'];
    final rawFindings = json['findings'];
    final rawLimitations = json['limitations'];
    if (schemaVersion != 1 ||
        score is! int ||
        score < 0 ||
        score > 100 ||
        grade is! String ||
        rawFindings is! List ||
        rawLimitations is! List) {
      throw const FormatException('Invalid AI analysis response.');
    }
    return ChimeraAIAnalysis(
      score: score,
      grade: grade,
      findings: rawFindings.map((item) {
        if (item is! Map) throw const FormatException('Invalid AI finding.');
        return ChimeraAIFinding.fromJson(Map<Object?, Object?>.from(item));
      }).toList(growable: false),
      limitations: rawLimitations.map((item) {
        if (item is! String) throw const FormatException('Invalid limitation.');
        return item;
      }).toList(growable: false),
    );
  }
}

/// Calls the local Go rule-based analyzer through the Android MethodChannel.
/// This does not itself run a speed test or request SIM permissions.
class ChimeraAI {
  ChimeraAI({MethodChannel? channel})
      : _channel = channel ?? const MethodChannel(_channelName);

  static const String _channelName = 'com.aminopy.chimera/core';
  final MethodChannel _channel;

  Future<ChimeraAIAnalysis> analyze(ChimeraNetworkReport report) async {
    final raw = convert.jsonEncode(report.toJson());
    if (raw.length > 64 * 1024) {
      throw ArgumentError('Network report exceeds the size limit.');
    }
    final response = await _channel.invokeMapMethod<Object?, Object?>(
      'analyzeNetwork',
      {'report': raw},
    );
    if (response == null) {
      throw const FormatException('Empty AI analysis response.');
    }
    return ChimeraAIAnalysis.fromJson(response);
  }
}
