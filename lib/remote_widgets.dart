import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

import 'app_layout.dart';
import 'search_input.dart';
import 'tv_keyboard.dart';

import 'package:flutter/services.dart';

/// 把获得焦点的控件滚进可视区域（0.8.7）。
///
/// `Scrollable.ensureVisible` 默认的对齐方式 `keepVisibleAtEnd` 只适合
/// "往下 / 往右"走：焦点在视口上方时，它把控件对齐到视口**底部**，
/// 遥控器从列表底部往上一格就滚不动。这里先看控件落在哪一层视口的
/// 哪一侧，再决定用 `keepVisibleAtStart` 还是 `keepVisibleAtEnd`。
void _revealFocused(BuildContext context) {
  final box = context.findRenderObject();
  if (box is! RenderBox || !box.attached || !box.hasSize) return;
  var policy = ScrollPositionAlignmentPolicy.keepVisibleAtEnd;
  final rect = box.localToGlobal(Offset.zero) & box.size;
  RenderObject? node = box.parent;
  while (node != null) {
    if (node is RenderBox && RenderAbstractViewport.maybeOf(node) != null) {
      // 先提升成 RenderBox 再问「是不是视口」。写 `is RenderAbstractViewport
      // && is RenderBox` 是不行的：两者是并列分支（RenderAbstractViewport 直接
      // 继承 RenderObject，RenderBox 由 RenderViewport 另行实现），Dart 不会把
      // 它提升成交集类型，node 会停在 RenderAbstractViewport 上取不到 size。
      final area = node.localToGlobal(Offset.zero) & node.size;
      if (rect.top < area.top || rect.left < area.left) {
        policy = ScrollPositionAlignmentPolicy.keepVisibleAtStart;
        break;
      }
      if (rect.bottom > area.bottom || rect.right > area.right) break;
    }
    node = node.parent;
  }
  Scrollable.ensureVisible(context, alignmentPolicy: policy);
}

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
      constraints: const BoxConstraints(minHeight: 40),
      padding: EdgeInsets.symmetric(
        horizontal: tv ? 12 : 10,
        vertical: tv ? 5 : 4,
      ),
      decoration: BoxDecoration(
        color: colors.surfaceContainerHighest.withValues(alpha: tv ? .62 : .78),
        borderRadius: BorderRadius.circular(12),
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
            size: tv ? 18 : 17,
            color: enabled ? colors.primary : colors.onSurfaceVariant,
          ),
          SizedBox(width: tv ? 8 : 7),
          Flexible(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w800,
              ),
            ),
          ),
          // 多站源时给出数量，避免「黄果」被误认为只有一个入口。
          if (tv && count > 1) ...[
            const SizedBox(width: 6),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
              decoration: BoxDecoration(
                color: colors.primary.withValues(alpha: .16),
                borderRadius: BorderRadius.circular(7),
              ),
              child: Text(
                '$count 源',
                style: TextStyle(
                  fontSize: 11,
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
              size: 20,
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

/// 焦点装饰的两种画法（0.8.9）。
///
/// 0.8.7 及之前统一用「四边各 4px 内边距 + 3px 白描边 + 1.045 放大 + 外发光」，
/// 三个问题在电视上都会被放大：
///
/// 1. **放大和外发光画在自身边界之外**。横向海报行的视口、海报的 `ClipRRect`、
///    弹窗里的 `SingleChildScrollView` 都会把它裁掉，表现就是"选框只显示一半"，
///    焦点越靠边越明显（首尾两张海报、热门搜索的最后一个词）。
/// 2. 描边与内容之间留了 4px 间隙，看上去像一个悬空的框，而不是"选中了这张卡"。
/// 3. 那 8px 的 chrome 是从海报身上挤出来的，封面平白小了 8px。
///
/// 现在所有焦点装饰都画在**自身边界之内**，任何父级裁剪都切不到它：
/// - [RemoteFocusStyle.ring]：紧贴内容的描边 + 填充，用于按钮、胶囊、列表项；
/// - [RemoteFocusStyle.corners]：四角标记，用于海报（不画整圈边框）。
enum RemoteFocusStyle { ring, corners }

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
    this.focusStyle = RemoteFocusStyle.ring,
  });

  /// 焦点样式额外占用的布局空间（0.8.9 起为 0）。
  ///
  /// 焦点装饰不再外扩、也不再放大，所以不需要预留空间。常量保留是为了
  /// 让 [DramaTile.extentFor] / 电视海报行的高度反算不必分版本判断，
  /// 读到 0 即为"按内容实际尺寸算"。
  static const double chrome = 0;

  final Widget child;
  final VoidCallback? onPressed;
  final FocusNode? focusNode;
  final VoidCallback? onFocus;
  final bool autofocus;
  final bool selected;
  final String? label;

  /// 焦点装饰画法，默认 [RemoteFocusStyle.ring]。
  final RemoteFocusStyle focusStyle;

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
    final ring = widget.focusStyle == RemoteFocusStyle.ring;
    // 0.8.9 焦点样式：
    // - 不再缩放、不再外发光、不再留 4px 间隙，装饰全部落在自身边界内，
    //   父级怎么裁剪都切不到（详见 [RemoteFocusStyle] 的说明）；
    // - 海报用四角标记，避免整圈边框在封面四周形成"虚框"；
    // - 按钮/胶囊用紧贴内容的描边 + 填充，观感是"这张卡亮了"而不是"套了个框"。
    final borderColor = _focused
        ? Colors.white
        : widget.selected
        ? accent
        : Colors.transparent;
    final borderWidth = _focused ? 3.0 : 2.0;
    return FocusableActionDetector(
      focusNode: widget.focusNode,
      autofocus: widget.autofocus,
      enabled: widget.onPressed != null,
      shortcuts: const <ShortcutActivator, Intent>{
        SingleActivator(LogicalKeyboardKey.select, includeRepeats: false):
            ActivateIntent(),
        SingleActivator(LogicalKeyboardKey.enter, includeRepeats: false):
            ActivateIntent(),
        SingleActivator(LogicalKeyboardKey.numpadEnter, includeRepeats: false):
            ActivateIntent(),
        SingleActivator(LogicalKeyboardKey.gameButtonA, includeRepeats: false):
            ActivateIntent(),
      },
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
            if (mounted && _focused) _revealFocused(context);
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
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 120),
              decoration: BoxDecoration(
                // 海报不铺底色，否则焦点时整块卡片蒙上一层色，封面发灰。
                color: ring && (_focused || widget.selected)
                    ? focusFill
                    : Colors.transparent,
                borderRadius: BorderRadius.circular(12),
              ),
              // 画在内容之上、且在自身矩形之内：不会被父级裁剪，也不会
              // 因为描边而让内容位移（foregroundDecoration 不参与布局）。
              foregroundDecoration: BoxDecoration(
                borderRadius: BorderRadius.circular(12),
                border: ring
                    ? Border.all(color: borderColor, width: borderWidth)
                    : null,
              ),
              child: Stack(
                fit: StackFit.passthrough,
                // 角标全部落在自身矩形内，禁止裁切没有意义，但保留 none
                // 以免以后有人往这一层加外扩装饰时被静默切掉。
                clipBehavior: Clip.none,
                children: [
                  ExcludeFocus(child: widget.child),
                  if (!ring)
                    Positioned.fill(
                      child: IgnorePointer(
                        child: _FocusCorners(
                          visible: _focused,
                          // 选集等"选中态"没有焦点时也给一圈极细的提示，
                          // 否则多选场景看不出哪些已经勾上。
                          selected: widget.selected,
                          accent: accent,
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 海报的焦点四角标记（0.8.9）。
///
/// 只在自身矩形**内部**画四个 L 形角标，不描整圈边框：
/// - 不会像整圈边框那样在封面外形成"虚框"，也不会被父级裁剪切掉；
/// - 在满屏封面的网格里，四角标记比细边框更容易一眼定位（10 英尺 UI）。
class _FocusCorners extends StatelessWidget {
  const _FocusCorners({
    required this.visible,
    required this.selected,
    required this.accent,
  });

  final bool visible;
  final bool selected;
  final Color accent;

  @override
  Widget build(BuildContext context) {
    const inset = 3.0;
    const length = 16.0;
    const stroke = 3.0;
    Widget corner(Alignment alignment, {required Color color}) {
      final isTop = alignment.y < 0;
      final isLeft = alignment.x < 0;
      final side = BorderSide(color: color, width: stroke);
      return Align(
        alignment: alignment,
        child: Container(
          width: length,
          height: length,
          decoration: BoxDecoration(
            border: Border(
              top: isTop ? side : BorderSide.none,
              bottom: isTop ? BorderSide.none : side,
              left: isLeft ? side : BorderSide.none,
              right: isLeft ? BorderSide.none : side,
            ),
          ),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.all(inset),
      child: Stack(
        children: [
          if (selected && !visible)
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(9),
                  border: Border.all(color: accent, width: 2),
                ),
              ),
            ),
          AnimatedOpacity(
            opacity: visible ? 1 : 0,
            duration: const Duration(milliseconds: 120),
            child: Stack(
              children: [
                corner(Alignment.topLeft, color: Colors.white),
                corner(Alignment.topRight, color: Colors.white),
                corner(Alignment.bottomLeft, color: Colors.white),
                corner(Alignment.bottomRight, color: Colors.white),
              ],
            ),
          ),
        ],
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
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 18),
            const SizedBox(width: 7),
          ],
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 15,
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
            style: const TextStyle(fontSize: 19, fontWeight: FontWeight.w800),
          ),
          const SizedBox(height: 12),
          for (var index = 0; index < options.length; index++)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: RemoteTarget(
                key: ValueKey('$keyPrefix-${options[index].$1}'),
                autofocus: index == firstEnabled,
                onPressed: options[index].$3
                    ? () => Navigator.of(context).pop(options[index].$1)
                    : null,
                label: options[index].$2,
                child: Container(
                  height: 42,
                  alignment: Alignment.centerLeft,
                  padding: const EdgeInsets.symmetric(horizontal: 14),
                  decoration: BoxDecoration(
                    color: colors.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: colors.outlineVariant, width: 1),
                  ),
                  child: Text(
                    options[index].$2,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 15,
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
                height: 42,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: colors.outlineVariant, width: 1),
                ),
                child: Text(
                  cancelLabel!,
                  style: TextStyle(
                    fontSize: 15,
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
              style: const TextStyle(fontSize: 16),
            ),
          ),
          if (vip) ...[
            const SizedBox(width: 3),
            Icon(
              Icons.workspace_premium_rounded,
              color: Theme.of(context).colorScheme.tertiary,
              size: 14,
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
      insetPadding: const EdgeInsets.symmetric(horizontal: 40, vertical: 20),
      title: Text(widget.title, style: const TextStyle(fontSize: 19)),
      content: SizedBox(
        width: 620,
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
                const SizedBox(height: 10),
                TvSearchKeyboard(
                  onCharacter: _append,
                  onBackspace: _backspace,
                  onClear: _clear,
                  onSubmit: _submit,
                ),
                const SizedBox(height: 8),
                Text(
                  // 遥控器打中文很痛苦，明确告诉用户可以用拼音首字母。
                  '支持拼音首字母：如「黄金大逆转」输入 hjdnz 即可',
                  style: TextStyle(
                    fontSize: 12,
                    color: colors.onSurfaceVariant,
                  ),
                ),
                if (keywords.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Text(
                    '热门搜索',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: colors.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      for (final word in keywords)
                        RemoteTarget(
                          key: ValueKey('tv-hot-$word'),
                          onPressed: () => Navigator.pop(context, word),
                          label: '搜索 $word',
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 7,
                            ),
                            decoration: BoxDecoration(
                              color: colors.surfaceContainerHighest,
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: Text(
                              word,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(fontSize: 14),
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
        FilledButton(onPressed: _submit, child: const Text('搜索')),
      ],
    );
  }
}
