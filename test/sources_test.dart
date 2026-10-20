import 'dart:async';
import 'dart:convert';

import 'package:duanju_app/local_profiles.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/source_status.dart';
import 'package:duanju_app/sources_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';

class SourceFixtureRepository extends FixtureRepository {
  final statuses = <String, SourceStatus>{};
  final operations = <String>[];
  final statusRequests = <String>[];
  Completer<SourceStatus>? pending;

  @override
  bool get supportsSourceManagement => true;

  @override
  Future<SourceStatus> sourceStatus(String source) async {
    statusRequests.add(source);
    return statuses[source] ??
        SourceStatus.fromJson({
          'source': source,
          'count': 3,
          'page': 2,
          'hasMore': true,
        });
  }

  @override
  Future<SourceStatus> startSourceJob(
    String source,
    String operation, {
    Drama? drama,
  }) async {
    operations.add('$source:$operation');
    if (pending != null) return pending!.future;
    return statuses[source] = SourceStatus.fromJson({
      'source': source,
      'count': 3,
      'running': true,
      'operation': operation,
      'stage': '查找新剧',
    });
  }

  @override
  Future<SourceStatus> cancelSourceJob(String source) async {
    operations.add('$source:cancel');
    return statuses[source] = SourceStatus.fromJson({
      'source': source,
      'count': 3,
      'running': false,
      'stage': '已停止',
    });
  }
}

void main() {
  testWidgets(
    'source update prevents duplicate submits and can stop without losing cache',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      final store = LocalStore(await SharedPreferences.getInstance());
      final repository = SourceFixtureRepository()
        ..pending = Completer<SourceStatus>();
      await tester.pumpWidget(
        MaterialApp(
          home: SourcesScreen(repository: repository, store: store),
        ),
      );
      await tester.pumpAndSettle();
      final update = find.byKey(const ValueKey('update-hongguo'));
      await tester.tap(update);
      await tester.pump();
      await tester.tap(update);
      expect(repository.operations, ['hongguo:update']);
      expect(tester.widget<FilledButton>(update).onPressed, isNull);
      repository.pending!.complete(
        SourceStatus.fromJson({
          'source': 'hongguo',
          'count': 3,
          'running': true,
          'stage': '查找新剧',
        }),
      );
      await tester.pump();
      await tester.tap(find.text('停止').first);
      await tester.pumpAndSettle();
      expect(repository.operations, ['hongguo:update', 'hongguo:cancel']);
      expect(find.text('已停止'), findsOneWidget);
      expect(find.text('3 部'), findsWidgets);
      await tester.pumpWidget(const SizedBox.shrink());
      store.dispose();
    },
  );

  testWidgets(
    'source manager requests only the current user permitted compiled sources',
    (tester) async {
      SharedPreferences.setMockInitialValues({
        'profiles': jsonEncode([
          LocalProfile(
            id: 'default',
            name: '管理员',
            admin: true,
            salt: '0' * 32,
            pinHash: '1' * 64,
          ).toJson(),
          const LocalProfile(
            id: 'viewer',
            name: '只看红果',
            sources: ['hongguo'],
            download: false,
          ).toJson(),
        ]),
        'activeProfile': 'viewer',
      });
      final store = LocalStore(await SharedPreferences.getInstance());
      final repository = SourceFixtureRepository();
      await tester.pumpWidget(
        MaterialApp(
          home: SourcesScreen(repository: repository, store: store),
        ),
      );
      await tester.pumpAndSettle();
      expect(repository.statusRequests, ['hongguo']);
      expect(find.byKey(const ValueKey('source-hongguo')), findsOneWidget);
      for (final source in SourceSite.knownValues.skip(1)) {
        expect(find.byKey(ValueKey('source-${source.id}')), findsNothing);
      }
      await tester.pumpWidget(const SizedBox.shrink());
      store.dispose();
    },
  );

  for (final operation in ['check', 'checkCatalog']) {
    testWidgets(
      'source diagnostics remain collapsed through $operation polling and completion',
      (tester) async {
        SharedPreferences.setMockInitialValues({});
        final store = LocalStore(await SharedPreferences.getInstance());
        SourceStatus status(String state, {bool running = false}) =>
            SourceStatus.fromJson({
              'source': 'hongguo',
              'count': 7,
              'operation': operation,
              'running': running,
              'stage': running ? '检测连接' : '检测完成',
              'health': {
                'state': state,
                'checkedAt': '2026-09-21T00:00:00Z',
                'sample': '合成检测剧集',
                'steps': [
                  {
                    'name': '入口与目录',
                    'state': 'ok',
                    'message': '目录可达',
                    'httpStatus': 200,
                  },
                ],
              },
            });
        final repository = SourceFixtureRepository()
          ..statuses['hongguo'] = status('ok')
          ..pending = Completer<SourceStatus>();
        await tester.pumpWidget(
          MaterialApp(
            home: SourcesScreen(repository: repository, store: store),
          ),
        );
        await tester.pumpAndSettle();
        final toggle = find.byKey(const ValueKey('health-toggle-hongguo'));
        final sample = find.text('检测剧集：合成检测剧集');
        expect(toggle, findsOneWidget);
        expect(sample, findsNothing);
        expect(find.byTooltip('复制诊断信息'), findsOneWidget);

        if (operation == 'check') {
          await tester.tap(find.byKey(const ValueKey('check-hongguo')));
        } else {
          await tester.tap(find.byTooltip('红果更多操作'));
          await tester.pumpAndSettle();
          await tester.tap(find.text('仅检测目录'));
        }
        await tester.pump();
        expect(repository.operations, ['hongguo:$operation']);
        expect(sample, findsOneWidget);
        await tester.ensureVisible(toggle);
        await tester.tap(toggle);
        await tester.pump();
        expect(sample, findsNothing);

        repository.pending!.complete(status('ok', running: true));
        await tester.pump();
        expect(sample, findsNothing);
        await tester.pump(const Duration(seconds: 2));
        expect(sample, findsNothing);
        await tester.pumpWidget(const SizedBox.shrink());
        store.dispose();
      },
    );
  }

  testWidgets('source diagnostics distinguish CF playback failure on a narrow screen', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(360, 760);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    SharedPreferences.setMockInitialValues({});
    final store = LocalStore(await SharedPreferences.getInstance());
    final repository = SourceFixtureRepository()
      ..statuses['hongguo'] = SourceStatus.fromJson({
        'source': 'hongguo',
        'count': 5,
        'health': {
          'state': 'error',
          'checkedAt': '2026-09-21T00:00:00Z',
          'sample': '检测剧集',
          'steps': [
            {
              'name': '播放地址',
              'state': 'error',
              'message': 'Cloudflare challenge / 403',
              'host': 'video.example.test',
              'httpStatus': 403,
              'cfRay': 'abc-TPE',
            },
          ],
        },
      });
    await tester.pumpWidget(
      MaterialApp(home: SourcesScreen(repository: repository, store: store)),
    );
    await tester.pumpAndSettle();
    expect(find.text('播放检测失败'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('health-toggle-hongguo')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Cloudflare'), findsOneWidget);
    expect(find.textContaining('403'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    store.dispose();
  });
}
