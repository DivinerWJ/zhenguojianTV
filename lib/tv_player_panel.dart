import 'package:flutter/material.dart';

import 'models.dart';
import 'remote_widgets.dart';

enum TvPlayerAction { episodes, speed, quality, follow }

const _tvAccent = Color(0xFFFF765F);

Future<T?> _showTvSidePanel<T>(
  BuildContext context, {
  required String barrierLabel,
  required WidgetBuilder builder,
  double width = 520,
}) => showGeneralDialog<T>(
  context: context,
  useRootNavigator: true,
  barrierDismissible: true,
  barrierLabel: barrierLabel,
  barrierColor: Colors.black.withValues(alpha: .56),
  transitionDuration: const Duration(milliseconds: 180),
  pageBuilder: (context, _, __) => Material(
    type: MaterialType.transparency,
    child: SafeArea(
      child: Align(
        alignment: Alignment.centerRight,
        child: Container(
          width: width,
          height: double.infinity,
          margin: const EdgeInsets.fromLTRB(0, 16, 18, 16),
          clipBehavior: Clip.antiAlias,
          decoration: BoxDecoration(
            color: const Color(0xFF17181D),
            borderRadius: BorderRadius.circular(24),
            border: Border.all(color: Colors.white.withValues(alpha: .14)),
            boxShadow: const [
              BoxShadow(color: Colors.black54, blurRadius: 36, spreadRadius: 4),
            ],
          ),
          child: builder(context),
        ),
      ),
    ),
  ),
  transitionBuilder: (context, animation, secondary, child) {
    final offset = Tween<Offset>(
      begin: const Offset(.12, 0),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: animation, curve: Curves.easeOutCubic));
    return FadeTransition(
      opacity: animation,
      child: SlideTransition(position: offset, child: child),
    );
  },
);

Widget _panelHeader(String title, String subtitle) => Padding(
  padding: const EdgeInsets.fromLTRB(28, 24, 28, 18),
  child: Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(
        title,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 26,
          fontWeight: FontWeight.w800,
        ),
      ),
      const SizedBox(height: 5),
      Text(
        subtitle,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          color: Colors.white.withValues(alpha: .62),
          fontSize: 15,
        ),
      ),
    ],
  ),
);

Future<TvPlayerAction?> showTvPlayerMenu(
  BuildContext context, {
  required String title,
  required int episode,
  required int totalEpisodes,
  required String speedLabel,
  required String qualityLabel,
  required bool followed,
}) {
  final items = <(TvPlayerAction, IconData, String, String)>[
    (
      TvPlayerAction.episodes,
      Icons.format_list_numbered_rounded,
      '选集',
      totalEpisodes > 0 ? '第 $episode 集 / 共 $totalEpisodes 集' : '选择播放集数',
    ),
    (TvPlayerAction.speed, Icons.speed_rounded, '倍速', '当前 $speedLabel'),
    (TvPlayerAction.quality, Icons.hd_rounded, '清晰度', qualityLabel),
    (
      TvPlayerAction.follow,
      followed ? Icons.bookmark_rounded : Icons.bookmark_border_rounded,
      followed ? '取消追剧' : '追剧',
      followed ? '已加入追剧列表' : '加入追剧列表',
    ),
  ];
  return _showTvSidePanel<TvPlayerAction>(
    context,
    barrierLabel: '关闭播放设置',
    width: 470,
    builder: (panelContext) => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _panelHeader('播放设置', title),
        Divider(height: 1, color: Colors.white.withValues(alpha: .10)),
        Expanded(
          child: ListView.separated(
            padding: const EdgeInsets.fromLTRB(18, 18, 18, 26),
            itemCount: items.length,
            separatorBuilder: (_, __) => const SizedBox(height: 10),
            itemBuilder: (_, index) {
              final item = items[index];
              return RemoteTarget(
                autofocus: index == 0,
                accent: _tvAccent,
                label: '${item.$3}，${item.$4}',
                onPressed: () => Navigator.of(panelContext).pop(item.$1),
                child: Container(
                  constraints: const BoxConstraints(minHeight: 82),
                  padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: .07),
                    borderRadius: BorderRadius.circular(14),
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 48,
                        height: 48,
                        decoration: BoxDecoration(
                          color: _tvAccent.withValues(alpha: .15),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Icon(item.$2, color: _tvAccent, size: 27),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              item.$3,
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 20,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            const SizedBox(height: 4),
                            Text(
                              item.$4,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                color: Colors.white.withValues(alpha: .58),
                                fontSize: 14,
                              ),
                            ),
                          ],
                        ),
                      ),
                      Icon(
                        Icons.chevron_right_rounded,
                        color: Colors.white.withValues(alpha: .48),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(28, 0, 28, 22),
          child: Text(
            'OK 播放/暂停  ·  ←/→ 快退快进  ·  菜单键打开本页  ·  返回关闭',
            style: TextStyle(color: Colors.white.withValues(alpha: .42), fontSize: 13),
          ),
        ),
      ],
    ),
  );
}

Future<int?> showTvEpisodePanel(
  BuildContext context, {
  required String title,
  required List<Episode> episodes,
  required int currentIndex,
}) {
  if (episodes.isEmpty) return Future<int?>.value();
  final safeIndex = currentIndex.clamp(0, episodes.length - 1);
  final keys = List<String>.generate(episodes.length, (i) => 'episode-$i');
  return _showTvSidePanel<int>(
    context,
    barrierLabel: '关闭选集',
    width: 610,
    builder: (panelContext) => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _panelHeader('选集', '$title · 当前第 ${episodes[safeIndex].number} 集 · 上下滚动'),
        Divider(height: 1, color: Colors.white.withValues(alpha: .10)),
        Expanded(
          child: RemoteGrid(
            itemKeys: keys,
            columns: 3,
            itemExtent: 72,
            spacing: 12,
            padding: const EdgeInsets.fromLTRB(22, 20, 22, 28),
            initialIndex: safeIndex,
            autofocus: true,
            itemBuilder: (_, index, node, onFocus) {
              final episode = episodes[index];
              final current = index == safeIndex;
              return RemoteTarget(
                focusNode: node,
                onFocus: onFocus,
                selected: current,
                accent: _tvAccent,
                label: '第 ${episode.number} 集${current ? '，正在播放' : ''}',
                onPressed: () => Navigator.of(panelContext).pop(index),
                child: Container(
                  alignment: Alignment.center,
                  padding: const EdgeInsets.symmetric(horizontal: 10),
                  decoration: BoxDecoration(
                    color: current
                        ? _tvAccent.withValues(alpha: .28)
                        : Colors.white.withValues(alpha: .07),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      if (current) ...[
                        const Icon(Icons.play_arrow_rounded, color: _tvAccent, size: 22),
                        const SizedBox(width: 3),
                      ],
                      Flexible(
                        child: Text(
                          '第 ${episode.number} 集',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 17,
                            fontWeight: current ? FontWeight.w800 : FontWeight.w600,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
      ],
    ),
  );
}
