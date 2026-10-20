import 'package:duanju_app/app_bottom_navigation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Widget host({
    required ValueChanged<int> onSelected,
    ValueChanged<int>? onDoubleTap,
  }) {
    return MaterialApp(
      home: Scaffold(
        body: const SizedBox.expand(),
        bottomNavigationBar: AppBottomNavigation(
          selectedIndex: 0,
          onDestinationSelected: onSelected,
          onDestinationDoubleTap: onDoubleTap,
          destinations: const [
            NavigationDestination(
              icon: Icon(Icons.home_outlined),
              selectedIcon: Icon(Icons.home),
              label: '首页',
            ),
            NavigationDestination(
              icon: Icon(Icons.explore_outlined),
              selectedIcon: Icon(Icons.explore),
              label: '发现',
            ),
          ],
        ),
      ),
    );
  }

  testWidgets('single tap switches destination without waiting for double tap', (
    tester,
  ) async {
    final selected = <int>[];
    var doubles = 0;
    await tester.pumpWidget(
      host(
        onSelected: selected.add,
        onDoubleTap: (_) => doubles++,
      ),
    );

    // 这里故意不 pump kDoubleTapTimeout。旧实现把 onTap/onDoubleTap 同时
    // 交给 InkWell 时，单击会等待双击手势竞争结束；现在应在 pointer up 后
    // 立刻切页。
    await tester.tap(find.byKey(const ValueKey('bottom-nav-1')));
    expect(selected, [1]);
    expect(doubles, 0);
  });

  testWidgets('two quick taps keep immediate taps and still trigger home double tap', (
    tester,
  ) async {
    final selected = <int>[];
    final doubles = <int>[];
    await tester.pumpWidget(
      host(
        onSelected: selected.add,
        onDoubleTap: doubles.add,
      ),
    );

    final home = find.byKey(const ValueKey('bottom-nav-0'));
    await tester.tap(home);
    expect(selected, [0]);
    expect(doubles, isEmpty);

    await tester.pump(const Duration(milliseconds: 80));
    await tester.tap(home);
    expect(selected, [0, 0]);
    expect(doubles, [0]);
  });

  testWidgets('double tap window expires and different destinations do not collide', (
    tester,
  ) async {
    final doubles = <int>[];
    await tester.pumpWidget(
      host(
        onSelected: (_) {},
        onDoubleTap: doubles.add,
      ),
    );

    await tester.tap(find.byKey(const ValueKey('bottom-nav-0')));
    await tester.pump(const Duration(milliseconds: 400));
    await tester.tap(find.byKey(const ValueKey('bottom-nav-0')));
    expect(doubles, isEmpty);

    await tester.tap(find.byKey(const ValueKey('bottom-nav-1')));
    expect(doubles, isEmpty);
  });
}
