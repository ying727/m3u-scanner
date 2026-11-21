import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

import 'models.dart';

class ApiException implements Exception {
  final String message;
  final int? statusCode;
  ApiException(this.message, [this.statusCode]);

  @override
  String toString() => message;
}

/// Go 后端 HTTP API 客户端(契约见 internal/ui/server.go)。
class ApiClient {
  final String baseUrl;
  final http.Client _client;

  ApiClient(this.baseUrl, {http.Client? client})
      : _client = client ?? http.Client();

  Uri _uri(String path, [Map<String, String>? q]) =>
      Uri.parse('$baseUrl$path').replace(queryParameters: q);

  Future<Map<String, dynamic>> _getJson(String path) async {
    final res =
        await _client.get(_uri(path)).timeout(const Duration(seconds: 15));
    return _decode(res);
  }

  Future<Map<String, dynamic>> _postJson(String path, Object body) async {
    final res = await _client
        .post(_uri(path),
            headers: const {'Content-Type': 'application/json'},
            body: jsonEncode(body))
        .timeout(const Duration(seconds: 30));
    return _decode(res);
  }

  Map<String, dynamic> _decode(http.Response res) {
    final body = utf8.decode(res.bodyBytes, allowMalformed: true);
    if (res.statusCode >= 400) {
      String msg = 'HTTP ${res.statusCode}';
      try {
        msg = (jsonDecode(body) as Map<String, dynamic>)['error'] ?? msg;
      } catch (_) {}
      throw ApiException(msg, res.statusCode);
    }
    if (body.isEmpty) return const {};
    return jsonDecode(body) as Map<String, dynamic>;
  }

  /// 后端是否就绪(用于 sidecar 启动探测)。
  Future<bool> ping() async {
    try {
      await _client
          .get(_uri('/api/status'))
          .timeout(const Duration(seconds: 2));
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<BackendStatus> status() async =>
      BackendStatus.fromJson(await _getJson('/api/status'));

  Future<ResultsPayload> results() async =>
      ResultsPayload.fromJson(await _getJson('/api/results'));

  Future<Settings> getSettings() async =>
      Settings.fromJson(await _getJson('/api/settings'));

  Future<void> saveSettings(Settings s) async {
    await _postJson('/api/settings', s.toJson());
  }

  /// 上传本地 m3u 文件(字段名必须为 `file`)。
  Future<void> uploadM3U(Uint8List bytes, String filename) async {
    final req = http.MultipartRequest('POST', _uri('/api/upload'))
      ..files.add(http.MultipartFile.fromBytes('file', bytes,
          filename: filename));
    final res = await req.send().timeout(const Duration(minutes: 2));
    final body = await res.stream.bytesToString();
    if (res.statusCode >= 400) {
      throw ApiException(_errorMessage(body), res.statusCode);
    }
  }

  Future<void> loadUrl(String url) async => _postJson('/api/load-url', {'url': url});

  Future<void> startScan() async => _postJson('/api/scan/start', const {});

  Future<void> stopScan() async => _postJson('/api/scan/stop', const {});

  Future<void> scanFailed() async => _postJson('/api/scan/failed', const {});

  Future<void> clearResults() async => _postJson('/api/results/clear', const {});

  String streamUrl(String url) => '${_uri('/api/stream', {'url': url})}';

  /// 下载导出文件(kind: m3u | json | csv)。
  Future<Uint8List> export(String kind) async {
    final res = await _client
        .get(_uri(kind == 'm3u'
            ? '/api/export'
            : kind == 'csv'
                ? '/api/export/csv'
                : '/api/export/json'))
        .timeout(const Duration(seconds: 30));
    if (res.statusCode >= 400) throw ApiException('导出失败 HTTP ${res.statusCode}');
    return res.bodyBytes;
  }

  /// SSE 事件流。后端只发裸字符串信号(progress / scan_complete / epg_loaded)。
  /// 服务端正常断开时 2 秒后自动重连;异常会终止流,由调用方决定重试节奏。
  Stream<String> events() async* {
    while (true) {
      final req = http.Request('GET', _uri('/api/events'))
        ..headers['Accept'] = 'text/event-stream';
      final res = await _client.send(req).timeout(const Duration(seconds: 10));
      if (res.statusCode >= 400) {
        throw ApiException('SSE 连接失败 HTTP ${res.statusCode}');
      }
      await for (final line in res.stream
          .transform(utf8.decoder)
          .transform(const LineSplitter())) {
        if (line.startsWith('data: ')) {
          yield line.substring(6).trim();
        }
      }
      await Future<void>.delayed(const Duration(seconds: 2));
    }
  }

  String _errorMessage(String body) {
    try {
      return (jsonDecode(body) as Map<String, dynamic>)['error'] ??
          '上传失败 HTTP ${body.length}';
    } catch (_) {
      return '上传失败';
    }
  }

  void close() => _client.close();
}
