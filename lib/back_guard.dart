/// 返回键去重（0.8.7）。
///
/// 部分电视盒子上，一次返回键会同时产生两条路径：
/// 键盘事件（`LogicalKeyboardKey.goBack`）和系统的 `popRoute`。
/// 两条都执行时表现为"按一次退两级"，或者"退出确认框关掉又立刻弹回来"。
/// 这里用时间窗把同一次按键的重复触发合并成一次。
library;

import 'package:flutter/material.dart';

class BackGuard {
  BackGuard({this.window = const Duration(milliseconds: 180)});

  final Duration window;
  DateTime? _last;

  /// 认领一次返回动作。窗口内的重复调用返回 false，调用方应当直接返回。
  bool claim() {
    final now = DateTime.now();
    final last = _last;
    if (last != null && now.difference(last) < window) return false;
    _last = now;
    return true;
  }

  void reset() => _last = null;
}

/// 返回键入口去重：键盘事件与系统 popRoute 只有一个能生效。
final entryBackGuard = BackGuard();

/// 键盘事件触发的返回：去重后交给 Navigator。
void popWithBackGuard(BuildContext context) {
  if (!entryBackGuard.claim()) return;
  Navigator.of(context).maybePop();
}
