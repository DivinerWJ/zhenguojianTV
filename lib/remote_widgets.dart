import 'package:flutter/material.dart';

import 'app_layout.dart';
import 'search_input.dart';
import 'tv_keyboard.dart';
import 'package:flutter/services.dart';

/// 「切换站源」胶囊入口（0.8.2）。
///
/// 首页 AppBar 和榜单页 AppBar 原本都是"裸文字 + 下箭头"，问题一致：
/// - 看不出可点（和标题长得一模一样）；
/// - 电视上 `InkWell` 不参与焦点遍历，遥控器**停不到这里**，等于没有入口。
///
/// 统一成胶囊后：图标 + 名称 + 站源数量 + 箭头，电视下再套 [RemoteTarget]
/// 拿到焦点环 / 放大 / 外发光。
class SourceSwitchPill extends StatelessWidget {
  const SourceSwitchPill({
    super.key,
    required this.label,
    required this.onPressed,
    this.count = 1,
    this.icon,
    this.television,
  });

  final String label;

  /// 该分组下的站源数量；>1 时显示数量徽标。
  final int count;
  final VoidCallback? onPressed;
  final IconData? icon;

  /// 覆盖电视判定（测试用）；为空时读 [AppLayout]。
  final bool? television;

  @override
  Widget build(BuildContext context) {
    final tv = television ?? AppLayout.isTelevision(context);
    final colors = Theme.of(context).colorScheme;
    final enabled = onPressed != null;
    final pill = Container(
      // 用 minHeight + 内边距而不是固定 height：系统字体放大（textScale 2.0）
      // 时固定高度会把标题裁掉，minHeight 能自然长高。
      constraints: BoxConstraints(minHeight: tv ? 52 : 40),
      padding: EdgeInsets.symmetric(
        horizontal: tv ? 14 : 10,
        vertical: tv ? 6 : 4,
      ),
      decoration: BoxDecoration(
        color: colors.surfaceContainerHighest.withValues(alpha: tv ? .62 : .78),
        borderRadius: BorderRadius.circular(tv ? 14 : 12),
        border: Border.all(
          color: colors.outlineVariant.withValues(alpha: .55),
          width: 1,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            icon ?? (tv ? Icons.live_tv_rounded : Icons.dns_rounded),
            size: tv ? 22 : 17,
            color: enabled ? colors.primary : colors.onSurfaceVariant,
          ),
          SizedBox(width: tv ? 10 : 7),
          Flexible(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: tv ? 20 : 16,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          // 多站源时给出数量，避免「黄果」被误认为只有一个入口。
          if (tv && count > 1) ...[
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
              decoration: BoxDecoration(
                color: colors.primary.withValues(alpha: .16),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                '$count 源',
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w700,
                  color: colors.primary,
                ),
              ),
            ),
          ],
          if (enabled) ...[
            const SizedBox(width: 2),
            Icon(
              Icons.expand_more_rounded,
              size: tv ? 26 : 20,
              color: colors.onSurfaceVariant,
            ),
          ],
        ],
      ),
    );
    if (tv) {
      return RemoteTarget(
        onPressed: onPressed,
        label: '站源：$label${enabled ? '，按确定切换' : '，当前不可用'}',
        child: pill,
      );
    }
    return InkWell(
      borderRadius: BorderRadius.circular(12),
      onTap: onPressed,
      child: pill,
    );
  }
}

class RemoteTarget extends StatefulWidget {
  const RemoteTarget({
    super.key,
    required this.child,
    required this.onPressed,
    this.focusNode,
    this.onFocus,
    this.autofocus = false,
    this.selected = false,
    this.label,
    this.accent,
  });
  final Widget child;
  final VoidCallback? onPressed;
  final FocusNode? focusNode;
  final VoidCallback? onFocus;
  final bool autofocus;
  final bool selected;
  final String? label;

  /// 焦点色覆盖（0.8.2）。
  ///
  /// 播放页的弹层是**深色自定义主题**（底色 #191A20、主色 #FF765F），
  /// 直接用 `colorScheme.primaryContainer` 会出现一块蓝色底，和橙色 UI 打架。
  /// 传入 accent 后，焦点描边/外发光/焦点底都用这个色系推导。
  final Color? accent;

  @override
  State<RemoteTarget> createState() => _RemoteTargetState();
}

