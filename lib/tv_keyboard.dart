import 'package:flutter/material.dart';

import 'remote_widgets.dart';

/// 电视端屏幕键盘（0.8.3）。
///
/// 为什么需要它：电视搜索原先直接 `autofocus` 一个 [TextField]，
/// 靠系统输入法打字。但相当一部分盒子（尤其运营商定制机、投影仪、
/// 模拟器）**根本没有安装输入法**——焦点落进输入框之后，
/// 遥控器方向键出不来、确定键没反应，整个搜索功能直接死在那里。
///
/// 这个键盘只用遥控器就能完成输入：
/// - 方向键在字母之间移动，确定键输入一个字符；
/// - ⌫ 删一个字符，清空 / 搜索在最后一行；
/// - 系统输入法可用的设备仍然可以点输入框用 IME 打字，两条路并存。
///
/// 键位排布按 QWERTY，不按拼音——没有词库做不了拼音转汉字，
/// 而站源搜索本身就是按剧名子串匹配，输入汉字才有意义。
class TvSearchKeyboard extends StatelessWidget {
  const TvSearchKeyboard({
    super.key,
    required this.onCharacter,
    required this.onBackspace,
    required this.onClear,
    required this.onSubmit,
  });

  /// 输入一个字符（小写字母 / 数字）。
  final ValueChanged<String> onCharacter;

  /// 删除最后一个字符。
  final VoidCallback onBackspace;

  /// 清空全部输入。
  final VoidCallback onClear;

  /// 提交搜索。
  final VoidCallback onSubmit;

  static const _rows = <String>['qwertyuiop', 'asdfghjkl', 'zxcvbnm'];
  static const _digits = '1234567890';

  /// 单个按键尺寸。10 键一行 × 52 + 间隙 ≈ 580，正好放进 760 宽的对话框。
  static const double _keyWidth = 52;
  static const double _keyHeight = 50;

  @override
  Widget build(BuildContext context) {
    // 整个键盘只在首帧抢一次焦点，后续重建（每输入一个字符都会重建）
    // 不再重复请求，否则会把用户已经移动过去的焦点拉回第一个键。
    var autofocusPending = true;

    Widget key(
      String label,
      VoidCallback onPressed, {
      String? id,
      double width = _keyWidth,
      IconData? icon,
      bool primary = false,
    }) {
      final autofocus = autofocusPending;
      autofocusPending = false;
      return RemoteTarget(
        key: ValueKey('tv-key-${id ?? label}'),
        autofocus: autofocus,
        onPressed: onPressed,
        label: label,
        child: _KeyCap(
          label: label,
          icon: icon,
          width: width,
          height: _keyHeight,
          primary: primary,
        ),
      );
    }

    Widget row(List<Widget> keys) => Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        for (final child in keys)
          Padding(padding: const EdgeInsets.all(3), child: child),
      ],
    );

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final line in _rows)
          row([
            for (final char in line.split(''))
              key(char, () => onCharacter(char), id: char),
            if (line == _rows.last)
              key(
                '删除',
                onBackspace,
                id: 'backspace',
                width: _keyWidth * 1.6,
                icon: Icons.backspace_outlined,
              ),
          ]),
        row([
          for (final char in _digits.split(''))
            key(char, () => onCharacter(char), id: char),
        ]),
        row([
          key('清空', onClear, id: 'clear', width: _keyWidth * 2.4),
          key(
            '搜索',
            onSubmit,
            id: 'submit',
            width: _keyWidth * 2.4,
            icon: Icons.search_rounded,
            primary: true,
          ),
        ]),
      ],
    );
  }
}

/// 键盘上的一个键帽。
class _KeyCap extends StatelessWidget {
  const _KeyCap({
    required this.label,
    required this.width,
    required this.height,
    this.icon,
    this.primary = false,
  });

  final String label;
  final double width;
  final double height;
  final IconData? icon;
  final bool primary;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      width: width,
      height: height,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: primary
            ? colors.primary.withValues(alpha: .16)
            : colors.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(9),
      ),
      child: icon == null
          ? Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 19,
                fontWeight: FontWeight.w600,
                color: primary ? colors.primary : colors.onSurface,
              ),
            )
          : Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  icon,
                  size: 20,
                  color: primary ? colors.primary : colors.onSurface,
                ),
                if (label.isNotEmpty) ...[
                  const SizedBox(width: 6),
                  Text(
                    label,
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w600,
                      color: primary ? colors.primary : colors.onSurface,
                    ),
                  ),
                ],
              ],
            ),
    );
  }
}
