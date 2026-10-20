import 'package:duanju_app/app_layout.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/vertical_feed_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'player_fixtures.dart';

const _drama = Drama(
  id: 'hongguo:tv-stable',
  source: 'hongguo',
  title: '电视稳定性测试剧',
  episodes: 3,
);

class _TvRepo extends FixtureRepository {
  final requestedEpisodes = <int>[];

  @override
  Future<CatalogPage> recommendations(
    String genre, {
    bool more = false,
    bool force = false,
  }) async => CatalogPage(const [_drama]);

  @override
  Future<DramaDetail> detail(Drama drama) async => DramaDetail(
    drama,
    List.generate(
      3,
      (index) => Episode({
        'id': '${index + 1}',
        'title': '第${index + 1}集',
        'currentEpisode': index + 1,
      }, index + 1),
    ),
  );

  @override
  Future<PlaybackPlan> resolve(
    Drama drama,
    Episode episode, {
    int quality = 0,
  }) async {
    requestedEpisodes.add(episode.number);
    return PlaybackPlan(
      url: 'https://example.test/tv-${episode.number}.mp4',
      session: 'tv-${episode.number}-${requestedEpisodes.length}',
      qualities: const [1080, 720],
      quality: quality == 0 ? 1080 : quality,
    );
  }
}

Future<void> _pumpFrames(WidgetTester tester, [int frames = 50]) async {
  for (var i = 0; i < frames; i++) {
    await tester.pump(const Duration(milliseconds: 20));
  }
}

Future<(LocalStore, _TvRepo, ScriptedPlayer)> _mount(
  WidgetTester tester, {
  bool singleDrama = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(1280, 720);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  SharedPreferences.setMockInitialValues(const {});
  final store = LocalStore(await SharedPreferences.getInstance());
  addTearDown(store.dispose);
  final repo = _TvRepo();
  final platform = ScriptedPlayer();

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData.dark(),
      home: Shortcuts(
        shortcuts: const {
          SingleActivator(LogicalKeyboardKey.select, includeRepeats: false):
              ActivateIntent(),
          SingleActivator(
            LogicalKeyboardKey.gameButtonA,
            includeRepeats: false,
          ): ActivateIntent(),
          SingleActivator(LogicalKeyboardKey.goBack): DismissIntent(),
        },
        child: AppLayout(
          television: true,
          version: '0.8.6',
          child: VerticalFeedScreen(
            repository: repo,
            store: store,
            singleDrama: singleDrama ? _drama : null,
            playerFactory: () => Player(platformPlayer: platform),
            videoBuilder: (_) => const ColoredBox(color: Colors.black),
          ),
        ),
      ),
    ),
  );
  await _pumpFrames(tester);
  expect(repo.requestedEpisodes, isNotEmpty);
  expect(platform.state.playing, isTrue);
  return (store, repo, platform);
}

void main() {
  testWidgets(
    'TV short OK toggles once and moves focus into visible controls',
    (tester) async {
      final (_, _, platform) = await _mount(tester);
      expect(FocusManager.instance.primaryFocus?.debugLabel, 'vertical-feed');

      await tester.sendKeyDownEvent(LogicalKeyboardKey.select);
      await tester.pump(const Duration(milliseconds: 100));
      expect(
        platform.state.playing,
        isTrue,
        reason: 'KeyDown must not pause before long-press is decided',
      );

      await tester.sendKeyUpEvent(LogicalKeyboardKey.select);
      await tester.pumpAndSettle();
      expect(platform.state.playing, isFalse);
      expect(
        FocusManager.instance.primaryFocus?.debugLabel,
        'feed-play-button',
        reason: '精简遥控器没有菜单键时，短按 OK 后仍必须能进入控件焦点链',
      );

      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(FocusManager.instance.primaryFocus?.debugLabel, 'vertical-feed');
    },
  );

  testWidgets(
    'TV long OK enters 3x and release restores without toggling playback',
    (tester) async {
      final (_, _, platform) = await _mount(tester);

      await tester.sendKeyDownEvent(LogicalKeyboardKey.select);
      await tester.pump(const Duration(milliseconds: 420));
      expect(platform.rates, isNotEmpty);
      expect(platform.rates.last, 3);
      expect(platform.state.playing, isTrue);
      expect(FocusManager.instance.primaryFocus?.debugLabel, 'vertical-feed');

      await tester.sendKeyUpEvent(LogicalKeyboardKey.select);
      await tester.pump(const Duration(milliseconds: 120));
      expect(platform.rates.last, 1);
      expect(platform.state.playing, isTrue, reason: '长按松手只能恢复倍速，不能额外触发一次暂停');
    },
  );

  testWidgets(
    'TV single-drama up/down changes episodes instead of dead-ending',
    (tester) async {
      final (_, repo, _) = await _mount(tester);
      expect(repo.requestedEpisodes.last, 1);

      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await _pumpFrames(tester, 20);
      expect(repo.requestedEpisodes.last, 2);

      // 竖滑/遥控切集共用 400ms 去重窗口。
      await tester.pump(const Duration(milliseconds: 450));
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowUp);
      await _pumpFrames(tester, 20);
      expect(repo.requestedEpisodes.last, 1);
    },
  );

  testWidgets('TV left/right seek works from root playback focus', (
    tester,
  ) async {
    final (_, _, platform) = await _mount(tester);
    await platform.seek(const Duration(seconds: 30));

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
    await tester.pump();
    expect(platform.state.position, const Duration(seconds: 40));

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
    await tester.pump();
    expect(platform.state.position, const Duration(seconds: 30));
  });

  testWidgets('TV menu is reachable and exposes all core playback actions', (
    tester,
  ) async {
    await _mount(tester, singleDrama: false);

    await tester.sendKeyEvent(LogicalKeyboardKey.contextMenu);
    await tester.pumpAndSettle();
    expect(find.text('播放设置'), findsOneWidget);
    expect(find.text('选集'), findsWidgets);
    expect(find.text('倍速'), findsWidgets);
    expect(find.text('清晰度'), findsWidgets);
    expect(find.text('连播模式'), findsWidgets);
    expect(find.text('追剧'), findsWidgets);
    expect(find.text('剧集详情'), findsWidgets);
    expect(
      find.byKey(const ValueKey('tv-player-menu-detail')),
      findsOneWidget,
      reason: '低分辨率 TV 上剧集详情也必须预先存在于遥控焦点树',
    );

    // 第一项自动聚焦，OK 应进入选集抽屉；当前集必须自动定位并聚焦。
    await tester.sendKeyEvent(LogicalKeyboardKey.select);
    await tester.pumpAndSettle();
    expect(find.text('选集'), findsWidgets);
    expect(FocusManager.instance.primaryFocus?.debugLabel, 'remote-episode-0');

    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('播放设置'), findsNothing);
  });

  testWidgets('TV back hierarchy hides controls before exit confirmation', (
    tester,
  ) async {
    final (_, _, _) = await _mount(tester);

    // 短按 OK 进入控件焦点。
    await tester.sendKeyEvent(LogicalKeyboardKey.select);
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus?.debugLabel, 'feed-play-button');

    // 第一次系统返回只收起控件并还焦点给播放画面。
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('退出播放？'), findsNothing);
    expect(FocusManager.instance.primaryFocus?.debugLabel, 'vertical-feed');

    // 第二次系统返回才出现退出确认。
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('退出播放？'), findsOneWidget);
    expect(find.text('继续观看'), findsOneWidget);
  });
}