class _RemoteTargetState extends State<RemoteTarget> {
  bool _focused = false;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final accent = widget.accent ?? colors.primary;
    final focusFill = widget.accent != null
        ? accent.withValues(alpha: .22)
        : colors.primaryContainer;
    // 0.8.1 焦点样式增强（10 英尺 UI）：
    // - 缩放 1.03：远距离一眼能看出"当前在哪"；
    // - 焦点环 3.5px + 外发光：海报本身颜色杂乱时，纯描边容易糊掉；
    // - 选中态（selected）保持细一档的描边，与焦点态区分开。
    return FocusableActionDetector(
      focusNode: widget.focusNode,
      autofocus: widget.autofocus,
      enabled: widget.onPressed != null,
      actions: {
        ActivateIntent: CallbackAction<ActivateIntent>(
          onInvoke: (_) {
            widget.onPressed?.call();
            return null;
          },
        ),
      },
      onFocusChange: (focused) {
        if (mounted) {
          setState(() => _focused = focused);
        }
        if (focused) {
          widget.onFocus?.call();
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted && _focused) {
              Scrollable.ensureVisible(
                context,
                alignmentPolicy: ScrollPositionAlignmentPolicy.keepVisibleAtEnd,
              );
            }
          });
        }
      },
      child: Semantics(
        button: true,
        enabled: widget.onPressed != null,
        focused: _focused,
        label: widget.label,
        child: MouseRegion(
          cursor: widget.onPressed == null
              ? SystemMouseCursors.basic
              : SystemMouseCursors.click,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: widget.onPressed,
            child: AnimatedScale(
              scale: _focused ? 1.045 : 1,
              duration: const Duration(milliseconds: 140),
              curve: Curves.easeOutCubic,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 120),
                padding: const EdgeInsets.all(4),
                decoration: BoxDecoration(
                  color: _focused || widget.selected
                      ? focusFill
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(14),
                  border: Border.all(
                    color: _focused
                        ? Colors.white
                        : widget.selected
                        ? accent
                        : Colors.transparent,
                    width: _focused ? 3 : 2.5,
                  ),
                  boxShadow: _focused
                      ? [
                          BoxShadow(
                            color: accent.withValues(alpha: .45),
                            blurRadius: 18,
                            spreadRadius: 2,
                          ),
                        ]
                      : null,
                ),
                child: ExcludeFocus(child: widget.child),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class RemoteButton extends StatelessWidget {
  const RemoteButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.icon,
    this.selected = false,
    this.autofocus = false,
    this.focusNode,
  });
  final String label;
  final VoidCallback? onPressed;
  final IconData? icon;
  final bool selected;
  final bool autofocus;
  final FocusNode? focusNode;

  @override
  Widget build(BuildContext context) => RemoteTarget(
    onPressed: onPressed,
    selected: selected,
    autofocus: autofocus,
    focusNode: focusNode,
    label: label,
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[Icon(icon, size: 22), const SizedBox(width: 8)],
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 18,
              color: onPressed == null
                  ? Theme.of(context).disabledColor
                  : Theme.of(context).colorScheme.onSurface,
            ),
          ),
        ],
      ),
    ),
  );
}

/// 电视端的「选项列表」对话框（0.8.2）。
///
/// 用来替代 `PopupMenuButton`：后者的弹出菜单在电视上贴边、无焦点环，
/// 遥控器很难看清停在哪一项，也无法用方向键稳定导航。
///
/// 每行都是可聚焦的 [RemoteTarget]，按确定回传对应的值；
/// 按返回键由 `DismissIntent` 关闭对话框并回传 null。
class RemoteOptionSheet extends StatelessWidget {
  const RemoteOptionSheet({
    super.key,
    required this.title,
    required this.options,
    this.keyPrefix = 'option',
    this.cancelLabel = '取消',
  });

  final String title;

