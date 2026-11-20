import 'dart:async';
import 'dart:io';

import 'api.dart';

/// Go 后端 sidecar 生命周期管理。
///
/// 打包形态:`backend/m3u-scanner.exe` 与主程序同目录分发,
/// 启动时挑一个空闲端口拉起,退出时回收。开发时可通过环境变量
/// `M3U_SCANNER_BACKEND_URL` 直接连接已运行的后端,跳过 sidecar。
class Sidecar {
  Process? _process;
  bool _shuttingDown = false;

  final File exePath;
  Sidecar._(this.exePath);

  /// 启动并等待 API 就绪,返回连接用的客户端。
  /// [onLog] 输出启动过程,便于排障。
  static Future<({ApiClient api, Sidecar? sidecar})> launch(
    void Function(String) onLog,
  ) async {
    final envUrl = Platform.environment['M3U_SCANNER_BACKEND_URL'];
    if (envUrl != null && envUrl.isNotEmpty) {
      onLog('开发模式:直连 $envUrl');
      return (api: ApiClient(envUrl), sidecar: null);
    }

    final exe = _locateBackendExe();
    if (exe == null) {
      throw SidecarException(
          '未找到后端程序 backend/m3u-scanner.exe,请确认安装包完整。'
          '开发调试可设置环境变量 M3U_SCANNER_BACKEND_URL 指向已运行的后端。');
    }

    final port = await _findFreePort();
    onLog('启动后端 ${exe.path}(端口 $port)');
    final sidecar = Sidecar._(exe);
    try {
      sidecar._process = await Process.start(
        exe.path,
        ['-port', '$port', '-parent-pid', '$pid'],
        workingDirectory: exe.parent.path,
        mode: ProcessStartMode.normal,
      );
    } on ProcessException catch (e) {
      throw SidecarException('后端启动失败: ${e.message}');
    }

    final api = ApiClient('http://127.0.0.1:$port');
    final ready = await _waitReady(api, sidecar._process!, onLog);
    if (!ready) {
      api.close();
      await sidecar.shutdown();
      throw SidecarException('后端 15 秒内未就绪,请查看日志或重试。');
    }
    onLog('后端就绪');
    return (api: api, sidecar: sidecar);
  }

  static File? _locateBackendExe() {
    final exeDir = File(Platform.resolvedExecutable).parent;
    final name = Platform.isWindows ? 'm3u-scanner.exe' : 'm3u-scanner';
    final candidate =
        File('${exeDir.path}${Platform.pathSeparator}backend'
            '${Platform.pathSeparator}$name');
    return candidate.existsSync() ? candidate : null;
  }

  static Future<int> _findFreePort() async {
    final socket = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final port = socket.port;
    await socket.close();
    return port;
  }

  static Future<bool> _waitReady(
    ApiClient api,
    Process process,
    void Function(String) onLog,
  ) async {
    for (var i = 0; i < 30; i++) {
      // 进程提前退出:立即失败并记录
      try {
        final code =
            await process.exitCode.timeout(const Duration(milliseconds: 1));
        onLog('后端进程已退出 (exitCode=$code)');
        return false;
      } on TimeoutException {
        // 仍在运行,继续探测
      }
      if (await api.ping()) return true;
      await Future<void>.delayed(const Duration(milliseconds: 500));
    }
    return false;
  }

  /// 结束 sidecar 进程。幂等。
  Future<void> shutdown() async {
    if (_shuttingDown) return;
    _shuttingDown = true;
    final p = _process;
    _process = null;
    if (p == null) return;
    p.kill(ProcessSignal.sigterm);
    int? exited;
    try {
      exited = await p.exitCode.timeout(const Duration(seconds: 3));
    } on TimeoutException {
      // 未在宽限期内退出,强制结束
    }
    if (exited == null) p.kill(ProcessSignal.sigkill);
  }
}

class SidecarException implements Exception {
  final String message;
  SidecarException(this.message);

  @override
  String toString() => message;
}
