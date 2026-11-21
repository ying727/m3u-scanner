import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../state.dart';
import '../theme.dart';

/// 打开本地 m3u 文件并上传为当前源。
Future<void> openLocalSource(BuildContext context) async {
  final state = context.read<AppState>();
  try {
    final file = await openFile(
      acceptedTypeGroups: [
        XTypeGroup(label: 'M3U', extensions: ['m3u', 'm3u8', 'txt']),
      ],
    );
    if (file == null) return;
    final bytes = await file.readAsBytes();
    await state.uploadM3U(bytes, file.name);
    if (context.mounted) showMessage(context, '已加载 ${file.name}');
  } catch (e) {
    if (context.mounted) showMessage(context, e.toString());
  }
}

/// 从 URL 加载播放列表源。
Future<void> showLoadUrlDialog(BuildContext context) async {
  final controller = TextEditingController();
  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('从 URL 加载源'),
      content: TextField(
        controller: controller,
        autofocus: true,
        decoration: const InputDecoration(
          hintText: 'https://example.com/playlist.m3u',
        ),
        onSubmitted: (_) => Navigator.pop(ctx, true),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消')),
        FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('加载')),
      ],
    ),
  );
  final url = controller.text.trim();
  if (ok != true || url.isEmpty || !context.mounted) return;

  try {
    await context.read<AppState>().loadUrl(url);
    if (context.mounted) showMessage(context, '源已加载');
  } catch (e) {
    if (context.mounted) showMessage(context, e.toString());
  }
}