  /// (回传值, 显示文案, 是否可用)
  final List<(String, String, bool)> options;
  final String keyPrefix;
  final String? cancelLabel;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final firstEnabled = options.indexWhere((option) => option.$3);
    return Container(
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 18),
      decoration: BoxDecoration(
        color: colors.surface,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(
          color: colors.outlineVariant.withValues(alpha: .8),
          width: 1,
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: .45),
            blurRadius: 32,
            spreadRadius: 4,
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 16),
          for (var index = 0; index < options.length; index++)
            Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: RemoteTarget(
                key: ValueKey('$keyPrefix-${options[index].$1}'),
                autofocus: index == firstEnabled,
                onPressed: options[index].$3
                    ? () => Navigator.of(context).pop(options[index].$1)
                    : null,
                label: options[index].$2,
                child: Container(
                  height: 52,
                  alignment: Alignment.centerLeft,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  decoration: BoxDecoration(
                    color: colors.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(color: colors.outlineVariant, width: 1),
                  ),
                  child: Text(
                    options[index].$2,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w600,
                      color: options[index].$3
                          ? colors.onSurface
                          : colors.onSurfaceVariant.withValues(alpha: .5),
                    ),
                  ),
                ),
              ),
            ),
          if (cancelLabel != null) ...[
            const SizedBox(height: 2),
            RemoteTarget(
              key: ValueKey('$keyPrefix-cancel'),
              onPressed: () => Navigator.of(context).pop(),
              label: cancelLabel!,
              child: Container(
                height: 52,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: colors.outlineVariant, width: 1),
                ),
                child: Text(
                  cancelLabel!,
                  style: TextStyle(
                    fontSize: 18,
                    color: colors.onSurfaceVariant,
                  ),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class RemoteGrid extends StatefulWidget {
  const RemoteGrid({
    super.key,
    required this.itemKeys,
    required this.columns,
    required this.itemExtent,
    required this.itemBuilder,
    this.controller,
    this.spacing = 14,
    this.padding = const EdgeInsets.all(18),
    this.footer,
    this.initialIndex = 0,
    this.autofocus = false,
  });
  final List<String> itemKeys;
  final int columns;
  final double itemExtent;
  final double spacing;
  final EdgeInsets padding;
  final ScrollController? controller;
  final Widget? footer;
  final int initialIndex;
  final bool autofocus;
  final Widget Function(BuildContext, int, FocusNode, VoidCallback) itemBuilder;

  @override
  State<RemoteGrid> createState() => _RemoteGridState();
}

class _RemoteGridState extends State<RemoteGrid> {
  final _ownScroll = ScrollController();
  final _nodes = <String, FocusNode>{};
  String? _focused;
  String? _target;
  int _generation = 0;
  ScrollController get _scroll => widget.controller ?? _ownScroll;

  FocusNode _node(int index) => _nodes.putIfAbsent(
    widget.itemKeys[index],
    () => FocusNode(debugLabel: 'remote-${widget.itemKeys[index]}'),
  );

  @override
  void initState() {
    super.initState();
    if (widget.autofocus) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && widget.itemKeys.isNotEmpty) {
          _focusAt(widget.initialIndex.clamp(0, widget.itemKeys.length - 1));
        }
      });
    }
  }

  void _focusAt(int index) {
    final generation = ++_generation;
    _target = widget.itemKeys[index];
    final node = _node(index);
    if (node.context != null) {
      node.requestFocus();
      _target = null;
      return;
    }
    if (_scroll.hasClients) {
      final top =
          widget.padding.top +
          (index ~/ widget.columns) * (widget.itemExtent + widget.spacing);
      final offset = top < _scroll.offset
          ? top
          : top + widget.itemExtent - _scroll.position.viewportDimension;
      _scroll.jumpTo(offset.clamp(0.0, _scroll.position.maxScrollExtent));
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && generation == _generation && node.context != null) {
        node.requestFocus();
        _target = null;
      }
    });
  }

  KeyEventResult _key(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }
    if (_focused == null || !(_nodes[_focused]?.hasFocus ?? false)) {
      return KeyEventResult.ignored;
    }
    final index = widget.itemKeys.indexOf(_target ?? _focused!);
    if (index < 0) return KeyEventResult.ignored;
    final key = event.logicalKey;
    final int next;
    if (key == LogicalKeyboardKey.arrowDown) {
      if (index ~/ widget.columns ==
          (widget.itemKeys.length - 1) ~/ widget.columns) {
        return KeyEventResult.ignored;
      }
      next = (index + widget.columns).clamp(0, widget.itemKeys.length - 1);
    } else if (key == LogicalKeyboardKey.arrowUp) {
      next = index - widget.columns;
    } else if (key == LogicalKeyboardKey.arrowRight) {
      if (index % widget.columns == widget.columns - 1 ||
          index == widget.itemKeys.length - 1) {
        return KeyEventResult.handled;
      }
      next = index + 1;
    } else if (key == LogicalKeyboardKey.arrowLeft) {
      if (index % widget.columns == 0) return KeyEventResult.ignored;
      next = index - 1;
    } else {
      return KeyEventResult.ignored;
    }
    if (next < 0 || next >= widget.itemKeys.length) {
      return KeyEventResult.ignored;
    }
    _focusAt(next);
    return KeyEventResult.handled;
  }

  @override
  void didUpdateWidget(covariant RemoteGrid oldWidget) {
    super.didUpdateWidget(oldWidget);
    final removed = _nodes.keys
        .where((key) => !widget.itemKeys.contains(key))
        .toList();
    final restore = removed.any((key) => _nodes[key]!.hasFocus);
    final index = oldWidget.itemKeys.indexOf(_focused ?? '');
    for (final key in removed) {
      _nodes.remove(key)?.dispose();
    }
    if (_focused != null && !widget.itemKeys.contains(_focused)) {
      _focused = null;
      _target = null;
      _generation++;
    }
    if (restore && widget.itemKeys.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && widget.itemKeys.isNotEmpty) {
          _focusAt(index.clamp(0, widget.itemKeys.length - 1));
        }
      });
    }
  }

  @override
  void dispose() {
    _generation++;
    for (final node in _nodes.values) {
      node.dispose();
    }
    _ownScroll.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Focus(
    canRequestFocus: false,
    skipTraversal: true,
    onKeyEvent: _key,
    child: CustomScrollView(
      controller: _scroll,
      slivers: [
        SliverPadding(
          padding: widget.padding,
          sliver: SliverGrid(
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: widget.columns,
              mainAxisExtent: widget.itemExtent,
              crossAxisSpacing: widget.spacing,
              mainAxisSpacing: widget.spacing,
            ),
            delegate: SliverChildBuilderDelegate(
              (context, index) =>
                  widget.itemBuilder(context, index, _node(index), () {
                    _focused = widget.itemKeys[index];
                  }),
              childCount: widget.itemKeys.length,
            ),
          ),
        ),
        if (widget.footer != null) SliverToBoxAdapter(child: widget.footer),
      ],
    ),
  );
}

