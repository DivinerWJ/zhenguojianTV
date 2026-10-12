import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/vertical_feed_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'danmaku_fixtures.dart';
import 'fixtures.dart';
import 'player_fixtures.dart';

void main() {
  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 15; i++) {
      await tester.pump(const Duration(milliseconds: 20));
    }
  }

  /// 挂载"单剧模式"的竖滑播放页（详情页/下载页点播放走的就是这条路径）。
  ///
  /// 单剧模式会跳过推荐流拉取，因此测试不需要首页目录数据。
  Future<void> pumpFeed(
    WidgetTester tester, {
    required DanmakuFixtureRepository repository,
    required ScriptedPlayer player,
    required LocalStore store,
    required bool active,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData.dark(),
        home: VerticalFeedScreen(
          repository: repository,
          store: store,
          embedded: true,
          singleDrama: FixtureRepository.free,
          active: active,
          playerFactory: () => Player(platformPlayer: player),
        ),
      ),
    );
    await settle(tester);
  }

  Future<(LocalStore, DanmakuFixtureRepository, ScriptedPlayer)> setUpFeed(
    WidgetTester tester, {
    required bool active,
  }) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(420, 880);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    SharedPreferences.setMockInitialValues(const {});
    final store = LocalStore(await SharedPreferences.getInstance());
    addTearDown(store.dispose);
    final repository = DanmakuFixtureRepository();
    final player = ScriptedPlayer();
    await pumpFeed(
      tester,
      repository: repository,
      player: player,
      store: store,
      active: active,
    );
    return (store, repository, player);
  }

  testWidgets(
    'feed player exposes a danmaku switch with status and persists it',
    (tester) async {
      final (store, _, _) = await setUpFeed(tester, active: true);

      // 弹幕入口必须存在（修复前新播放器完全没有这个入口）。
      expect(find.byKey(const ValueKey('feed-danmaku')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('feed-danmaku')));
      await settle(tester);
      expect(find.byKey(const ValueKey('feed-danmaku-sheet')), findsOneWidget);

      // 开关初始为开（偏好默认值），切换后写入偏好并持久化。
      final toggle = find.byKey(const ValueKey('feed-danmaku-enabled'));
      expect(toggle, findsOneWidget);
      await tester.tap(toggle);
      await settle(tester);
      expect(store.playbackPreferences.danmaku, isFalse);

      await tester.tap(toggle);
      await settle(tester);
      expect(store.playbackPreferences.danmaku, isTrue);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('an inactive feed page never drives a player, and loads once it '
      'becomes active', (tester) async {
    final (store, repository, player) = await setUpFeed(tester, active: false);

    // 未激活的页面不允许创建播放器（`_ensurePlayer` 的硬前置）。
    expect(player.opened, isEmpty);
    expect(player.disposed, isFalse);

    // 变为当前页后正常加载，且只加载一次（防止"释放后重复 open"这类回归）。
    await pumpFeed(
      tester,
      repository: repository,
      player: player,
      store: store,
      active: true,
    );
    expect(player.opened, hasLength(1));
    expect(player.disposed, isFalse);
    expect(tester.takeException(), isNull);
  });
}
