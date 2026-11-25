import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';
import '../theme.dart';
import 'dialogs.dart';
import 'sidebar.dart';

/// 主内容区:顶栏、眉题标题、扫描控制台、频道分节网格。
class ChannelView extends StatelessWidget {
  const ChannelView({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final scanning = state.payload.scanning;
    final progress = state.payload.progress;

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 顶栏:面包屑 + ffprobe 徽章
          Row(
            children: [
              Text(
                '频道库 / ${state.groupFilter ?? '全部'}',
                style: TextStyle(fontSize: 12.5, color: context.muted),
              ),
              const Spacer(),
              const SizedBox(width: 12),
              FFprobeBadge(
                ready: state.ffprobeAvailable ?? false,
              ),
            ],
          ),
          const SizedBox(height: 22),
          // 眉题 + 大标题 + 源胶囊
          const Eyebrow('YOUR CHANNELS, VERIFIED.'),
          const SizedBox(height: 8),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              const Expanded(
                child: Text(
                  '扫一遍,只留能看的。',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 30,
                    height: 1.2,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1.5,
                  ),
                ),
              ),
              const SizedBox(width: 16),
              SourcePill(
                label: state.sourceLabel ?? '未选择源',
                onTap: () => _showSourceMenu(context),
              ),
            ],
          ),
          const SizedBox(height: 18),
          // 扫描控制台 hero 卡(浅叶色)
          _ScanConsole(scanning: scanning, progress: progress),
          const SizedBox(height: 14),
          // 状态提示条
          if (!scanning && progress.total == 0)
            Notice('后端服务已就绪 · 加载 m3u 源后即可开始检测;深度检测将给出分辨率与编码信息。')
          else if (scanning)
            Notice('正在扫描 ${progress.completed}/${progress.total},可用 ${progress.available},失败 ${progress.failed}。可随时停止。'),
          const SizedBox(height: 22),
          // 频道分节 + 过滤器
          Expanded(
            child: _ChannelSection(state: state),
          ),
          const SizedBox(height: 20),
        ],
      ),
    );
  }

  void _showSourceMenu(BuildContext context) {
    final RenderBox box = context.findRenderObject()! as RenderBox;
    final origin = box.localToGlobal(Offset.zero);
    showMenu<String>(
      context: context,
      position: RelativeRect.fromLTRB(origin.dx - 160, origin.dy + 40, 16, 0),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      color: context.panel,
      items: const [
        PopupMenuItem(value: 'file', child: Text('打开本地 m3u 文件')),
        PopupMenuItem(value: 'url', child: Text('从 URL 加载源')),
      ],
    ).then((choice) {
      if (choice == null || !context.mounted) return;
      if (choice == 'url') {
        showLoadUrlDialog(context);
      } else {
        openLocalSource(context);
      }
    });
  }
}

/// 扫描控制台:浅叶色大卡,左侧状态与操作,右侧可用率大字。
class _ScanConsole extends StatelessWidget {
  final bool scanning;
  final ScanProgress progress;

  const _ScanConsole({required this.scanning, required this.progress});

