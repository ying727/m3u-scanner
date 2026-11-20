import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../state.dart';

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
            onPressed: () => Navigator.pop(ctx, true), child: const Text('加载')),
      ],
    ),
  );
  final url = controller.text.trim();
  if (ok != true || url.isEmpty || !context.mounted) return;

  try {
    await context.read<AppState>().loadUrl(url);
    if (context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('源已加载')));
    }
  } catch (e) {
    if (context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }
}