class RemoteEpisodeButton extends StatelessWidget {
  const RemoteEpisodeButton({
    super.key,
    required this.number,
    required this.onPressed,
    this.vip = false,
    this.current = false,
    this.focusNode,
    this.onFocus,
  });
  final int number;
  final bool vip;
  final bool current;
  final VoidCallback onPressed;
  final FocusNode? focusNode;
  final VoidCallback? onFocus;

  @override
  Widget build(BuildContext context) => RemoteTarget(
    focusNode: focusNode,
    onFocus: onFocus,
    selected: current,
    onPressed: onPressed,
    label: '第 $number 集${vip ? '，VIP 试看' : ''}',
    child: Center(
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Flexible(
            child: Text(
              '$number',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 20),
            ),
          ),
          if (vip) ...[
            const SizedBox(width: 4),
            Icon(
              Icons.workspace_premium_rounded,
              color: Theme.of(context).colorScheme.tertiary,
              size: 18,
            ),
          ],
        ],
      ),
    ),
  );
}

/// 电视端搜索对话框（0.8.0 引入，0.8.3 重做）。
///
/// 0.8.0 版本只有一个 `autofocus` 的输入框 + 最近搜索，问题是：
/// 1. 焦点一进来就落在 `TextField` 上，没有输入法的盒子会**卡死在输入框**里；
/// 2. 没有输入法就只能搜"最近搜索"里已有的词，等于搜不了新剧。
///
/// 0.8.3 改成三条路并存：
/// - 屏幕键盘（[TvSearchKeyboard]）：只用遥控器就能打字，且是默认焦点；
/// - 输入框保留：有输入法的设备点一下照样用 IME；
/// - 热门搜索：直接从当前剧库取片名，无输入法也能一键搜到正在看的剧。
class TelevisionSearchDialog extends StatefulWidget {
  const TelevisionSearchDialog({
    super.key,
    required this.initialValue,
    required this.title,
    this.suggestions,
    this.recentSearches = const [],
    this.hotSearches = const [],
    this.onCancel,
  });