  @override
  Widget build(BuildContext context) {
    final total = progress.total;
    final done = total > 0 && progress.completed >= total;
    final percent = total > 0 ? (progress.available / progress.completed * 100) : 0.0;

    return Material(
      color: leaf,
      borderRadius: BorderRadius.circular(24),
      clipBehavior: Clip.antiAlias,
      child: Padding(
        padding: const EdgeInsets.all(22),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Expanded(
              flex: 5,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 标签 chip
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: .55),
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          scanning
                              ? Icons.radar_rounded
                              : done
                                  ? Icons.task_alt_rounded
                                  : Icons.pause_circle_outline_rounded,
                          size: 13,
                          color: pine,
                        ),
                        const SizedBox(width: 6),
                        Text(
                          scanning
                              ? '检测进行中'
                              : done
                                  ? '本轮已完成'
                                  : '检测待开始',
                          style: const TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.w700,
                            color: pine,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    scanning
                        ? '正在逐一探测频道…'
                        : total == 0
                            ? '还没有可检测的频道'
                            : '本轮检测已结束',
                    style: const TextStyle(
                      fontSize: 21,
                      fontWeight: FontWeight.w700,
                      color: Color(0xFF243C32),
                      letterSpacing: 1,
                    ),
                  ),
                  const SizedBox(height: 10),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(6),
                    child: LinearProgressIndicator(
                      value: total == 0 ? 0 : progress.fraction,
                      minHeight: 7,
                      backgroundColor: Colors.white.withValues(alpha: .5),
                      valueColor: const AlwaysStoppedAnimation(pine),
                    ),
                  ),
                  const SizedBox(height: 10),
                  Text(
                    total == 0
                        ? '从左侧加载本地文件或 URL 开始'
                        : '${progress.completed}/$total 已完成  ·  可用 ${progress.available}  ·  失败 ${progress.failed}',
                    style: const TextStyle(
                      fontSize: 12,
                      color: Color(0xFF41573F),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 16),
                  Row(
                    children: [
                      FilledButton.icon(
                        style: FilledButton.styleFrom(
                          backgroundColor: pine,
                          foregroundColor: leaf,
                        ),
                        onPressed: scanning
                            ? null
                            : () => _run(context, () => context.read<AppState>().startScan()),
                        icon: Icon(
                          scanning ? Icons.hourglass_top_rounded : Icons.play_arrow_rounded,
                          size: 18,
                          color: leaf,
                        ),
                        label: Text(scanning ? '扫描中…' : '开始扫描'),
                      ),
                      const SizedBox(width: 10),
                      if (scanning)
                        OutlinedButton.icon(
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: Color(0xFF7A9169)),
                            foregroundColor: const Color(0xFF243C32),
                          ),
                          onPressed: () =>
                              _run(context, () => context.read<AppState>().stopScan()),
                          icon: const Icon(Icons.stop_rounded, size: 18),
                          label: const Text('停止'),
                        )
                      else if (progress.failed > 0)
                        OutlinedButton.icon(
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: Color(0xFF7A9169)),
                            foregroundColor: const Color(0xFF243C32),
                          ),
                          onPressed: () =>
                              _run(context, () => context.read<AppState>().rescanFailed()),
                          icon: const Icon(Icons.refresh_rounded, size: 18),
                          label: Text('重扫失败 ${progress.failed}'),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(width: 24),
            // 右侧大数字:可用率
            Expanded(
              flex: 2,
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Text(
                    total == 0 ? '--' : percent.toStringAsFixed(0),
                    style: const TextStyle(
                      fontSize: 54,
                      height: 1,
                      fontWeight: FontWeight.w700,
                      color: Color(0xFF243C32),
                    ),
                  ),
                  const Padding(
                    padding: EdgeInsets.only(top: 4, right: 4),
                    child: Text(
                      '% 可用',
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: Color(0xFF41573F),
                        letterSpacing: 1,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _run(BuildContext context, Future<void> Function() fn) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await fn();
    } catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }
}

/// 频道分节:标题 + 过滤行 + 卡片网格。
class _ChannelSection extends StatelessWidget {
  final AppState state;
  const _ChannelSection({required this.state});

  @override
  Widget build(BuildContext context) {
    final results = state.filteredResults;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            const Expanded(
              child: SectionTitle(
                no: '01',
                title: '频道',
                subtitle: '选择频道在右侧观看;长按或右键可复制地址。',
              ),
            ),
            SectionTitle(
              no: '',
              title: '',
              count: results.length,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            Expanded(
              flex: 5,
              child: TextField(
                onChanged: state.setSearch,
                style: const TextStyle(fontSize: 13),
                decoration: const InputDecoration(
                  prefixIcon: Icon(Icons.search_rounded, size: 18),
                  hintText: '搜索频道名…',
                ),
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              flex: 3,
              child: DropdownMenu<String>(
                width: 220,
                expandedInsets: EdgeInsets.zero,
                initialSelection: state.groupFilter ?? '全部',
                requestFocusOnTap: false,
                menuStyle: MenuStyle(
                  backgroundColor: WidgetStatePropertyAll(context.panel),
                  shape: WidgetStatePropertyAll(
                    RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
                  ),
                ),
                dropdownMenuEntries: [
                  const DropdownMenuEntry(value: '全部', label: '全部分组'),
                  ...state.groups.map((g) => DropdownMenuEntry(value: g, label: g)),
                ],
                onSelected: state.setGroup,
              ),
            ),
            const SizedBox(width: 10),
            _FilterChip(
              label: '仅可用',
              selected: state.onlyAvailable,
              onTap: () => state.setOnlyAvailable(!state.onlyAvailable),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Expanded(
          child: results.isEmpty
              ? _EmptyResults(hasSource: state.payload.results.isNotEmpty)
              : GridView.builder(
                  gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                    maxCrossAxisExtent: 420,
                    mainAxisExtent: 74,
                    mainAxisSpacing: 10,
                    crossAxisSpacing: 10,
                  ),
                  itemCount: results.length,
                  itemBuilder: (ctx, i) => _ChannelCard(result: results[i]),
                ),
        ),
      ],
    );
  }
}

class _FilterChip extends StatelessWidget {
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _FilterChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: selected ? context.tint : Colors.transparent,
      shape: StadiumBorder(
        side: BorderSide(color: selected ? context.ok : context.line),
      ),
      child: InkWell(
        onTap: onTap,
        customBorder: const StadiumBorder(),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                selected ? Icons.check_circle_rounded : Icons.circle_outlined,
                size: 15,
                color: selected ? context.ok : context.muted,
              ),
              const SizedBox(width: 8),
              Text(
                label,
                style: TextStyle(
                  fontSize: 12.5,
                  fontWeight: FontWeight.w600,
                  color: selected ? context.ok : context.muted,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _EmptyResults extends StatelessWidget {
  final bool hasSource;
  const _EmptyResults({required this.hasSource});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.live_tv_outlined, size: 44, color: context.muted),
          const SizedBox(height: 12),
          Text(
            hasSource ? '没有匹配过滤条件的频道' : '加载 m3u 源后,频道会出现在这里',
            style: TextStyle(fontSize: 13, color: context.muted),
          ),
        ],
      ),
    );
  }
}

