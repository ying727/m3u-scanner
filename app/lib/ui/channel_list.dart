import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';

class ChannelList extends StatelessWidget {
  const ChannelList({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final results = state.filteredResults;

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 8),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  decoration: const InputDecoration(
                    prefixIcon: Icon(Icons.search, size: 18),
                    hintText: '搜索频道名',
                  ),
                  onChanged: state.setSearch,
                ),
              ),
              const SizedBox(width: 8),
              DropdownMenu<String>(
                width: 180,
                initialSelection: state.groupFilter ?? '全部',
                requestFocusOnTap: false,
                dropdownMenuEntries: [
                  const DropdownMenuEntry(value: '全部', label: '全部分组'),
                  ...state.groups.map(
                      (g) => DropdownMenuEntry(value: g, label: g)),
                ],
                onSelected: state.setGroup,
              ),
              const SizedBox(width: 8),
              InkWell(
                onTap: () => state.setOnlyAvailable(!state.onlyAvailable),
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 6),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Checkbox(
                        value: state.onlyAvailable,
                        onChanged: (v) => state.setOnlyAvailable(v ?? false),
                      ),
                      const Text('仅可用'),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Align(
            alignment: Alignment.centerLeft,
            child: Text(
              '${results.length} 个频道  ·  可用 ${state.availableCount}',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
        ),
        const SizedBox(height: 4),
        Expanded(
          child: results.isEmpty
              ? _EmptyResults(state: state)
              : ListView.builder(
                  itemCount: results.length,
                  itemBuilder: (ctx, i) =>
                      _ChannelTile(result: results[i]),
                ),
        ),
      ],
    );
  }
}

class _EmptyResults extends StatelessWidget {
  final AppState state;
  const _EmptyResults({required this.state});

  @override
  Widget build(BuildContext context) {
    final hasSource = state.payload.results.isNotEmpty;
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.playlist_add,
              size: 40, color: Theme.of(context).colorScheme.outline),
          const SizedBox(height: 8),
          Text(
            hasSource ? '没有匹配过滤条件的频道' : '先加载 m3u 文件或 URL,然后开始扫描',
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ],
      ),
    );
  }
}

class _ChannelTile extends StatelessWidget {
  final ScanResult result;
  const _ChannelTile({required this.result});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final scheme = Theme.of(context).colorScheme;
    final selected = state.selected?.channel.url == result.channel.url;
    final info = result.streamInfo;

    final statusColor = info == null
        ? scheme.outline
        : info.available
            ? Colors.greenAccent
            : scheme.error;

    final statusText = info == null
        ? '未检测'
        : info.available
            ? '可用'
            : '失败';

    return ListTile(
      selected: selected,
      dense: true,
      leading: Icon(
        info == null
            ? Icons.circle_outlined
            : info.available
                ? Icons.circle
                : Icons.cancel_outlined,
        size: 16,
        color: statusColor,
      ),
      title: Text(
        result.channel.name.isEmpty ? result.channel.url : result.channel.name,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        [
          if (result.channel.groupTitle.isNotEmpty) result.channel.groupTitle,
          if (info != null && info.available && info.resolution.isNotEmpty)
            info.resolution,
          if (info != null && info.available && info.responseTimeMs > 0)
            '${info.responseTimeMs}ms',
          if (info != null && !info.available && info.error != null)
            info.error!,
        ].join('  ·  '),
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: Theme.of(context).textTheme.bodySmall,
      ),
      trailing: Text(statusText,
          style: TextStyle(fontSize: 12, color: statusColor)),
      onTap: () => state.select(result),
      onLongPress: () => Clipboard.setData(
          ClipboardData(text: result.channel.url)),
    );
  }
}
