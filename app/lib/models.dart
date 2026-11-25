/// 与 Go 后端 (`internal/ui/server.go` / `internal/scanner/scanner.go`) 对齐的数据模型。
library;

class Channel {
  final String name;
  final String url;
  final String groupTitle;
  final String logo;
  final String tvgId;
  final String tvgName;

  const Channel({
    required this.name,
    required this.url,
    this.groupTitle = '',
    this.logo = '',
    this.tvgId = '',
    this.tvgName = '',
  });

  factory Channel.fromJson(Map<String, dynamic> j) => Channel(
        name: (j['name'] ?? '') as String,
        url: (j['url'] ?? '') as String,
        groupTitle: (j['group_title'] ?? '') as String,
        logo: (j['logo'] ?? '') as String,
        tvgId: (j['tvg_id'] ?? '') as String,
        tvgName: (j['tvg_name'] ?? '') as String,
      );
}

class FormatInfo {
  final String name;
  final int bitRate;

  const FormatInfo({required this.name, this.bitRate = 0});

  factory FormatInfo.fromJson(Map<String, dynamic> j) => FormatInfo(
        name: (j['name'] ?? '') as String,
        bitRate: _asInt(j['bit_rate']),
      );
}

class VideoStream {
  final String codec;
  final int width;
  final int height;
  final double frameRate;
  final String pixelFormat;

  const VideoStream({
    required this.codec,
    this.width = 0,
    this.height = 0,
    this.frameRate = 0,
    this.pixelFormat = '',
  });

  factory VideoStream.fromJson(Map<String, dynamic> j) => VideoStream(
        codec: (j['codec'] ?? '') as String,
        width: _asInt(j['width']),
        height: _asInt(j['height']),
        frameRate: _asDouble(j['frame_rate']),
        pixelFormat: (j['pixel_format'] ?? '') as String,
      );
}

class AudioStream {
  final String codec;
  final int sampleRate;
  final int channels;

  const AudioStream({required this.codec, this.sampleRate = 0, this.channels = 0});

  factory AudioStream.fromJson(Map<String, dynamic> j) => AudioStream(
        codec: (j['codec'] ?? '') as String,
        sampleRate: _asInt(j['sample_rate']),
        channels: _asInt(j['channels']),
      );
}

/// StreamInfo。注意 Go 侧 `ResponseTime time.Duration` 序列化为纳秒整数。
class StreamInfo {
  final bool available;
  final int responseTimeNs;
  final String? error;
  final String? errorType;
  final String? usedFallback;
  final FormatInfo? format;
  final List<VideoStream> videoStreams;
  final List<AudioStream> audioStreams;

  const StreamInfo({
    required this.available,
    this.responseTimeNs = 0,
    this.error,
    this.errorType,
    this.usedFallback,
    this.format,
    this.videoStreams = const [],
    this.audioStreams = const [],
  });

  factory StreamInfo.fromJson(Map<String, dynamic> j) => StreamInfo(
        available: (j['available'] ?? false) as bool,
        responseTimeNs: _asInt(j['response_time']),
        error: j['error'] as String?,
        errorType: j['error_type'] as String?,
        usedFallback: j['used_fallback'] as String?,
        format: j['format'] == null
            ? null
            : FormatInfo.fromJson(j['format'] as Map<String, dynamic>),
        videoStreams: _list(j['video_streams'], VideoStream.fromJson),
        audioStreams: _list(j['audio_streams'], AudioStream.fromJson),
      );

  /// 首个视频流的分辨率文本,如 "1920×1080"。
  String get resolution {
    if (videoStreams.isEmpty) return '';
    final v = videoStreams.first;
    if (v.width <= 0 || v.height <= 0) return '';
    return '${v.width}×${v.height}';
  }

  /// 响应时间毫秒。
  int get responseTimeMs => responseTimeNs ~/ 1000000;
}

class ScanResult {
  final Channel channel;
  final StreamInfo? streamInfo;
  final DateTime? scannedAt;

  const ScanResult({required this.channel, this.streamInfo, this.scannedAt});

  factory ScanResult.fromJson(Map<String, dynamic> j) => ScanResult(
        channel: Channel.fromJson(j['channel'] as Map<String, dynamic>),
        streamInfo: j['stream_info'] == null
            ? null
            : StreamInfo.fromJson(j['stream_info'] as Map<String, dynamic>),
        scannedAt: j['scanned_at'] == null
            ? null
            : DateTime.tryParse(j['scanned_at'] as String),
      );

  bool get available => streamInfo?.available ?? false;
}

class ScanProgress {
  final int total;
  final int completed;
  final int available;
  final int failed;

  const ScanProgress({
    this.total = 0,
    this.completed = 0,
    this.available = 0,
    this.failed = 0,
  });

  factory ScanProgress.fromJson(Map<String, dynamic> j) => ScanProgress(
        total: _asInt(j['total']),
        completed: _asInt(j['completed']),
        available: _asInt(j['available']),
        failed: _asInt(j['failed']),
      );

  double get fraction => total <= 0 ? 0 : (completed / total).clamp(0.0, 1.0);
}

/// GET /api/settings 与 POST /api/settings 的载荷。
class Settings {
  final int concurrency;
  final int timeout;
  final bool quickCheck;
  final String userAgent;

  const Settings({
    this.concurrency = 20,
    this.timeout = 15,
    this.quickCheck = false,
    this.userAgent = '',
  });

  factory Settings.fromJson(Map<String, dynamic> j) => Settings(
        concurrency: _asInt(j['concurrency'], fallback: 20),
        timeout: _asInt(j['timeout'], fallback: 15),
        quickCheck: (j['quickCheck'] ?? false) as bool,
        userAgent: (j['userAgent'] ?? '') as String,
      );

  Map<String, dynamic> toJson() => {
        'concurrency': concurrency,
        'timeout': timeout,
        'quickCheck': quickCheck,
        'userAgent': userAgent,
      };
}

/// GET /api/status。
class BackendStatus {
  final bool ffprobeAvailable;
  final String ffprobePath;

  const BackendStatus({required this.ffprobeAvailable, this.ffprobePath = ''});

  factory BackendStatus.fromJson(Map<String, dynamic> j) => BackendStatus(
        ffprobeAvailable: (j['ffprobe'] ?? false) as bool,
        ffprobePath: (j['ffprobePath'] ?? '') as String,
      );
}

/// GET /api/results。
class ResultsPayload {
  final List<ScanResult> results;
  final ScanProgress progress;
  final bool scanning;
  final DateTime? lastScanTime;

  const ResultsPayload({
    this.results = const [],
    this.progress = const ScanProgress(),
    this.scanning = false,
    this.lastScanTime,
  });

  factory ResultsPayload.fromJson(Map<String, dynamic> j) => ResultsPayload(
        results: _list(j['results'], ScanResult.fromJson),
        progress: j['progress'] == null
            ? const ScanProgress()
            : ScanProgress.fromJson(j['progress'] as Map<String, dynamic>),
        scanning: (j['scanning'] ?? false) as bool,
        lastScanTime: j['lastScanTime'] == null
            ? null
            : DateTime.tryParse(j['lastScanTime'] as String),
      );
}

int _asInt(dynamic v, {int fallback = 0}) =>
    v is int ? v : (v is num ? v.toInt() : fallback);

double _asDouble(dynamic v) => v is num ? v.toDouble() : 0;

List<T> _list<T>(dynamic v, T Function(Map<String, dynamic>) from) =>
    v is List ? v.whereType<Map<String, dynamic>>().map(from).toList() : const [];
