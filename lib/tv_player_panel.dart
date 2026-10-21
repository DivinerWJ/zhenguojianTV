import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'models.dart';
import 'remote_widgets.dart';

enum TvPlayerAction { episodes, speed, quality, follow, autoNext, detail, push }

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
  padding: const EdgeInsets.fromLTRB(20, 16, 20, 12),
  child: Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(
        title,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 19,
          fontWeight: FontWeight.w800,
        ),
      ),
      const SizedBox(height: 3),
      Text(
        subtitle,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(
          color: Colors.white.withValues(alpha: .62),
          fontSize: 12,
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
  required bool canToggleAutoNext,
  required bool autoNextEpisode,
  required bool canOpenDetail,
  required bool canPush,
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
    if (canToggleAutoNext)
      (
        TvPlayerAction.autoNext,
        autoNextEpisode ? Icons.repeat_one_rounded : Icons.skip_next_rounded,
        '连播模式',
        autoNextEpisode ? '当前：连播本剧' : '当前：连播下一部',
      ),
    (
      TvPlayerAction.follow,
      followed ? Icons.bookmark_rounded : Icons.bookmark_border_rounded,
      followed ? '取消追剧' : '追剧',
      followed ? '已加入追剧列表' : '加入追剧列表',
    ),
    if (canOpenDetail)
      (
        TvPlayerAction.detail,
        Icons.info_outline_rounded,
        '剧集详情',
        '查看简介、剧集和更多信息',
      ),
    if (canPush)
      (TvPlayerAction.push, Icons.cast_rounded, '推送播放', '发送到局域网内的另一台设备'),
  ];
  final focusNodes = List<FocusNode>.generate(
    items.length,
    (index) => FocusNode(debugLabel: 'tv-player-menu-$index'),
  );
  final dialog = _showTvSidePanel<TvPlayerAction>(
    context,
    barrierLabel: '关闭播放设置',
    width: 380,
    builder: (panelContext) => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _panelHeader('播放设置', title),
        Divider(height: 1, color: Colors.white.withValues(alpha: .10)),
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(14, 12, 14, 18),
            child: Column(
              children: [
                for (var index = 0; index < items.length; index++) ...[
                  CallbackShortcuts(
                    bindings: {
                      const SingleActivator(LogicalKeyboardKey.arrowUp): () {
                        if (index > 0) focusNodes[index - 1].requestFocus();
                      },
                      const SingleActivator(LogicalKeyboardKey.arrowDown): () {
                        if (index + 1 < focusNodes.length) {
                          focusNodes[index + 1].requestFocus();
                        }
                      },
                    },
                    child: RemoteTarget(
                      key: ValueKey('tv-player-menu-${items[index].$1.name}'),
                      focusNode: focusNodes[index],
                      autofocus: index == 0,
                      accent: _tvAccent,
                      label: '${items[index].$3}，${items[index].$4}',
                      onPressed: () =>
                          Navigator.of(panelContext).pop(items[index].$1),
                      child: Container(
                        constraints: const BoxConstraints(minHeight: 62),
                        padding: const EdgeInsets.symmetric(
                          horizontal: 14,
                          vertical: 9,
                        ),
                        decoration: BoxDecoration(
                          color: Colors.white.withValues(alpha: .07),
                          borderRadius: BorderRadius.circular(14),
                        ),
                        child: Row(
                          children: [
                            Container(
                              width: 36,
                              height: 36,
                              decoration: BoxDecoration(
                                color: _tvAccent.withValues(alpha: .15),
                                borderRadius: BorderRadius.circular(10),
                              ),
                              child: Icon(
                                items[index].$2,
                                color: _tvAccent,
                                size: 21,
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                mainAxisSize: MainAxisSize.min,
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    items[index].$3,
                                    style: const TextStyle(
                                      color: Colors.white,
                                      fontSize: 16,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  const SizedBox(height: 3),
                                  Text(
                                    items[index].$4,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: TextStyle(
                                      color: Colors.white.withValues(
                                        alpha: .58,
                                      ),
                                      fontSize: 12,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            Icon(
                              Icons.chevron_right_rounded,
                              size: 18,
                              color: Colors.white.withValues(alpha: .48),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                  if (index + 1 < items.length) const SizedBox(height: 8),
                ],
              ],
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 16),
          child: Text(
            'OK 播放/暂停 · 长按 OK 3 倍速 · ←/→ 快退快进 · 菜单键设置 · 返回逐层退出',
            style: TextStyle(
              color: Colors.white.withValues(alpha: .42),
              fontSize: 11,
            ),
          ),
        ),
      ],
    ),
  );
  return dialog.whenComplete(() {
    for (final node in focusNodes) {
      node.dispose();
    }
  });
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
    width: 480,
    builder: (panelContext) => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _panelHeader(
          '选集',
          '$title · 当前第 ${episodes[safeIndex].number} 集 · 上下滚动',
        ),
        Divider(height: 1, color: Colors.white.withValues(alpha: .10)),
        Expanded(
          child: RemoteGrid(
            itemKeys: keys,
            columns: 3,
            itemExtent: 54,
            spacing: 10,
            padding: const EdgeInsets.fromLTRB(16, 14, 16, 20),
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
                        const Icon(
                          Icons.play_arrow_rounded,
                          color: _tvAccent,
                          size: 17,
                        ),
                        const SizedBox(width: 2),
                      ],
                      Flexible(
                        child: Text(
                          '第 ${episode.number} 集',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 14,
                            fontWeight: current
                                ? FontWeight.w800
                                : FontWeight.w600,
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
