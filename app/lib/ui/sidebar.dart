import 'dart:io';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../state.dart';
import '../theme.dart';
import 'dialogs.dart';
import 'settings_dialog.dart';

/// 左侧栏:品牌、源操作、底部状态卡与版本脚注。
class Sidebar extends StatelessWidget {
  const Sidebar({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final ffprobe = state.backendStatus?.ffprobeAvailable ?? false;

    return SizedBox(
      width: 232,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(left: 4, top: 4),
              child: const Brand(),
            ),
            const SizedBox(height: 28),
            _NavLabel('源'),
            _NavItem(
              icon: Icons.folder_open_outlined,
              label: '打开本地源',
              onTap: () => _pickAndUpload(context),
            ),
            _NavItem(
              icon: Icons.public_outlined,
              label: '从 URL 加载',
              onTap: () => showLoadUrlDialog(context),
            ),
            _NavItem(
              icon: Icons.file_download_outlined,
              label: '导出结果',
              trailing: Icons.keyboard_arrow_down_rounded,
              onTap: (state.payload.results.isEmpty)
                  ? null
                  : () => _showExportMenu(context),
            ),
            const SizedBox(height: 20),
            _NavLabel('更多'),
            _NavItem(
              icon: Icons.settings_outlined,
              label: '扫描设置',
              onTap: () => showSettingsDialog(context),
            ),
            const Spacer(),
            _SideQuoteCard(
              title: ffprobe ? '深度检测就绪' : '快速检测模式',
              lines: [
                '共 ${state.payload.results.length} 个频道',
                '可用 ${state.availableCount}',
              ],
            ),
            const SizedBox(height: 12),
            Padding(
              padding: const EdgeInsets.only(left: 4),
              child: Text(
                'PORTABLE · v1.0',
                style: TextStyle(
                  fontSize: 9,
                  letterSpacing: 1.2,
                  color: context.muted,
                ),
              ),
            ),
          ],
        ),
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
      if (context.mounted) showMessage(context, e.toString());
    }
  }

  void _showExportMenu(BuildContext context) {
    final RenderBox box = context.findRenderObject()! as RenderBox;
    final state = context.read<AppState>();
    showMenu<String>(
      context: context,
      position: RelativeRect.fromLTRB(
        box.localToGlobal(Offset.zero).dx + 8,
        box.localToGlobal(Offset.zero).dy + 240,
        8,
        0,
      ),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      color: context.panel,
      items: const [
        PopupMenuItem(value: 'm3u', child: Text('导出可用 m3u')),
        PopupMenuItem(value: 'json', child: Text('导出 JSON')),
        PopupMenuItem(value: 'csv', child: Text('导出 CSV')),
      ],
    ).then((kind) async {
      if (kind == null) return;
      try {
        final ext = kind == 'csv' ? 'csv' : kind == 'json' ? 'json' : 'm3u';
        final location = await getSaveLocation(suggestedName: 'available_channels.$ext');
        if (location == null) return;
        final bytes = await state.api.export(kind);
        await File(location.path).writeAsBytes(bytes);
      } catch (e) {
        if (context.mounted) showMessage(context, e.toString());
      }
    });
  }
}

class _NavLabel extends StatelessWidget {
  final String text;
  const _NavLabel(this.text);

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(left: 14, bottom: 8),
        child: Text(
          text,
          style: TextStyle(
            fontSize: 10,
            fontWeight: FontWeight.w600,
            letterSpacing: 1.6,
            color: context.muted,
          ),
        ),
      );
}

class _NavItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final IconData? trailing;
  final VoidCallback? onTap;

  const _NavItem({
    required this.icon,
    required this.label,
    this.trailing,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null;
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(14),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
            child: Row(
              children: [
                Icon(icon,
                    size: 19,
                    color: enabled ? context.ink : context.muted),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      fontSize: 13.5,
                      fontWeight: FontWeight.w600,
                      color: enabled ? context.ink : context.muted,
                    ),
                  ),
                ),
                ?(trailing == null
                    ? null
                    : Icon(trailing, size: 18, color: context.muted)),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 底部引言卡:统计摘要 + 一句副题。
class _SideQuoteCard extends StatelessWidget {
  final String title;
  final List<String> lines;

  const _SideQuoteCard({required this.title, required this.lines});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: context.tint,
        borderRadius: BorderRadius.circular(18),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.radar_rounded, size: 18, color: context.ok),
          const SizedBox(height: 10),
          Text(
            title,
            style: const TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 4),
          Text(
            lines.join('  ·  '),
            style: TextStyle(fontSize: 11.5, height: 1.5, color: context.muted),
          ),
        ],
      ),
    );
  }
}

/// 供首页顶栏与其它区域复用的源胶囊。
class SourcePill extends StatelessWidget {
  final String label;
  final VoidCallback? onTap;

  const SourcePill({super.key, required this.label, this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      shape: StadiumBorder(side: BorderSide(color: context.line)),
      child: InkWell(
        onTap: onTap,
        customBorder: const StadiumBorder(),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.source_outlined, size: 16, color: context.muted),
              const SizedBox(width: 8),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 360),
                child: Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(width: 6),
              Icon(Icons.keyboard_arrow_down_rounded,
                  size: 16, color: context.muted),
            ],
          ),
        ),
      ),
    );
  }
}

/// 顶栏右侧的 ffprobe 状态徽章。
class FFprobeBadge extends StatelessWidget {
  final bool ready;
  const FFprobeBadge({super.key, required this.ready});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: context.tint,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            ready ? Icons.check_circle_rounded : Icons.warning_amber_rounded,
            size: 13,
            color: ready ? context.ok : Colors.orangeAccent,
          ),
          const SizedBox(width: 6),
          Text(
            ready ? 'ffprobe' : 'ffprobe 缺失',
            style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w600),
          ),
        ],
      ),
    );
  }
}
