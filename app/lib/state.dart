import 'dart:async';

import 'package:flutter/foundation.dart';

import 'api.dart';
import 'models.dart';
import 'sidecar.dart';

enum BootState { booting, ready, error }

/// 全局应用状态:sidecar 启动、数据拉取、SSE 信号驱动刷新。
class AppState extends ChangeNotifier {
  BootState bootState = BootState.booting;
  String? bootError;
  final List<String> bootLog = [];

  late ApiClient api;
  Sidecar? _sidecar;
  StreamSubscription<String>? _sseSub;
  Timer? _refreshDebounce;

  ResultsPayload payload = const ResultsPayload();
  BackendStatus? backendStatus;
  Settings settings = const Settings();

  // 列表过滤
  String searchText = '';
  String? groupFilter; // null = 全部分组
  bool onlyAvailable = false;

  ScanResult? selected;
  String? lastActionError;
  String? sourceLabel; // 当前加载的源(文件名或 URL),仅会话内展示用

  Future<void> init() async {
    try {
      final launched = await Sidecar.launch(_log);
      api = launched.api;
      _sidecar = launched.sidecar;

      final results = await api.results();
      payload = results;
      final status = await api.status();
      backendStatus = status;
      try {
        settings = await api.getSettings();
      } on ApiException {
        settings = const Settings();
      }
      bootState = BootState.ready;
      notifyListeners();
      _listenSse();
    } catch (e) {
      _log(e.toString());
      bootError = e.toString();
      bootState = BootState.error;
      notifyListeners();
    }
  }

  void _log(String line) {
    bootLog.add(line);
    if (bootLog.length > 200) bootLog.removeRange(0, bootLog.length - 200);
    debugPrint('[sidecar] $line');
  }

  void _listenSse() {
    _sseSub?.cancel();
    _sseSub = api.events().listen(
      (event) {
        // 后端只发信号不带货,收到后重新拉取
        switch (event) {
          case 'progress':
          case 'scan_complete':
          case 'epg_loaded':
            scheduleRefresh();
        }
      },
      onDone: () {
        // 流终止(断线):稍后重连
        if (bootState == BootState.ready) {
          Future.delayed(const Duration(seconds: 3), _listenSse);
        }
      },
      onError: (Object _) {
        if (bootState == BootState.ready) {
          Future.delayed(const Duration(seconds: 3), _listenSse);
        }
      },
      cancelOnError: true,
    );
  }

  /// SSE 信号可能高频到达,合并成单次拉取。
  void scheduleRefresh() {
    _refreshDebounce?.cancel();
    _refreshDebounce = Timer(const Duration(milliseconds: 150), () async {
      try {
        payload = await api.results();
        notifyListeners();
      } on ApiException {
        // 拉取失败保持旧数据,等下一次信号
      }
    });
  }

  /// 启动失败后的重试入口(重置引导状态)。
  void resetBoot() {
    bootState = BootState.booting;
    bootError = null;
    bootLog.clear();
    notifyListeners();
  }

  Future<void> refreshAll() async {
    payload = await api.results();
    backendStatus = await api.status();
    notifyListeners();
  }

  // ---- 操作 ----

  Future<void> uploadM3U(Uint8List bytes, String filename) =>
      _action(() async {
        await api.uploadM3U(bytes, filename);
        sourceLabel = filename;
        await refreshAll();
        return '已加载 $filename';
      });

  Future<void> loadUrl(String url) => _action(() async {
        await api.loadUrl(url);
        sourceLabel = url;
        await refreshAll();
        return '已加载源';
      });

  Future<void> startScan() => _action(() async {
        await api.startScan();
        scheduleRefresh();
        return null;
      });

  Future<void> stopScan() => _action(() async {
        await api.stopScan();
        scheduleRefresh();
        return null;
      });

  Future<void> rescanFailed() => _action(() async {
        await api.scanFailed();
        scheduleRefresh();
        return null;
      });

  Future<void> clearResults() => _action(() async {
        await api.clearResults();
        await refreshAll();
        return '已清空结果';
      });

  Future<void> saveSettings(Settings s) => _action(() async {
        await api.saveSettings(s);
        settings = s;
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

  /// 统一处理操作反馈与错误。
  Future<void> _action(Future<String?> Function() fn) async {
    try {
      final msg = await fn();
      lastActionError = null;
      if (msg != null) notifyListeners();
    } on ApiException catch (e) {
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
    if (onlyAvailable) {
      list = list.where((r) => r.available);
    }
    if (groupFilter != null) {
      list = list.where((r) => r.channel.groupTitle.trim() == groupFilter);
    }
    final q = searchText.trim().toLowerCase();
    if (q.isNotEmpty) {
      list = list.where((r) => r.channel.name.toLowerCase().contains(q));
    }
    return list.toList();
  }

  int get availableCount =>
      payload.results.where((r) => r.available).length;

  @override
  void dispose() {
    _sseSub?.cancel();
    _refreshDebounce?.cancel();
    super.dispose();
  }

  /// 应用退出前调用:停 SSE、关 sidecar。
  Future<void> shutdown() async {
    await _sseSub?.cancel();
    await _sidecar?.shutdown();
  }
}
