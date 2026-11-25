import 'package:flutter/material.dart';

/// 设计令牌与共享组件,设计语言来自 tyg(gate_flutter):
/// 松绿 pine / 浅鼠尾草 leaf 双色系,24px 圆角卡,16px 胶囊,NotoSansSC。
const pine = Color(0xFF24594B);
const leaf = Color(0xFFDCEACB);

ThemeData appTheme() {
  final scheme = ColorScheme.fromSeed(
    seedColor: pine,
    brightness: Brightness.dark,
    primary: const Color(0xFFACD2BD),
    surface: const Color(0xFF182720),
  );
  final base = ThemeData(useMaterial3: true, colorScheme: scheme);
  return base.copyWith(
    scaffoldBackgroundColor: const Color(0xFF111C17),
    textTheme: base.textTheme.apply(
      fontFamily: 'NotoSansSC',
      bodyColor: const Color(0xFFE8EEE7),
      displayColor: const Color(0xFFE8EEE7),
    ),
    dividerColor: const Color(0xFF314239),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: const Color(0xFF111E18),
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: BorderSide.none,
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: const BorderSide(color: Color(0xFF314239)),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: BorderSide(color: scheme.primary, width: 1.5),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size(48, 46),
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        textStyle: const TextStyle(
          fontFamily: 'NotoSansSC',
          fontSize: 14,
          fontWeight: FontWeight.w600,
        ),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(48, 44),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        side: const BorderSide(color: Color(0xFF3E5447)),
      ),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: const Color(0xFF182720),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
    ),
    snackBarTheme: const SnackBarThemeData(
      behavior: SnackBarBehavior.floating,
      backgroundColor: leaf,
      contentTextStyle: TextStyle(
          fontFamily: 'NotoSansSC', color: pine, fontWeight: FontWeight.w600),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(16)),
      ),
    ),
    tooltipTheme: const TooltipThemeData(waitDuration: Duration(milliseconds: 450)),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected) ? pine : null,
      ),
      trackColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected) ? leaf : null,
      ),
    ),
  );
}

extension AppColors on BuildContext {
  bool get isDark => Theme.of(this).brightness == Brightness.dark;
  Color get muted => const Color(0xFFA2B6A8);
  Color get line => Theme.of(this).dividerColor;
  Color get panel => Theme.of(this).colorScheme.surface;
  Color get ink => const Color(0xFFE8EEE7);
  Color get tint => const Color(0xFF263D2E);
  Color get ok => const Color(0xFF9CCDAF);
  Color get bad => const Color(0xFFDFA9A0);
}

/// 品牌标:图标块 + 字标 + 小字副题。
class Brand extends StatelessWidget {
  const Brand({super.key, this.large = false});
  final bool large;

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: large ? 54 : 42,
            height: large ? 54 : 42,
            decoration: BoxDecoration(
              color: pine,
              borderRadius: BorderRadius.circular(large ? 18 : 14),
            ),
            child: Icon(Icons.smart_display_outlined, color: leaf, size: large ? 28 : 22),
          ),
          const SizedBox(width: 12),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'M3U Scanner',
                style: TextStyle(
                  fontSize: large ? 22 : 17,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 1.5,
                ),
              ),
              Text(
                'SCAN · IPTV 频道检测',
                style: TextStyle(
                  fontSize: 9,
                  letterSpacing: 1.1,
                  color: context.muted,
                ),
              ),
            ],
          ),
        ],
      );
}

/// 主内容面板:细描边 + 24px 圆角。
class SurfaceCard extends StatelessWidget {
  const SurfaceCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(24),
    this.color,
  });

  final Widget child;
  final EdgeInsetsGeometry padding;
  final Color? color;

  @override
  Widget build(BuildContext context) => Material(
        color: color ?? context.panel,
        shape: RoundedRectangleBorder(
          side: BorderSide(color: context.line),
          borderRadius: BorderRadius.circular(24),
        ),
        clipBehavior: Clip.antiAlias,
        child: Padding(padding: padding, child: child),
      );
}

/// 英文眉题:小号大间距,编辑感排版。
class Eyebrow extends StatelessWidget {
  const Eyebrow(this.text, {super.key, this.color});
  final String text;
  final Color? color;

  @override
  Widget build(BuildContext context) => Text(
        text,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          letterSpacing: 2,
          color: color ?? context.muted,
        ),
      );
}

/// 分节标题:"01 / 频道" + 计数徽章 + 副题。
class SectionTitle extends StatelessWidget {
  const SectionTitle({
    super.key,
    required this.no,
    required this.title,
    this.count,
    this.subtitle,
  });

  final String no;
  final String title;
  final int? count;
  final String? subtitle;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Text(
              '$no / ',
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w700,
                color: context.muted,
                letterSpacing: 1,
              ),
            ),
            Text(
              title,
              style: const TextStyle(
                fontSize: 17,
                fontWeight: FontWeight.w700,
                letterSpacing: 1,
              ),
            ),
            if (count != null) ...[
              const SizedBox(width: 10),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 2),
                decoration: BoxDecoration(
                  color: context.tint,
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  '$count',
                  style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: Color(0xFFACD2BD),
                  ),
                ),
              ),
            ],
          ],
        ),
        if (subtitle != null) ...[
          const SizedBox(height: 4),
          Text(subtitle!,
              style: TextStyle(fontSize: 12, color: context.muted)),
        ],
      ],
    );
  }
}

/// 信息横幅:浅 tint 底 + 图标,用于后端/ffprobe 状态提示。
class Notice extends StatelessWidget {
  const Notice(this.text, {super.key, this.error = false, this.action});
  final String text;
  final bool error;
  final Widget? action;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: error
              ? Theme.of(context).colorScheme.errorContainer.withValues(alpha: .5)
              : context.tint,
          borderRadius: BorderRadius.circular(14),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              error ? Icons.info_outline : Icons.verified_user_outlined,
              size: 19,
              color: error
                  ? Theme.of(context).colorScheme.error
                  : Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                text,
                style: TextStyle(
                  fontSize: 12,
                  height: 1.65,
                  color: error ? Theme.of(context).colorScheme.error : context.ink,
                ),
              ),
            ),
            ?action,
          ],
        ),
      );
}

/// 按钮内的忙碌态(图标位换成小圈)。
class BusyLabel extends StatelessWidget {
  const BusyLabel(this.label,
      {super.key, required this.busy, this.icon = Icons.arrow_forward_rounded});
  final String label;
  final bool busy;
  final IconData icon;

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          if (busy)
            const SizedBox(
                width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
          else
            Icon(icon, size: 19),
          const SizedBox(width: 10),
          Text(label),
        ],
      );
}

void showMessage(BuildContext context, String message) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(content: Text(message)));
}
