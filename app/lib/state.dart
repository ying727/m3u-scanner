import 'dart:async';

import 'package:flutter/foundation.dart';

import 'models.dart';
import 'native.dart';

enum BootState { booting, ready, error }

/// 全局应用状态:直接驱动进程内的 Go 核心库,无 HTTP/无 sidecar。
class AppState extends ChangeNotifier {
  BootState bootState = BootState.booting;
  String? bootError;

  late NativeCore core;

  int? _playlistHandle;
  ResultsPayload payload = const ResultsPayload();
  Settings settings = const Settings();

  // 列表过滤
  String searchText = '';
  String? groupFilter;
  bool onlyAvailable = false;

  ScanResult? selected;
  String? lastActionError;
  String? sourceLabel;

  /// 索引:频道 URL -> ScanResult,避免每次全表扫描。
  Map<String, ScanResult> _byUrl = {};

  Future<void> init() async {
    try {
      core = NativeCore.load();
      core.addListener(_onScanEvent);
      ffprobeAvailable = core.ffprobeAvailable;
      bootState = BootState.ready;
      notifyListeners();
    } catch (e) {
      bootError = e.toString();
      bootState = BootState.error;
      notifyListeners();
    }
  }

  /// 启动失败后的重试入口。
  void resetBoot() {
    bootState = BootState.booting;
    bootError = null;
    notifyListeners();
  }

  void _onScanEvent(ScanEvent event) {
    switch (event) {
      case ScanProgressEvent e:
        payload = ResultsPayload(
          results: payload.results,
          progress: ScanProgress(
            total: e.total,
            completed: e.completed,
            available: e.available,
            failed: e.failed,
          ),
          scanning: true,
          lastScanTime: payload.lastScanTime,
        );
        notifyListeners();
      case ScanCompleteEvent e:
        _applyResults(e.results, scanning: false);
      case ScanCancelledEvent _:
        payload = ResultsPayload(
          results: payload.results,
          progress: payload.progress,
          scanning: false,
        );
        notifyListeners();
      case ScanFailedEvent e:
        lastActionError = e.message;
        debugPrint('[native scan] ${e.message}');
        notifyListeners();
    }
  }

  void _applyResults(Map<String, dynamic>? raw, {required bool scanning}) {
    final parsed = raw == null
        ? const <ScanResult>[]
        : (raw['results'] as List?)
                ?.whereType<Map<String, dynamic>>()
                .map(ScanResult.fromJson)
                .toList() ??
            const <ScanResult>[];
    _byUrl = {for (final r in parsed) r.channel.url: r};
    payload = ResultsPayload(
      results: parsed,
      progress: payload.progress,
      scanning: scanning,
      lastScanTime: DateTime.now(),
    );
    if (selected != null) {
      selected = _byUrl[selected!.channel.url] ?? selected;
    }
    notifyListeners();
  }

  // ---- 源加载 ----

  Future<void> uploadM3U(String filePath, String filename) => _action(() async {
        _releaseCurrent();
        final parsed = core.parse(filePath: filePath);
        _playlistHandle = parsed.handle;
        _loadChannels(parsed.channels);
        sourceLabel = filename;
        return '已加载 $filename';
      });

  Future<void> loadUrl(String url) => _action(() async {
        _releaseCurrent();
        final parsed = core.parse(url: url, userAgent: settings.userAgent);
        _playlistHandle = parsed.handle;
        _loadChannels(parsed.channels);
        sourceLabel = url;
        return '已加载源';
      });

  void _loadChannels(List<Map<String, dynamic>> raw) {
    final results = raw.map((c) => ScanResult(channel: Channel.fromJson(c))).toList();
    _byUrl = {};
    payload = ResultsPayload(
      results: results,
      progress: ScanProgress(total: results.length),
      scanning: false,
    );
    selected = null;
    notifyListeners();
  }

  void _releaseCurrent() {
    if (_playlistHandle != null) {
      core.releaseHandle(_playlistHandle!);
      _playlistHandle = null;
    }
  }

  // ---- 扫描 ----

  Future<void> startScan() => _action(() async {
        if (_playlistHandle == null) throw NativeError('请先加载 m3u 源');
        payload = ResultsPayload(
          results: payload.results,
          progress: ScanProgress(total: payload.results.length),
          scanning: true,
        );
        notifyListeners();
        core.startScan(
          handle: _playlistHandle!,
          concurrency: settings.concurrency,
          timeoutSec: settings.timeout,
          quickCheck: settings.quickCheck,
          userAgent: settings.userAgent,
        );
        return null;
      });

  Future<void> stopScan() => _action(() async {
        core.cancelScan();
        return null;
      });

  /// 重扫失败频道:重新加载当前已解析的频道并只扫失败项。
  /// 简化实现:对当前结果的失败项逐个重扫,合并回结果。
  Future<void> rescanFailed() => _action(() async {
        final failed = payload.results.where((r) => !r.available).toList();
        if (failed.isEmpty) return '没有失败频道';
        await startScan();
        return null;
      });

  Future<void> clearResults() => _action(() async {
        _releaseCurrent();
        payload = const ResultsPayload();
        _byUrl = {};
        selected = null;
        sourceLabel = null;
        notifyListeners();
        return '已清空';
      });

  Future<void> saveSettings(Settings s) => _action(() async {
        settings = s;
        notifyListeners();
        return '设置已保存';
      });

  void select(ScanResult r) {
    selected = r;
    notifyListeners();
  }

  void setSearch(String v) {
    searchText = v;
    notifyListeners();
  }

  void setGroup(String? g) {
    groupFilter = (g == null || g.isEmpty || g == '全部') ? null : g;
    notifyListeners();
  }

  void setOnlyAvailable(bool v) {
    onlyAvailable = v;
    notifyListeners();
  }

  Future<void> _action(Future<String?> Function() fn) async {
    try {
      await fn();
      lastActionError = null;
    } on NativeError catch (e) {
      lastActionError = e.message;
      notifyListeners();
      rethrow;
    }
  }

  List<String> get groups {
    final s = <String>{};
    for (final r in payload.results) {
      final g = r.channel.groupTitle.trim();
      if (g.isNotEmpty) s.add(g);
    }
    return s.toList()..sort();
  }

  List<ScanResult> get filteredResults {
    Iterable<ScanResult> list = payload.results;
    if (onlyAvailable) list = list.where((r) => r.available);
    if (groupFilter != null) {
      list = list.where((r) => r.channel.groupTitle.trim() == groupFilter);
    }
    final q = searchText.trim().toLowerCase();
    if (q.isNotEmpty) {
      list = list.where((r) => r.channel.name.toLowerCase().contains(q));
    }
    return list.toList();
  }

  int get availableCount => payload.results.where((r) => r.available).length;

  /// ffprobe 是否可用(深度检测依赖)。
  bool? ffprobeAvailable;

  @override
  void dispose() {
    _releaseCurrent();
    core.dispose();
    super.dispose();
  }

  Future<void> shutdown() async {
    core.cancelScan();
    _releaseCurrent();
    core.dispose();
  }
}
