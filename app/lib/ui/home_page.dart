import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:window_manager/window_manager.dart';

import '../state.dart';
import 'channel_list.dart';
import 'detail_panel.dart';
import 'toolbar.dart';

/// 主界面:顶部工具栏 + 左列表 / 右详情(播放器)。
/// 同时负责窗口关闭时回收 sidecar。
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
          body: Column(
            children: [
              const Toolbar(),
              const Divider(height: 1),
              Expanded(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Expanded(flex: 3, child: ChannelList()),
                    const VerticalDivider(width: 1),
                    const Expanded(flex: 2, child: DetailPanel()),
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
            CircularProgressIndicator(),
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
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(Icons.error_outline,
                  size: 48, color: Theme.of(context).colorScheme.error),
              const SizedBox(height: 16),
              Text('启动失败',
                  style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 8),
              SelectableText(message),
              const SizedBox(height: 24),
              OutlinedButton.icon(
                icon: const Icon(Icons.refresh),
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
    );
  }
}
