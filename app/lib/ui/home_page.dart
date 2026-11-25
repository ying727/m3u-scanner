import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:window_manager/window_manager.dart';

import '../state.dart';
import '../theme.dart';
import 'channel_view.dart';
import 'sidebar.dart';
import 'watch_rail.dart';

/// 主界面:侧栏 + 主内容(扫描控制台/频道网格) + 观看栏。
/// 同时负责窗口关闭时取消扫描并释放原生资源。
class HomePage extends StatefulWidget {
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> with WindowListener {
  @override
  void initState() {
    super.initState();
    windowManager.addListener(this);
    windowManager.setPreventClose(true);
    context.read<AppState>().init();
  }

  @override
  void dispose() {
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  void onWindowClose() async {
    await context.read<AppState>().shutdown();
    await windowManager.destroy();
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    switch (state.bootState) {
      case BootState.booting:
        return const _BootingView();
      case BootState.error:
        return _ErrorView(message: state.bootError ?? '未知错误');
      case BootState.ready:
        return Scaffold(
          body: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Sidebar(),
              VerticalDivider(width: 1, color: context.line),
              Expanded(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Expanded(child: ChannelView()),
                    VerticalDivider(width: 1, color: context.line),
                    const WatchRail(),
                  ],
                ),
              ),
            ],
          ),
        );
    }
  }
}

class _BootingView extends StatelessWidget {
  const _BootingView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: const [
            Brand(large: true),
            SizedBox(height: 28),
            SizedBox(
              width: 22,
              height: 22,
              child: CircularProgressIndicator(strokeWidth: 2.4),
            ),
            SizedBox(height: 16),
            Text('正在启动后端服务…'),
          ],
        ),
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  final String message;
  const _ErrorView({required this.message});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: SizedBox(
          width: 520,
          child: SurfaceCard(
            padding: const EdgeInsets.all(28),
            child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Brand(large: true),
              const SizedBox(height: 20),
              Notice(
                message,
                error: true,
              ),
              const SizedBox(height: 20),
              FilledButton.icon(
                style: FilledButton.styleFrom(
                  backgroundColor: pine,
                  foregroundColor: leaf,
                ),
                icon: const Icon(Icons.refresh_rounded,
                    size: 18, color: leaf),
                label: const Text('重试'),
                onPressed: () {
                  final state = context.read<AppState>();
                  state.resetBoot();
                  state.init();
                },
              ),
            ],
            ),
          ),
        ),
      ),
    );
  }
}
