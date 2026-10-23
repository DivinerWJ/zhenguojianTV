import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'app_build.dart';

/// 开屏页（启动画面）。
///
/// 解决的问题：进入软件后，首页要等原生核心初始化 + 拉取第一批视频
/// 才显示内容，中间这段时间是空白等待。
///
/// 做法：先用开屏页盖住，等首页真正"有视频可播"再淡出，
/// 用户感知上就是"一点开就有画面"，而不是"进去后干等"。
///
/// 设计约束：
/// - 不阻塞启动：只做视觉遮挡，初始化逻辑仍在后台并行进行。
/// - **最多停留 [timeout]**：首次拉数据可能较慢，不能让用户干等；
///   超时后**无论如何都进入首页**，首页自己显示加载占位或离线提示。
/// - 淡出动画不改变布局，不影响底部导航与顶部标题栏。
///
/// 0.8.10 修复（用户反馈"选了香蕉这种不可用站源，退出再进就一直卡在加载页"）：
/// 原来 [timeout] 只把"最短展示时间"置位，仍然要求 `ready` 为真才淡出。
/// 而 `ready` 只在首页**真的拉到第一批视频**时才置位 —— 站源不可用、
/// 或者断网启动时它永远是 false，于是开屏页永久盖在上面，用户只能杀进程。
/// 现在超时是**硬超时**：时间一到直接放行，剩下的交给首页的
/// 错误态 / 离线态（首页本身有"重试"按钮，所以离线也能进）。
class AppSplash extends StatefulWidget {
  const AppSplash({
    super.key,
    required this.child,
    required this.ready,
    this.minimumDuration = const Duration(milliseconds: 420),
    this.timeout = const Duration(milliseconds: 2200),
  });

  /// 真正的主界面。
  final Widget child;

  /// 主界面是否已经准备好（例如首页已拉到第一批视频）。
  final ValueListenable<bool> ready;

  /// 开屏页最短展示时间，避免"闪一下就没了"。
  final Duration minimumDuration;

  /// 最长等待时间，超过则**强制**放行（不再等 [ready]）。
  final Duration timeout;

  @override
  State<AppSplash> createState() => _AppSplashState();
}

class _AppSplashState extends State<AppSplash> {
  bool _dismissed = false;
  bool _minimumElapsed = false;
  bool _timedOut = false;
  Timer? _minimumTimer;
  Timer? _timeoutTimer;

  @override
  void initState() {
    super.initState();
    _minimumTimer = Timer(widget.minimumDuration, () {
      if (!mounted) return;
      _minimumElapsed = true;
      _maybeDismiss();
    });
    _timeoutTimer = Timer(widget.timeout, () {
      if (!mounted) return;
      // 硬超时：置位后 `_maybeDismiss` 不再看 `ready`。
      _timedOut = true;
      _minimumElapsed = true;
      _maybeDismiss();
    });
    widget.ready.addListener(_maybeDismiss);
    _maybeDismiss();
  }

  @override
  void didUpdateWidget(covariant AppSplash oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.ready != widget.ready) {
      oldWidget.ready.removeListener(_maybeDismiss);
      widget.ready.addListener(_maybeDismiss);
    }
  }

  @override
  void dispose() {
    _minimumTimer?.cancel();
    _timeoutTimer?.cancel();
    widget.ready.removeListener(_maybeDismiss);
    super.dispose();
  }

  void _maybeDismiss() {
    if (!mounted || _dismissed) return;
    if (!_minimumElapsed) return;
    if (!_timedOut && !widget.ready.value) return;
    setState(() => _dismissed = true);
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        widget.child,
        // 用 `IgnorePointer` + 淡出：淡出期间不吞掉首页的点击，
        // 否则刚进首页的前 300ms 遥控器/手指操作会被开屏页吃掉。
        if (!_dismissed)
          TickerMode(
            enabled: !_dismissed,
            child: IgnorePointer(child: const _SplashSurface()),
          ),
      ],
    );
  }
}

/// 开屏页视觉。
///
/// 0.8.10 重新设计：原来是一个图标 + 标题 + 转圈，转圈在站源不可用时
/// 会一直转（因为永远不结束）。现在改成「品牌 + 一条不确定进度条」，
/// 进度条本身不表达"还需要多久"，只表示"在准备"，配合 2.2 秒的硬超时，
/// 不会给用户"卡住了"的错觉。
class _SplashSurface extends StatelessWidget {
  const _SplashSurface();

  static const _accent = Color(0xFFFF765F);
  static const _background = Color(0xFF101114);

  @override
  Widget build(BuildContext context) => DecoratedBox(
    decoration: const BoxDecoration(
      gradient: LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [_background, Color(0xFF16181D), _background],
      ),
    ),
    child: Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 92,
            height: 92,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(28),
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [_accent, Color(0xFFFF9A6B)],
              ),
              boxShadow: [
                BoxShadow(
                  color: _accent.withValues(alpha: .32),
                  blurRadius: 32,
                  spreadRadius: 2,
                ),
              ],
            ),
            child: const Icon(
              Icons.play_arrow_rounded,
              color: Colors.white,
              size: 56,
            ),
          ),
          const SizedBox(height: 26),
          Text(
            appName,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 27,
              fontWeight: FontWeight.w800,
              letterSpacing: 6,
            ),
          ),
          const SizedBox(height: 10),
          const Text(
            '短剧 · 漫剧 · AI 剧',
            style: TextStyle(
              color: Colors.white38,
              fontSize: 12,
              letterSpacing: 3,
            ),
          ),
          const SizedBox(height: 44),
          const SizedBox(
            width: 132,
            height: 3,
            child: _SplashBar(color: _accent),
          ),
        ],
      ),
    ),
  );
}

/// 不确定进度条：一条循环滑动的短线，比 `CircularProgressIndicator` 更安静。
class _SplashBar extends StatefulWidget {
  const _SplashBar({required this.color});

  final Color color;

  @override
  State<_SplashBar> createState() => _SplashBarState();
}

class _SplashBarState extends State<_SplashBar>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  )..repeat();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => ClipRRect(
    borderRadius: BorderRadius.circular(2),
    child: ColoredBox(
      color: Colors.white.withValues(alpha: .10),
      child: AnimatedBuilder(
        animation: _controller,
        builder: (context, _) {
          final progress = _controller.value;
          // 线段在 0..1 之间来回滑动，宽度固定 38%。
          final start = (progress * 1.38 - .38).clamp(0.0, 1.0);
          return Align(
            alignment: Alignment(-1 + start * 2, 0),
            child: FractionallySizedBox(
              widthFactor: .38,
              child: Container(
                height: 3,
                decoration: BoxDecoration(
                  color: widget.color,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
          );
        },
      ),
    ),
  );
}
