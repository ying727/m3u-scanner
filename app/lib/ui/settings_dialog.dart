import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models.dart';
import '../state.dart';
import '../theme.dart';

/// 扫描设置。对应后端 Settings{concurrency,timeout,quickCheck,userAgent}。
Future<void> showSettingsDialog(BuildContext context) async {
  final state = context.read<AppState>();
  final s = state.settings;

  final concurrency = TextEditingController(text: '${s.concurrency}');
  final timeout = TextEditingController(text: '${s.timeout}');
  final userAgent = TextEditingController(text: s.userAgent);
  var quickCheck = s.quickCheck;

  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => StatefulBuilder(
      builder: (ctx, setState) => AlertDialog(
        title: const Text('扫描设置'),
        content: SizedBox(
          width: 420,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: _NumberField(
                      controller: concurrency,
                      label: '并发数',
                      hint: '1-100',
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _NumberField(
                      controller: timeout,
                      label: '超时(秒)',
                      hint: '5-60',
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text(
                  '快速检测(仅 HTTP 探活)',
                  style: TextStyle(fontSize: 13.5, fontWeight: FontWeight.w600),
                ),
                subtitle: Text(
                  '关闭后使用 ffprobe 深度检测,可获取分辨率/编码信息',
                  style: TextStyle(fontSize: 11.5, color: context.muted),
                ),
                value: quickCheck,
                onChanged: (v) => setState(() => quickCheck = v),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: userAgent,
                decoration: const InputDecoration(
                  labelText: '自定义 User-Agent(可选)',
                  hintText: '留空使用默认 UA',
                ),
              ),
              const SizedBox(height: 14),
              Notice(
                (state.ffprobeAvailable ?? false)
                    ? 'ffprobe 已就绪,深度检测可用。'
                    : '未检测到 ffprobe.exe,深度检测不可用,将只能进行快速探活。',
                error: !(state.ffprobeAvailable ?? false),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('取消')),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: pine,
              foregroundColor: leaf,
            ),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('保存'),
          ),
        ],
      ),
    ),
  );

  if (ok != true || !context.mounted) return;

  final newSettings = Settings(
    concurrency: int.tryParse(concurrency.text.trim()) ?? s.concurrency,
    timeout: int.tryParse(timeout.text.trim()) ?? s.timeout,
    quickCheck: quickCheck,
    userAgent: userAgent.text.trim(),
  );
  try {
    await state.saveSettings(newSettings);
    if (context.mounted) showMessage(context, '设置已保存');
  } catch (e) {
    if (context.mounted) showMessage(context, e.toString());
  }
}

class _NumberField extends StatelessWidget {
  final TextEditingController controller;
  final String label;
  final String hint;

  const _NumberField({
    required this.controller,
    required this.label,
    required this.hint,
  });

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      keyboardType: TextInputType.number,
      decoration: InputDecoration(labelText: label, hintText: hint),
    );
  }
}
