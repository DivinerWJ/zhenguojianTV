import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/vertical_feed_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'player_fixtures.dart';

class _Repo extends FixtureRepository {
  int recommendationCalls = 0;

  @override
  Future<CatalogPage> recommendations(
    String genre, {
    bool more = false,
    bool force = false,
  }) async {
    recommendationCalls++;
    return CatalogPage(const [
      Drama(id: 'hongguo:1', source: 'hongguo', title: '性能测试剧'),
    ]);
  }
}

void main() {
  testWidgets(
    'hidden phone feed defers network work until the tab becomes active',
    (tester) async {
      SharedPreferences.setMockInitialValues(const {});
      final store = LocalStore(await SharedPreferences.getInstance());
      addTearDown(store.dispose);
      final repo = _Repo();
      var active = false;
      late StateSetter change;

      await tester.pumpWidget(
        MaterialApp(
          home: StatefulBuilder(
            builder: (context, setState) {
              change = setState;
              return VerticalFeedScreen(
                repository: repo,
                store: store,
                embedded: false,
                active: active,
                sources: const [SourceSite.hongguo],
                playerFactory: () => Player(platformPlayer: ScriptedPlayer()),
                videoBuilder: (_) => const SizedBox.expand(),
              );
            },
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));
      expect(repo.recommendationCalls, 0);

      change(() => active = true);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));
      expect(repo.recommendationCalls, greaterThan(0));
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump(const Duration(seconds: 1));
    },
  );
}
