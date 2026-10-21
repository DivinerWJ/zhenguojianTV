import 'package:flutter/material.dart';

import 'catalog_browser.dart';
import 'core_bridge.dart';
import 'models.dart';
import 'remote_widgets.dart';
import 'widgets.dart';

/// 电视首页（0.8.3）。
///
/// 为什么要单独做一版：
///
/// 竖滑短视频流是**手机**的交互——上下滑换剧，靠的是手指。
/// 搬到电视上之后，遥控器上下键只能"下一部/上一部"，
/// 用户永远不知道剧库里还有什么，也回不到刚才看过的那部，
/// 而且一进应用就开始播（电视上很吵、很吓人）。
///
/// 电视的浏览习惯是**横向海报行**：一行一个主题，左右移动挑片，
/// 上下切主题。所以这里给电视单独一页：
///
/// 1. 「开始刷剧」——保留原来的竖滑流，作为"我就想随便看"的入口；
/// 2. 「继续观看」——按观看记录排序，一键续播；
/// 3. 「热门短剧」——当前站源剧库；
/// 4. 分类行——按剧类分组，最多 4 行。
///
/// 竖滑流本身没有删掉，只是从"首页"变成了"刷剧模式"（[onStartFeed]）。
class TvHomeScreen extends StatefulWidget {
  const TvHomeScreen({
    super.key,
    required this.repository,
    required this.items,
    required this.history,
    required this.loading,
    required this.onOpen,
    required this.onResume,
    required this.onStartFeed,
    required this.onBrowse,
    required this.onSearch,
    required this.onRefresh,
    this.searchQuery = '',
    this.onClearSearch,
    this.emptyMessage,
    this.onContentReady,
  });

  final AppRepository repository;

  /// 当前站源剧库（已按站源权限过滤）。
  final List<Drama> items;

  /// 观看记录（已按时间倒序）。
  final List<WatchEntry> history;

  /// 是否正在加载第一批数据。
  final bool loading;

  /// 无站源 / 加载失败时的提示。
  final String? emptyMessage;

  final void Function(Drama drama) onOpen;
  final void Function(WatchEntry entry) onResume;

  /// 进入竖滑"刷剧"模式。
  final VoidCallback onStartFeed;

  /// 切到「发现」页浏览全部。
  final VoidCallback onBrowse;

  /// 打开搜索（0.8.4）：电视上搜索是主入口之一，不该先按左导航进「发现」。
  final VoidCallback onSearch;

  final VoidCallback onRefresh;

  /// 当前生效的拼音/首字母关键词（0.8.4），空串表示不在搜索态。
  ///
  /// 非空时首页整页切换成"搜索结果"，不再显示海报行和分类行——
  /// 否则搜索结果会混在「热门短剧」里，电视用户根本判断不出搜到没有。
  final String searchQuery;

  /// 清除搜索、回到常规首页。
  final VoidCallback? onClearSearch;

  /// 首批内容渲染完成（用于收起开屏页）。
  final VoidCallback? onContentReady;

  @override
  State<TvHomeScreen> createState() => _TvHomeScreenState();
}

class _TvHomeScreenState extends State<TvHomeScreen> {
  bool _reportedReady = false;

  @override
  void initState() {
    super.initState();
    _reportIfReady();
  }

  @override
  void didUpdateWidget(covariant TvHomeScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    _reportIfReady();
  }