  final Future<List<String>> Function(String)? suggestions;
  final List<String> recentSearches;

  /// 热门搜索词（0.8.3）。调用方传当前剧库里的片名，
  /// 这样"热门"是**真的有货**的词，而不是写死的营销词。
  final List<String> hotSearches;
  final VoidCallback? onCancel;
  final String initialValue;
  final String title;

  @override
  State<TelevisionSearchDialog> createState() => _TelevisionSearchDialogState();
}

class _TelevisionSearchDialogState extends State<TelevisionSearchDialog> {
  late final _controller = TextEditingController(text: widget.initialValue);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// 最近搜索在前、热门在后，去重后最多 10 个。
  List<String> get _keywords {
    final seen = <String>{};
    final result = <String>[];
    for (final word in [...widget.recentSearches, ...widget.hotSearches]) {
      final trimmed = word.trim();
      if (trimmed.isEmpty || !seen.add(trimmed)) continue;
      result.add(trimmed);
      if (result.length >= 10) break;
    }
    return result;
  }

  void _setText(String text) {
    _controller.value = TextEditingValue(
      text: text,
      selection: TextSelection.collapsed(offset: text.length),
    );
    // 重建一次，让输入框右侧的「清空 / 搜索」图标跟随文本变化。
    setState(() {});
  }

  void _append(String char) {
    // 60 字上限：够长，又能挡住误触长按把缓冲区打爆。
    if (_controller.text.length >= 60) return;
    _setText('${_controller.text}$char');
  }

  void _backspace() {
    final text = _controller.text;
    if (text.isEmpty) return;
    _setText(text.substring(0, text.length - 1));
  }

  void _clear() => _setText('');

  void _submit() => Navigator.pop(context, _controller.text.trim());

  @override
  Widget build(BuildContext context) {
    final keywords = _keywords;
    final colors = Theme.of(context).colorScheme;
    return AlertDialog(
      // 电视过扫描：四周留够，避免对话框边缘被切掉。
      insetPadding: const EdgeInsets.symmetric(horizontal: 48, vertical: 24),
      title: Text(widget.title),
      content: SizedBox(
        width: 760,
        child: ConstrainedBox(
          // 720p 盒子也要放得下：内容超出时滚动，而不是撑破屏幕。
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * .74,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SearchInput(
                  // 关键改动：不再 autofocus。
                  // 焦点交给下面的屏幕键盘，没有输入法的盒子也不会卡在输入框里。
                  autofocus: false,
                  controller: _controller,
                  hint: '输入剧名，或拼音首字母（如 hjdnz）',
                  suggestions: widget.suggestions,
                  onCancel: widget.onCancel,
                  onChanged: (value) => setState(() {}),
                  onSearch: (value) => Navigator.pop(context, value),
                ),
                const SizedBox(height: 14),
                TvSearchKeyboard(
                  onCharacter: _append,
                  onBackspace: _backspace,
                  onClear: _clear,
                  onSubmit: _submit,
                ),
                const SizedBox(height: 10),
                Text(
                  // 遥控器打中文很痛苦，明确告诉用户可以用拼音首字母。
                  '支持拼音首字母：如「黄金大逆转」输入 hjdnz 即可',
                  style: TextStyle(
                    fontSize: 13,
                    color: colors.onSurfaceVariant,
                  ),
                ),
                if (keywords.isNotEmpty) ...[
                  const SizedBox(height: 18),
                  Text(
                    '热门搜索',
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                      color: colors.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 10),
                  Wrap(
                    spacing: 10,
                    runSpacing: 10,
                    children: [
                      for (final word in keywords)
                        RemoteTarget(
                          key: ValueKey('tv-hot-$word'),
                          onPressed: () => Navigator.pop(context, word),
                          label: '搜索 $word',
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 10,
                            ),
                            decoration: BoxDecoration(
                              color: colors.surfaceContainerHighest,
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Text(
                              word,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(fontSize: 16),
                            ),
                          ),
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        TextButton(
          onPressed: () => Navigator.pop(context, ''),
          child: const Text('清空'),
        ),
        FilledButton(
          onPressed: _submit,
          child: const Text('搜索'),
        ),
      ],
    );
  }
}
