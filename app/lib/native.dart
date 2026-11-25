import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:ffi/ffi.dart';

/// FFI 原生回调:接收 JSON 事件字符串,宿主负责释放。
typedef _ProgressNative = Void Function(Pointer<Utf8>);

typedef _StrFnNative = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _StrFnDart = Pointer<Utf8> Function(Pointer<Utf8>);

typedef _VoidNative = Void Function();
typedef _VoidDart = void Function();

typedef _FreeNative = Void Function(Pointer<Utf8>);
typedef _FreeDart = void Function(Pointer<Utf8>);

typedef _SetPathNative = Void Function(Pointer<Utf8>);
typedef _SetPathDart = void Function(Pointer<Utf8>);

typedef _SetCbNative = Void Function(Pointer<NativeFunction<_ProgressNative>>);
typedef _SetCbDart = void Function(Pointer<NativeFunction<_ProgressNative>>);

typedef _ReleaseNative = Void Function(LongLong);
typedef _ReleaseDart = void Function(int);

typedef _IntNative = Int32 Function();
typedef _IntDart = int Function();

/// 扫描事件,来自原生回调。
sealed class ScanEvent {
  const ScanEvent();
  static ScanEvent parse(String json) {
    final m = jsonDecode(json) as Map<String, dynamic>;
    return switch (m['type']) {
      'progress' => ScanProgressEvent(
          total: (m['total'] as num?)?.toInt() ?? 0,
          completed: (m['completed'] as num?)?.toInt() ?? 0,
          available: (m['available'] as num?)?.toInt() ?? 0,
          failed: (m['failed'] as num?)?.toInt() ?? 0,
        ),
      'complete' => ScanCompleteEvent(m['results'] as Map<String, dynamic>?),
      'cancelled' => const ScanCancelledEvent(),
      'error' => ScanFailedEvent(m['error'] as String? ?? '未知错误'),
      _ => const ScanCancelledEvent(),
    };
  }
}

class ScanProgressEvent extends ScanEvent {
  final int total, completed, available, failed;
  const ScanProgressEvent({
    required this.total,
    required this.completed,
    required this.available,
    required this.failed,
  });
  double get fraction => total <= 0 ? 0 : (completed / total).clamp(0.0, 1.0);
}

class ScanCompleteEvent extends ScanEvent {
  final Map<String, dynamic>? results;
  const ScanCompleteEvent(this.results);
}

class ScanCancelledEvent extends ScanEvent {
  const ScanCancelledEvent();
}

class ScanFailedEvent extends ScanEvent {
  final String message;
  const ScanFailedEvent(this.message);
}

class NativeError implements Exception {
  final String message;
  NativeError(this.message);
  @override
  String toString() => message;
}

/// 解析结果:句柄 + 播放列表。
class ParsedPlaylist {
  final int handle;
  final List<Map<String, dynamic>> channels;
  final String epgUrl;
  ParsedPlaylist(this.handle, this.channels, this.epgUrl);
}

/// Go 核心库(m3u_core.dll)的 FFI 绑定。
///
/// 全部处理在进程内完成:没有 HTTP 服务、没有端口、没有 sidecar 进程。
class NativeCore {
  final DynamicLibrary _lib;
  late final _StrFnDart _parse;
  late final _StrFnDart _scan;
  late final _VoidDart _cancelScan;
  late final _StrFnDart _scanState;
  late final _ReleaseDart _releaseHandle;
  late final _SetPathDart _setFFprobePath;
  late final _FreeDart _freeString;
  late final _SetCbDart _setCallback;
  late final _IntDart _ffprobeAvailable;

  NativeCallable<_ProgressNative>? _callback;

  /// 扫描事件广播;由 AppState 订阅。
  final _listeners = <void Function(ScanEvent)>[];

  NativeCore._(this._lib) {
    _parse = _lib.lookupFunction<_StrFnNative, _StrFnDart>('M3UParse');
    _scan = _lib.lookupFunction<_StrFnNative, _StrFnDart>('M3UScan');
    _cancelScan = _lib.lookupFunction<_VoidNative, _VoidDart>('M3UCancelScan');
    _scanState = _lib.lookupFunction<_StrFnNative, _StrFnDart>('M3UScanState');
    _releaseHandle = _lib.lookupFunction<_ReleaseNative, _ReleaseDart>('M3UReleaseHandle');
    _setFFprobePath = _lib.lookupFunction<_SetPathNative, _SetPathDart>('M3USetFFprobePath');
    _freeString = _lib.lookupFunction<_FreeNative, _FreeDart>('M3UFreeString');
    _setCallback = _lib.lookupFunction<_SetCbNative, _SetCbDart>('M3USetProgressCallback');
    _ffprobeAvailable = _lib.lookupFunction<_IntNative, _IntDart>('M3UFFprobeAvailable');

    // listener 模式:回调从 Go 的 goroutine 线程安全地投递到 Dart 主 isolate
    _callback = NativeCallable<_ProgressNative>.listener(_onNativeEvent);
    _setCallback(_callback!.nativeFunction);

    _freeStringPtr = _freeString;
  }

