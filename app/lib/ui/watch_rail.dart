import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';
import '../theme.dart';

/// 右侧观看栏:播放器卡片 + 流信息。
class WatchRail extends StatefulWidget {
  const WatchRail({super.key});

  @override
  State<WatchRail> createState() => _WatchRailState();
}

class _WatchRailState extends State<WatchRail> {
  late final Player _player;
  late final VideoController _controller;
  String? _playingUrl;

  @override
  void initState() {
    super.initState();
    _player = Player();
    _controller = VideoController(_player);
  }

  @override
  void dispose() {
    _player.dispose();
    super.dispose();
  }

  /// 直连播放:由 mpv 自己带 UA/Referer,不再经过本地 HTTP 代理。
  void _play(String url) {
    _playingUrl = url;
    final ua = context.read<AppState>().settings.userAgent;
    _player.open(Media(
      url,
      httpHeaders: {
        'User-Agent': ua.isEmpty
            ? 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 '
                '(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'
            : ua,
        // 部分 CDN 校验 Referer;用流自身地址兜底
        'Referer': url,
      },
    ));
  }

  void _stop() {
    _playingUrl = null;
    _player.stop();
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final result = state.selected;
    final railColor = context.panel;

    return Container(
      width: 340,
      color: Theme.of(context).scaffoldBackgroundColor,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(0, 12, 16, 16),
        child: SurfaceCard(
          padding: const EdgeInsets.all(18),
          color: railColor,
          child: result == null
              ? _buildEmptyWatch(context)
              : _Watching(
                  context: context,
                  result: result,
                  player: _player,
                  controller: _controller,
                  playingUrl: _playingUrl,
                  onPlay: () {
                    _play(result.channel.url);
                    setState(() {});
                  },
                  onStop: _stop,
                ),
        ),
      ),
    );
  }

  Widget _buildEmptyWatch(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Eyebrow('NOW WATCHING'),
        Expanded(
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.tv_outlined, size: 36, color: context.muted),
                const SizedBox(height: 10),
                Text(
                  '点击左侧频道开始观看',
                  style: TextStyle(fontSize: 12.5, color: context.muted),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _Watching extends StatelessWidget {
  final BuildContext context;
  final ScanResult result;
  final Player player;
  final VideoController controller;
  final String? playingUrl;
  final VoidCallback onPlay;
  final VoidCallback onStop;

  const _Watching({
    required this.context,
    required this.result,
    required this.player,
    required this.controller,
    required this.playingUrl,
    required this.onPlay,
    required this.onStop,
  });

  @override
  Widget build(BuildContext context) {
    final ch = result.channel;
    final info = result.streamInfo;
    final isPlaying = playingUrl == ch.url;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Eyebrow('NOW WATCHING'),
            const Spacer(),
            IconButton(
              tooltip: '复制频道地址',
              icon: Icon(Icons.copy_rounded, size: 16, color: context.muted),
              onPressed: () {
                Clipboard.setData(ClipboardData(text: ch.url));
                showMessage(context, '频道地址已复制');
              },
            ),
          ],
        ),
        const SizedBox(height: 6),
        Text(
          ch.name.isEmpty ? ch.url : ch.name,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700, height: 1.3),
        ),
        if (ch.groupTitle.trim().isNotEmpty) ...[
          const SizedBox(height: 4),
          Text(ch.groupTitle.trim(),
              style: TextStyle(fontSize: 11.5, color: context.muted)),
        ],
        const SizedBox(height: 12),
        // 播放器
        Expanded(
          child: Container(
            color: const Color(0xFF0B120E),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(16),
              child: Video(controller: controller, controls: NoVideoControls),
            ),
          ),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            FilledButton.icon(
              style: FilledButton.styleFrom(
                backgroundColor: pine,
                foregroundColor: leaf,
              ),
              onPressed: isPlaying ? onStop : onPlay,
              icon: Icon(
                isPlaying ? Icons.stop_rounded : Icons.play_arrow_rounded,
                size: 18,
                color: leaf,
              ),
              label: Text(isPlaying ? '停止' : '播放'),
            ),
            if (isPlaying)
              StreamBuilder<bool>(
                stream: player.stream.playing,
                initialData: false,
                builder: (ctx, snap) => OutlinedButton.icon(
                  onPressed: () => player.playOrPause(),
                  icon: Icon(
                    snap.data == true
                        ? Icons.pause_rounded
                        : Icons.play_arrow_rounded,
                    size: 16,
                  ),
                  label: Text(snap.data == true ? '暂停' : '继续'),
                ),
              ),
          ],
        ),
        const SizedBox(height: 14),
        Divider(color: context.line, height: 1),
        const SizedBox(height: 10),
        _StreamInfo(info: info),
      ],
    );
  }
}

class _StreamInfo extends StatelessWidget {
  final StreamInfo? info;
  const _StreamInfo({required this.info});

  @override
  Widget build(BuildContext context) {
    if (info == null) {
      return Text('该频道尚未检测 · 点击「开始扫描」获取结果',
          style: TextStyle(fontSize: 11.5, color: context.muted));
    }
    final rows = <(String, String)>[
      ('状态', info!.available ? '可用' : '失败'),
      if (info!.responseTimeMs > 0) ('响应', '${info!.responseTimeMs} ms'),
      if (info!.resolution.isNotEmpty) ('分辨率', info!.resolution),
      if (info!.videoStreams.isNotEmpty)
        ('视频', info!.videoStreams.first.codec),
      if (info!.videoStreams.isNotEmpty && info!.videoStreams.first.frameRate > 0)
        ('帧率', info!.videoStreams.first.frameRate.toStringAsFixed(1)),
      if (info!.audioStreams.isNotEmpty)
        ('音频', info!.audioStreams.first.codec),
      if (info!.error != null) ('错误', info!.error!),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final (k, v) in rows)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 2),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 52,
                  child: Text(k,
                      style: TextStyle(
                          fontSize: 11.5, color: context.muted)),
                ),
                Expanded(
                  child: SelectableText(
                    v,
                    style: const TextStyle(fontSize: 11.5, height: 1.4),
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}
