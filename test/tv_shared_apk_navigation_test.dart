import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/main.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'remote_test_helpers.dart';

void main() {
  Future<LocalStore> makeStore() async {
    SharedPreferences.setMockInitialValues({});
    final store = LocalStore(await SharedPreferences.getInstance());
    addTearDown(store.dispose);
    return store;
  }

  void televisionViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(960, 540);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  testWidgets(
    'the shared APK exposes every main tab to a TV remote without phone navigation',
    (tester) async {
      televisionViewport(tester);
      await tester.pumpWidget(
        DuanjuApp(
          repository: FixtureRepository(),
          store: await makeStore(),
          television: true,
        ),
      );
      await tester.pumpAndSettle();

      // 同一 APK 在 TV 模式下必须走 TV 导航，而不是把手机底栏强行放大复用。
      for (var index = 0; index < 5; index++) {
        expect(find.byKey(ValueKey('tv-nav-$index')), findsOneWidget);
        expect(find.byKey(ValueKey('bottom-nav-$index')), findsNothing);
      }

      // 五个主入口都必须只靠方向键/确认键可达，切换期间不能抛异常。
      for (var index = 0; index < 5; index++) {
        final target = find.byKey(ValueKey('tv-nav-$index'));
        focusRemote(tester, target);
        await tester.pumpAndSettle();
        await tester.sendKeyEvent(LogicalKeyboardKey.select);
        await tester.pumpAndSettle();
        expect(target, findsOneWidget);
        expect(tester.takeException(), isNull);
      }

      // 回到首页后 TV 导航仍存在，证明页签切换没有把 TV shell 重建丢失。
      final home = find.byKey(const ValueKey('tv-nav-0'));
      focusRemote(tester, home);
      await tester.sendKeyEvent(LogicalKeyboardKey.select);
      await tester.pumpAndSettle();
      expect(home, findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}