  /// 有内容可看时通知外层收起开屏页。
  ///
  /// 手机上这个信号来自竖滑流的 `onReady`；电视首页不再实例化竖滑流，
  /// 不补这一下开屏页就只能等 3 秒超时。
  ///
  /// 必须推迟到帧后：`initState` / `didUpdateWidget` 都发生在 build 阶段，
  /// 此时回调会直接改外层的 `ValueNotifier` → 开屏页 `setState`
  /// → 触发 "setState() or markNeedsBuild() called during build"。
  void _reportIfReady() {
    if (_reportedReady || widget.loading) return;
    if (widget.items.isEmpty && widget.history.isEmpty) return;
    _reportedReady = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) widget.onContentReady?.call();
    });
  }

  @override
  Widget build(BuildContext context) {
    if (widget.searchQuery.isNotEmpty) return _searchResults();
    if (widget.items.isEmpty && widget.history.isEmpty) {
      return _placeholder();
    }
    final rows = <Widget>[
      _hero(),
      if (widget.history.isNotEmpty) _historyRow(),
      if (widget.items.isNotEmpty)
        _posterRow(
          title: '热门短剧',
          items: widget.items.take(20).toList(),
          onMore: widget.onBrowse,
        ),
      ..._categoryRows(),
    ];
    return ListView.separated(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 24),
      itemCount: rows.length,
      separatorBuilder: (_, _) => const SizedBox(height: 18),
      itemBuilder: (_, index) => rows[index],
    );
  }

  Widget _placeholder() => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.loading)
          const CircularProgressIndicator(strokeWidth: 3)
        else ...[
          const Icon(Icons.live_tv_outlined, size: 44, color: Colors.white54),
          const SizedBox(height: 12),
          Text(
            widget.emptyMessage ?? '还没有可播放的内容',
            style: const TextStyle(fontSize: 15, color: Colors.white70),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          RemoteButton(
            label: '重新加载',
            icon: Icons.refresh_rounded,
            autofocus: true,
            onPressed: widget.onRefresh,
          ),
        ],
      ],
    ),
  );

  /// 顶部大卡：开始刷剧 / 搜索 / 浏览全部。
  Widget _hero() => Row(
    children: [
      Expanded(
        flex: 3,
        child: _heroCard(
          key: const ValueKey('tv-home-start-feed'),
          autofocus: true,
          onPressed: widget.onStartFeed,
          label: '开始刷剧，进入竖滑短视频流',
          icon: Icons.play_circle_fill_rounded,
          iconColor: const Color(0xFFFF765F),
          iconSize: 38,
          title: '开始刷剧',
          subtitle: '上下键换剧/集，左右键快进，OK 播放暂停，长按 OK 3 倍速',
          accent: const Color(0xFF3A1F1A),
          titleSize: 20,
        ),
      ),
      const SizedBox(width: 12),
      Expanded(
        flex: 2,
        child: _heroCard(
          key: const ValueKey('tv-home-search'),
          onPressed: widget.onSearch,
          label: '搜索短剧，支持拼音/首字母',
          icon: Icons.search_rounded,
          iconSize: 30,
          title: '搜索',
          subtitle: '剧名或拼音/首字母',
          titleSize: 17,
        ),
      ),
      const SizedBox(width: 12),
      Expanded(
        flex: 2,
        child: _heroCard(
          key: const ValueKey('tv-home-browse'),
          onPressed: widget.onBrowse,
          label: '浏览全部短剧',
          icon: Icons.grid_view_rounded,
          iconSize: 30,
          title: '浏览全部',
          subtitle: '按分类、榜单挑片',
          titleSize: 17,
        ),
      ),
    ],
  );

  /// 首页顶部的一张卡（0.8.4 抽出：三张卡共用同一套尺寸与配色）。
  Widget _heroCard({
    required Key key,
    required VoidCallback onPressed,
    required String label,
    required IconData icon,
    required String title,
    required String subtitle,
    required double titleSize,
    double iconSize = 44,
    Color? iconColor,
    Color accent = const Color(0xFF1C1D22),
    bool autofocus = false,
  }) => RemoteTarget(
    key: key,
    autofocus: autofocus,
    onPressed: onPressed,
    label: label,
    child: Container(
      height: 84,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: accent,
      ),
      child: Row(
        children: [
          Icon(icon, size: iconSize, color: iconColor),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: titleSize,
                    fontWeight: FontWeight.w800,
                    color: Colors.white,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.white.withValues(alpha: .62),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    ),
  );

  /// 搜索结果页（0.8.4）。
  ///
  /// 用 `Wrap` 而不是单行横向列表：遥控器在 `Wrap` 里上下左右都能按几何位置
  /// 找邻居，而横向 `ListView` 只有左右可用，搜出几十部时得一路按到头。
  Widget _searchResults() {
    final items = widget.items.take(40).toList();
    final subtitle = items.isEmpty
        ? '换个关键词再试，或按「返回」回到首页'
        : '支持子序列：hjdnz / hdz / hnz 都能搜到「黄金大逆转」';
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 14, 20, 24),
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                items.isEmpty
                    ? '「${widget.searchQuery}」没有找到匹配的短剧'
                    : '「${widget.searchQuery}」· 找到 ${items.length} 部',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 17,
                  fontWeight: FontWeight.w800,
                  color: Colors.white,
                ),
              ),
            ),
            if (widget.onClearSearch != null)
              RemoteTarget(
                key: const ValueKey('tv-search-clear'),
                // 一部都没搜到时焦点直接落在这里，用户按确定就能退出搜索态，
                // 不会卡在一个空页面上找不到出口。
                autofocus: items.isEmpty,
                onPressed: widget.onClearSearch!,
                label: '清除搜索，返回电视首页',
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 7,
                  ),
                  decoration: BoxDecoration(
                    color: const Color(0xFF1C1D22),
                    borderRadius: BorderRadius.circular(9),
                  ),
                  child: const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.close_rounded, size: 15, color: Colors.white),
                      SizedBox(width: 5),
                      Text(
                        '清除搜索',
                        style: TextStyle(fontSize: 13, color: Colors.white),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ),
        const SizedBox(height: 6),
        Text(
          subtitle,
          style: TextStyle(
            fontSize: 12,
            color: Colors.white.withValues(alpha: .6),
          ),
        ),
        const SizedBox(height: 14),
        if (items.isNotEmpty)
          Wrap(
            spacing: 10,
            runSpacing: 12,
            children: [
              for (var index = 0; index < items.length; index++)
                _poster(
                  key: ValueKey(
                    'tv-search-${items[index].source}-${items[index].id}',
                  ),
                  drama: items[index],
                  autofocus: index == 0,
                  subtitle: items[index].episodes > 0
                      ? '共 ${items[index].episodes} 集'
                      : null,
                  onPressed: () => widget.onOpen(items[index]),
                ),
            ],
          ),
      ],
    );
  }

  Widget _historyRow() {
    final entries = widget.history.take(20).toList();
    return _section(
      title: '继续观看',
      child: SizedBox(
        height: _posterHeight(context),
        child: ListView.separated(
          scrollDirection: Axis.horizontal,
          itemCount: entries.length,
          separatorBuilder: (_, _) => const SizedBox(width: 10),
          itemBuilder: (_, index) {
            final entry = entries[index];
            final percent = entry.duration <= 0
                ? 0.0
                : (entry.position / entry.duration).clamp(0.0, 1.0);
            return _poster(
              key: ValueKey('tv-continue-${entry.drama.id}'),
              drama: entry.drama,
              subtitle: entry.finished
                  ? '已看完 · 第 ${entry.episode} 集'
                  : '第 ${entry.episode} 集 · ${(percent * 100).round()}%',
              progress: percent,
              onPressed: () => widget.onResume(entry),
            );
          },
        ),
      ),
    );
  }

  Widget _posterRow({
    required String title,
    required List<Drama> items,
    VoidCallback? onMore,
  }) => _section(
    title: title,
    onMore: onMore,
    child: SizedBox(
      height: _posterHeight(context),
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: items.length,
        separatorBuilder: (_, _) => const SizedBox(width: 10),
        itemBuilder: (_, index) {
          final drama = items[index];
          return _poster(
            key: ValueKey('tv-poster-${drama.source}-${drama.id}'),
            drama: drama,
            subtitle: drama.episodes > 0 ? '共 ${drama.episodes} 集' : null,
            onPressed: () => widget.onOpen(drama),
          );
        },
      ),
    ),
  );

  /// 按剧类分组，最多 4 行，每行最多 20 部。
  List<Widget> _categoryRows() {
    final groups = <String, List<Drama>>{};
    for (final drama in widget.items) {
      final name = categoryName(drama.category);
      if (name.isEmpty) continue;
      groups.putIfAbsent(name, () => []).add(drama);
    }
    final ordered = groups.entries.toList()
      ..sort((a, b) => b.value.length.compareTo(a.value.length));
    return [
      for (final group in ordered.take(4))
        if (group.value.length >= 4)
          _posterRow(title: group.key, items: group.value.take(20).toList()),
    ];
  }

  Widget _section({
    required String title,
    required Widget child,
    VoidCallback? onMore,
  }) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Row(
        children: [
          Expanded(
            child: Text(
              title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 17,
                fontWeight: FontWeight.w800,
                color: Colors.white,
              ),
            ),
          ),
          if (onMore != null)
            RemoteTarget(
              onPressed: onMore,
              label: '$title，查看全部',
              child: const Padding(
                padding: EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                child: Text(
                  '查看全部',
                  style: TextStyle(fontSize: 13, color: Colors.white70),
                ),
              ),
            ),
        ],
      ),
      const SizedBox(height: 8),
      child,
    ],
  );

  static const double _posterWidth = 128;
  static const double _posterTitleSize = 13;
  static const double _posterSubtitleSize = 11;

  /// 海报行高度按"封面 2:3 + 两行文字 + 焦点框"反算。
  ///
  /// 写死高度会让封面被 `BoxFit.cover` 裁掉上下两端（标题、主演名字都没了），
  /// 或者反过来比格子高、底部文字被裁。1.45 是 CJK 字体的行高上界。
  static double _posterHeight(BuildContext context) {
    final scaler = MediaQuery.textScalerOf(context);
    return ((_posterWidth - RemoteTarget.chrome) * 1.5 +
            4 +
            RemoteTarget.chrome +
            scaler.scale(_posterTitleSize) * 1.45 +
            scaler.scale(_posterSubtitleSize) * 1.45)
        .ceilToDouble();
  }

  Widget _poster({
    required Key key,
    required Drama drama,
    required VoidCallback onPressed,
    String? subtitle,
    double? progress,
    bool autofocus = false,
  }) => SizedBox(
    key: key,
    width: _posterWidth,
    height: _posterHeight(context),
    child: RemoteTarget(
      autofocus: autofocus,
      onPressed: onPressed,
      label: subtitle == null ? drama.title : '${drama.title}，$subtitle',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(6),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  DramaCover(drama: drama, repository: widget.repository),
                  if (progress != null)
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: LinearProgressIndicator(
                        value: progress,
                        minHeight: 3,
                        backgroundColor: Colors.black45,
                        color: const Color(0xFFFF765F),
                      ),
                    ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            drama.title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              fontSize: _posterTitleSize,
              fontWeight: FontWeight.w600,
              color: Colors.white,
            ),
          ),
          if (subtitle != null)
            Text(
              subtitle,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: _posterSubtitleSize,
                color: Colors.white.withValues(alpha: .55),
              ),
            ),
        ],
      ),
    ),
  );
}
