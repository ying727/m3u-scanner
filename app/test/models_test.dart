import 'package:flutter_test/flutter_test.dart';
import 'package:m3u_scanner_app/models.dart';

void main() {
  group('StreamInfo', () {
    test('解析完整深度检测结果', () {
      final j = {
        'available': true,
        'response_time': 234567890,
        'format': {'name': 'mpegts', 'bit_rate': 2048000},
        'video_streams': [
          {'codec': 'h264', 'width': 1920, 'height': 1080, 'frame_rate': 25.0},
        ],
        'audio_streams': [
          {'codec': 'aac', 'sample_rate': 48000, 'channels': 2},
        ],
      };
      final s = StreamInfo.fromJson(j);
      expect(s.available, isTrue);
      // Go time.Duration 序列化为纳秒
      expect(s.responseTimeMs, 234);
      expect(s.resolution, '1920×1080');
      expect(s.videoStreams.first.codec, 'h264');
      expect(s.audioStreams.first.sampleRate, 48000);
    });

    test('解析失败结果', () {
      final s = StreamInfo.fromJson({
        'available': false,
        'response_time': 0,
        'error': 'connection refused',
      });
      expect(s.available, isFalse);
      expect(s.error, 'connection refused');
      expect(s.resolution, isEmpty);
    });
  });

  group('ResultsPayload', () {
    test('解析 /api/results 载荷', () {
      final p = ResultsPayload.fromJson({
        'results': [
          {
            'channel': {
              'name': 'CCTV-1',
              'url': 'http://example.com/1',
              'group_title': '央视',
              'logo': '',
              'tvg_id': '',
              'tvg_name': '',
            },
            'stream_info': {'available': true, 'response_time': 120000000},
            'scanned_at': '2026-11-21T10:00:00Z',
          },
          {
            'channel': {'name': 'B', 'url': 'http://example.com/2'},
            'stream_info': null,
          },
        ],
        'progress': {'total': 2, 'completed': 1, 'available': 1, 'failed': 0},
        'scanning': true,
        'lastScanTime': null,
      });
      expect(p.results.length, 2);
      expect(p.results.first.channel.name, 'CCTV-1');
      expect(p.results.first.channel.groupTitle, '央视');
      expect(p.results.first.available, isTrue);
      expect(p.results.last.available, isFalse);
      expect(p.progress.fraction, closeTo(0.5, 1e-9));
      expect(p.scanning, isTrue);
    });
  });

  group('Settings', () {
    test('往返序列化', () {
      final s = Settings(
          concurrency: 30, timeout: 8, quickCheck: true, userAgent: 'UA/1.0');
      final restored = Settings.fromJson(s.toJson());
      expect(restored.concurrency, 30);
      expect(restored.timeout, 8);
      expect(restored.quickCheck, isTrue);
      expect(restored.userAgent, 'UA/1.0');
    });
  });
}