  late final _FreeDart _freeStringPtr;

  void _onNativeEvent(Pointer<Utf8> eventJson) {
    try {
      final json = eventJson.toDartString();
      final event = ScanEvent.parse(json);
      for (final l in List.of(_listeners)) {
        l(event);
      }
    } finally {
      _freeStringPtr(eventJson);
    }
  }

  /// 加载随程序分发的 m3u_core 动态库,并指向同目录的 ffprobe。
  static NativeCore load({String? ffprobePath}) {
    final exeDir = File(Platform.resolvedExecutable).parent.path;
    final libPath = Platform.isWindows
        ? '$exeDir${Platform.pathSeparator}m3u_core.dll'
        : '$exeDir${Platform.pathSeparator}libm3u_core.so';

    final core = NativeCore._(DynamicLibrary.open(libPath));

    final probe = ffprobePath ??
        (Platform.isWindows
            ? '$exeDir${Platform.pathSeparator}ffprobe.exe'
            : '$exeDir${Platform.pathSeparator}ffprobe');
    if (File(probe).existsSync()) {
      core.setFFprobePath(probe);
    }
    return core;
  }

  void addListener(void Function(ScanEvent) f) => _listeners.add(f);
  void removeListener(void Function(ScanEvent) f) => _listeners.remove(f);

  void setFFprobePath(String path) {
    final p = path.toNativeUtf8();
    _setFFprobePath(p);
    calloc.free(p);
  }

  /// 解析本地文件或 URL,返回句柄与频道列表。
  ParsedPlaylist parse({String? url, String? filePath, String userAgent = ''}) {
    final out = _callJson(_parse, {
      'url': ?url,
      'file_path': ?filePath,
      'user_agent': userAgent,
    });
    final handle = (out['handle'] as num?)?.toInt() ?? 0;
    final pl = out['playlist'] as Map<String, dynamic>? ?? const {};
    final channels = (pl['channels'] as List?)
            ?.whereType<Map<String, dynamic>>()
            .toList() ??
        const [];
    return ParsedPlaylist(handle, channels, (pl['epg_url'] as String?) ?? '');
  }

  /// 启动扫描(异步),进度与结果通过事件回调送达。
  void startScan({
    required int handle,
    required int concurrency,
    required int timeoutSec,
    required bool quickCheck,
    String userAgent = '',
  }) {
    _callJson(_scan, {
      'handle': handle,
      'concurrency': concurrency,
      'timeout_sec': timeoutSec,
      'quick_check': quickCheck,
      'user_agent': userAgent,
    });
  }

  void cancelScan() => _cancelScan();

  /// ffprobe 是否可用(深度检测依赖它)。
  bool get ffprobeAvailable => _ffprobeAvailable() == 1;

  Map<String, dynamic> scanState() {
    final p = ''.toNativeUtf8();
    final r = _scanState(p);
    calloc.free(p);
    try {
      return jsonDecode(r.toDartString()) as Map<String, dynamic>;
    } finally {
      _freeString(r);
    }
  }

  void releaseHandle(int handle) => _releaseHandle(handle);

  /// 处理 native 返回:UTF-8 JSON,含 error 字段即抛异常。
  Map<String, dynamic> _callJson(
    _StrFnDart fn,
    Map<String, dynamic> request,
  ) {
    final input = jsonEncode(request).toNativeUtf8();
    final result = fn(input);
    calloc.free(input);
    try {
      final text = result.toDartString();
      final m = jsonDecode(text) as Map<String, dynamic>;
      if (m['error'] != null) {
        throw NativeError(m['error'] as String);
      }
      return m;
    } finally {
      _freeString(result);
    }
  }

  void dispose() {
    _listeners.clear();
    _setCallback(Pointer.fromAddress(0));
    _callback?.close();
    _callback = null;
  }
}
