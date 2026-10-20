import 'package:duanju_app/downloads_screen.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';

class _PollingRepository extends FixtureRepository {
  int downloadCalls = 0;

  @override
  bool get supportsDownloads => true;

  @override
  Future<List<DownloadJob>> downloads() async {
    downloadCalls++;
    return const [];
  }
}

void main() {
  testWidgets(
    'hidden DownloadsScreen does not poll and resumes only while active',
    (tester) async {
      SharedPreferences.setMockInitialValues(const {});
      final store = LocalStore(await SharedPreferences.getInstance());
      addTearDown(store.dispose);
      final repository = _PollingRepository();
      final active = ValueNotifier<bool>(false);
      addTearDown(active.dispose);

      await tester.pumpWidget(
        MaterialApp(
          home: ValueListenableBuilder<bool>(
            valueListenable: active,
            builder: (context, value, child) => DownloadsScreen(
              repository: repository,
              store: store,
              active: value,
            ),
          ),
        ),
      );
      await tester.pump(const Duration(seconds: 3));
      expect(repository.downloadCalls, 0);

      active.value = true;
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(repository.downloadCalls, 1);

      await tester.pump(const Duration(seconds: 2));
      await tester.pump(const Duration(milliseconds: 50));
      expect(repository.downloadCalls, greaterThanOrEqualTo(2));

      active.value = false;
      await tester.pump();
      final pausedAt = repository.downloadCalls;
      await tester.pump(const Duration(seconds: 3));
      expect(repository.downloadCalls, pausedAt);
    },
  );
}
