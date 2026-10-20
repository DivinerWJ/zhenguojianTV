import 'dart:async';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

class AppBottomNavigation extends StatefulWidget {
  const AppBottomNavigation({
    super.key,
    required this.selectedIndex,
    required this.onDestinationSelected,
    required this.destinations,
    this.onDestinationDoubleTap,
  });

  final int selectedIndex;
  final ValueChanged<int> onDestinationSelected;
  final List<NavigationDestination> destinations;

  /// 双击某个标签时的回调（例如双击"首页"重新随机）。
  ///
  /// 不直接绑定到 [InkWell.onDoubleTap]：Flutter 的单击/双击手势识别器会
  /// 竞争同一个 pointer arena，单击需要等待双击窗口确认，手机上会产生一种
  /// "点了以后顿一下才切页"的感觉。这里让第一次点击立即切页，再用时间窗
  /// 识别第二次点击，因此保留双击能力的同时不牺牲单击响应速度。
  final ValueChanged<int>? onDestinationDoubleTap;

  @override
  State<AppBottomNavigation> createState() => _AppBottomNavigationState();
}

class _AppBottomNavigationState extends State<AppBottomNavigation> {
  int? _lastTapIndex;
  Timer? _doubleTapWindow;

  void _handleTap(int index) {
    final doubleTap =
        widget.onDestinationDoubleTap != null &&
        _lastTapIndex == index &&
        (_doubleTapWindow?.isActive ?? false);

    // 第一次点击立即切页，不等待双击识别超时。
    widget.onDestinationSelected(index);

    _doubleTapWindow?.cancel();
    if (doubleTap) {
      _lastTapIndex = null;
      _doubleTapWindow = null;
      widget.onDestinationDoubleTap!(index);
      return;
    }

    _lastTapIndex = index;
    _doubleTapWindow = Timer(kDoubleTapTimeout, () {
      _lastTapIndex = null;
      _doubleTapWindow = null;
    });
  }

  @override
  void dispose() {
    _doubleTapWindow?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    return Material(
      color: theme.scaffoldBackgroundColor,
      child: SafeArea(
        top: false,
        child: DecoratedBox(
          decoration: BoxDecoration(
            border: Border(
              top: BorderSide(color: colors.outlineVariant, width: .5),
            ),
          ),
          child: Row(
            children: [
              for (final (index, destination) in widget.destinations.indexed)
                Expanded(
                  child: Semantics(
                    container: true,
                    button: true,
                    selected: index == widget.selectedIndex,
                    child: Tooltip(
                      message: destination.label,
                      excludeFromSemantics: true,
                      child: InkWell(
                        key: ValueKey('bottom-nav-$index'),
                        onTap: () => _handleTap(index),
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(
                            minHeight: 68,
                            minWidth: 48,
                          ),
                          child: Padding(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 4,
                              vertical: 9,
                            ),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                AnimatedContainer(
                                  duration: const Duration(milliseconds: 120),
                                  curve: Curves.easeOut,
                                  width: 18,
                                  height: 3,
                                  decoration: BoxDecoration(
                                    color: index == widget.selectedIndex
                                        ? colors.primary
                                        : Colors.transparent,
                                    borderRadius: BorderRadius.circular(1.5),
                                  ),
                                ),
                                const SizedBox(height: 6),
                                IconTheme(
                                  data: IconThemeData(
                                    size: 24,
                                    color: index == widget.selectedIndex
                                        ? colors.primary
                                        : colors.onSurfaceVariant,
                                  ),
                                  child: index == widget.selectedIndex
                                      ? destination.selectedIcon ??
                                            destination.icon
                                      : destination.icon,
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  destination.label,
                                  textAlign: TextAlign.center,
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 12,
                                    height: 1.2,
                                    fontWeight: index == widget.selectedIndex
                                        ? FontWeight.w700
                                        : FontWeight.w500,
                                    color: index == widget.selectedIndex
                                        ? colors.primary
                                        : colors.onSurfaceVariant,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
