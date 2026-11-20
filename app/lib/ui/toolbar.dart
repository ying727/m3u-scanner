import 'dart:io';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';
import 'dialogs.dart';
import 'settings_dialog.dart';

class Toolbar extends StatelessWidget {
  const Toolbar({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final scanning = state.payload.scanning;
    final progress = state.payload.progress;
    final ffprobe = state.backendStatus?.ffprobeAvailable ?? false;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Row(
        children: [
          _IconBtn(
            tooltip: '打开本地 m3u 文件',
            icon: Icons.folder_open,
            onTap: () => _pickAndUpload(context),
          ),
          _IconBtn(
            tooltip: '从 URL 加载源',
            icon: Icons.public,
            onTap: () => showLoadUrlDialog(context),
          ),
          const SizedBox(width: 8),
          if (scanning)
            FilledButton.tonalIcon(
              icon: const Icon(Icons.stop, size: 18),
              label: const Text('停止扫描'),
              onPressed: () => _run(context, () => state.stopScan()),
            )
          else
            FilledButton.icon(
              icon: const Icon(Icons.play_arrow, size: 18),
              label: const Text('开始扫描'),
              onPressed: state.payload.results.isEmpty
                  ? null
                  : () => _run(context, () => state.startScan()),
            ),
          const SizedBox(width: 8),
          OutlinedButton.icon(
            icon: const Icon(Icons.refresh, size: 18),
            label: const Text('重扫失败'),
            onPressed: scanning
                ? null
                : () => _run(context, () => state.rescanFailed()),
          ),
          const SizedBox(width: 16),
          Expanded(child: _ScanProgress(scanning: scanning, progress: progress)),
          const SizedBox(width: 16),
          PopupMenuButton<String>(
            tooltip: '导出结果',
            icon: const Icon(Icons.file_download),
            onSelected: (kind) => _export(context, kind),
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'm3u', child: Text('导出可用 m3u')),
              PopupMenuItem(value: 'json', child: Text('导出 JSON')),
              PopupMenuItem(value: 'csv', child: Text('导出 CSV')),
            ],
          ),
          _IconBtn(
            tooltip: '设置',
            icon: Icons.settings_outlined,
            onTap: () => showSettingsDialog(context),
          ),
          _IconBtn(
            tooltip: '清空结果',
            icon: Icons.delete_sweep_outlined,
            onTap: () => _confirmClear(context),
          ),
          const SizedBox(width: 8),
          _StatusChip(
            label: ffprobe ? 'ffprobe' : 'ffprobe 缺失',
            ok: ffprobe,
          ),
        ],
      ),
    );
  }

  Future<void> _pickAndUpload(BuildContext context) async {
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
    } catch (e) {
      if (context.mounted) _showError(context, e.toString());
    }
  }

  Future<void> _export(BuildContext context, String kind) async {
    final state = context.read<AppState>();
    try {
      final ext = kind == 'csv' ? 'csv' : kind == 'json' ? 'json' : 'm3u';
      final suggested = 'available_channels.$ext';
      final location = await getSaveLocation(suggestedName: suggested);
      if (location == null) return;
      final bytes = await state.api.export(kind);
      await File(location.path).writeAsBytes(bytes);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('已导出到 ${location.path}')),
        );
      }
    } catch (e) {
      if (context.mounted) _showError(context, e.toString());
    }
  }

  Future<void> _confirmClear(BuildContext context) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('清空结果'),
        content: const Text('将删除当前所有扫描结果,确定继续?'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('确定')),
        ],
      ),
    );
    if (ok != true || !context.mounted) return;
    await _run(context, () => context.read<AppState>().clearResults());
  }

  Future<void> _run(BuildContext context, Future<void> Function() fn) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await fn();
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(content: Text(e.toString()), behavior: SnackBarBehavior.floating),
      );
    }
  }

  void _showError(BuildContext context, String message) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), behavior: SnackBarBehavior.floating),
    );
  }
}

class _ScanProgress extends StatelessWidget {
  final bool scanning;
  final ScanProgress progress;

  const _ScanProgress({required this.scanning, required this.progress});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (!scanning && progress.total == 0) {
      return Text('尚未扫描', style: theme.textTheme.bodySmall);
    }
    final text = scanning
        ? '${progress.completed}/${progress.total}  ·  可用 ${progress.available}  ·  失败 ${progress.failed}'
        : '共 ${progress.total}  ·  可用 ${progress.available}  ·  失败 ${progress.failed}';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(text, style: theme.textTheme.bodySmall),
        const SizedBox(height: 4),
        ClipRRect(
          borderRadius: BorderRadius.circular(3),
          child: LinearProgressIndicator(
            value: scanning ? progress.fraction : 1,
            minHeight: 4,
          ),
        ),
      ],
    );
  }
}

class _IconBtn extends StatelessWidget {
  final String tooltip;
  final IconData icon;
  final VoidCallback onTap;

  const _IconBtn(
      {required this.tooltip, required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return IconButton(tooltip: tooltip, icon: Icon(icon), onPressed: onTap);
  }
}

class _StatusChip extends StatelessWidget {
  final String label;
  final bool ok;

  const _StatusChip({required this.label, required this.ok});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final color = ok ? Colors.greenAccent : Colors.orangeAccent;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(ok ? Icons.check_circle : Icons.warning_amber_rounded,
              size: 12, color: color),
          const SizedBox(width: 4),
          Text(label, style: TextStyle(fontSize: 11, color: scheme.onSurface)),
        ],
      ),
    );
  }
}