class _ChannelCard extends StatelessWidget {
  final ScanResult result;
  const _ChannelCard({required this.result});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final selected = state.selected?.channel.url == result.channel.url;
    final info = result.streamInfo;
    final ch = result.channel;

    final statusColor = info == null
        ? context.muted
        : info.available
            ? context.ok
            : context.bad;
    final statusText = info == null ? '未检测' : info.available ? '可用' : '失败';

    final meta = [
      if (ch.groupTitle.trim().isNotEmpty) ch.groupTitle.trim(),
      if (info != null && info.available && info.resolution.isNotEmpty) info.resolution,
      if (info != null && info.available && info.responseTimeMs > 0)
        '${info.responseTimeMs}ms',
    ].join('  ·  ');

    return Material(
      color: selected ? context.tint : context.panel,
      shape: RoundedRectangleBorder(
        side: BorderSide(
          color: selected ? context.ok : context.line,
          width: selected ? 1.4 : 1,
        ),
        borderRadius: BorderRadius.circular(20),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => state.select(result),
        onLongPress: () => _copyUrl(context, ch.url),
        onSecondaryTap: () => _copyUrl(context, ch.url),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              // 头像圈:首字母
              Container(
                width: 38,
                height: 38,
                decoration: BoxDecoration(
                  color: selected ? pine : context.tint,
                  shape: BoxShape.circle,
                ),
                alignment: Alignment.center,
                child: Text(
                  _initial(ch.name, ch.url),
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: selected ? leaf : context.ok,
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      ch.name.isEmpty ? ch.url : ch.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 13.5,
                        fontWeight: FontWeight.w600,
                        height: 1.3,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      meta.isEmpty ? '—' : meta,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: 11, color: context.muted),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 10),
              // 状态点
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    info == null
                        ? Icons.circle_outlined
                        : info.available
                            ? Icons.circle
                            : Icons.cancel_outlined,
                    size: 13,
                    color: statusColor,
                  ),
                  const SizedBox(width: 5),
                  Text(
                    statusText,
                    style: TextStyle(
                      fontSize: 11.5,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _initial(String name, String url) {
    final s = name.trim().isNotEmpty ? name.trim() : url;
    return s.isEmpty ? '?' : s.characters.first.toUpperCase();
  }

  void _copyUrl(BuildContext context, String url) {
    Clipboard.setData(ClipboardData(text: url));
    showMessage(context, '频道地址已复制');
  }
}
