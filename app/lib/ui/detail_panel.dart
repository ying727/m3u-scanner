import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';

class DetailPanel extends StatefulWidget {
  const DetailPanel({super.key});

  @override
  State<DetailPanel> createState() => _DetailPanelState();
}

class _DetailPanelState extends State<DetailPanel> {
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

  void _play(String url) {
    _playingUrl = url;
    _player.open(Media(url));
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final result = state.selected;

    if (result == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.tv_outlined,
                size: 40, color: Theme.of(context).colorScheme.outline),
            const SizedBox(height: 8),
            const Text('点击左侧频道查看详情并播放'),
          ],
        ),
      );
    }

    final channel = result.channel;
    final info = result.streamInfo;
    // 走后端 /api/stream 代理播放,复用 UA/Referer/HLS 修复逻辑
    final proxyUrl = state.api.streamUrl(channel.url);

    return Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // 标题 + 操作
          Row(
            children: [
              Expanded(
                child: Text(
                  channel.name.isEmpty ? channel.url : channel.name,
                  style: Theme.of(context).textTheme.titleMedium,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                tooltip: '复制频道地址',
                icon: const Icon(Icons.copy, size: 18),
                onPressed: () =>
                    Clipboard.setData(ClipboardData(text: channel.url)),
              ),
            ],
          ),
          const SizedBox(height: 8),
          // 播放器
          Expanded(
            child: Container(
              color: Colors.black,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: Video(
                  controller: _controller,
                  controls: NoVideoControls,
                ),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              FilledButton.tonalIcon(
                icon: Icon(_playingUrl == proxyUrl ? Icons.stop : Icons.play_arrow,
                    size: 18),
                label: Text(_playingUrl == proxyUrl ? '停止' : '播放'),
                onPressed: () {
                  if (_playingUrl == proxyUrl) {
                    _playingUrl = null;
                    _player.stop();
                    setState(() {});
                  } else {
                    _play(proxyUrl);
                    setState(() {});
                  }
                },
              ),
              const SizedBox(width: 8),
              StreamBuilder<bool>(
                stream: _player.stream.playing,
                initialData: false,
                builder: (ctx, snap) => snap.data == true
                    ? OutlinedButton.icon(
                        icon: const Icon(Icons.pause, size: 18),
                        label: const Text('暂停/继续'),
                        onPressed: () => _player.playOrPause(),
                      )
                    : const SizedBox.shrink(),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _StreamInfo(info: info),
        ],
      ),
    );
  }
}

class _StreamInfo extends StatelessWidget {
  final StreamInfo? info;
  const _StreamInfo({required this.info});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (info == null) {
      return Text('尚未检测该频道', style: theme.textTheme.bodySmall);
    }
    final rows = <(String, String)>[
      ('状态', info!.available ? '可用' : '失败'),
      if (info!.responseTimeMs > 0) ('响应', '${info!.responseTimeMs} ms'),
      if (info!.resolution.isNotEmpty) ('分辨率', info!.resolution),
      if (info!.videoStreams.isNotEmpty) ('视频编码', info!.videoStreams.first.codec),
      if (info!.videoStreams.isNotEmpty && info!.videoStreams.first.frameRate > 0)
        ('帧率', info!.videoStreams.first.frameRate.toStringAsFixed(1)),
      if (info!.audioStreams.isNotEmpty) ('音频编码', info!.audioStreams.first.codec),
      if (info!.audioStreams.isNotEmpty && info!.audioStreams.first.sampleRate > 0)
        ('采样率', '${info!.audioStreams.first.sampleRate} Hz'),
      if (info!.error != null) ('错误', info!.error!),
    ];
    return DefaultTextStyle(
      style: theme.textTheme.bodySmall!,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final (k, v) in rows)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 1),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                      width: 64,
                      child: Text(k,
                          style: TextStyle(color: theme.colorScheme.outline))),
                  Expanded(child: SelectableText(v)),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
