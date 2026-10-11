import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'app_layout.dart';

/// 应用内更新检查。
///
/// 打开应用时向 GitHub 查询发布仓库的最新 Release 版本号，
/// 若高于当前版本则提示用户，用户点击后下载 APK 并拉起系统安装界面。
///
/// 设计约束（用户明确要求，不扩展）：
/// - 不接第三方推送服务，不做后台保活，不用服务器中转。
/// - 只在打开应用时检查一次。
/// - 网络失败静默跳过，不打扰用户。
/// - 用户选择“稍后”后，当天不再提示。
class AppUpdater {
  AppUpdater._();

  /// 发布仓库（用户自有仓库，README 已标明原作者 Topkill/guoapp3）。
  static const _owner = 'kuoluoss';
  static const _repo = 'zhenguojian';

  /// GitHub API 直连地址（国内可能不通，失败即静默跳过）。
  static const _apiUrl =
      'https://api.github.com/repos/$_owner/$_repo/releases/latest';

  /// 下载加速镜像前缀（GitHub 直连失败时尝试，用户此前用于其他下载且验证可用）。
  static const _mirrorPrefix = 'https://ghfast.top/';

  /// 记录用户点击“稍后”的日期，格式 yyyy-MM-dd。
  static const _snoozeKey = 'appUpdateSnoozedDate';

  /// 每天最多提示一次，避免反复打扰。
  static Future<bool> _shouldPromptToday() async {
    final preferences = await SharedPreferences.getInstance();
    final today = _today();
    return preferences.getString(_snoozeKey) != today;
  }

  static Future<void> _snoozeToday() async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(_snoozeKey, _today());
  }

  static String _today() {
    final now = DateTime.now();
    final month = now.month.toString().padLeft(2, '0');
    final day = now.day.toString().padLeft(2, '0');
    return '${now.year}-$month-$day';
  }

  /// 仅 Android 支持（Windows 版本次不实现更新）。
  static bool get _supported => Platform.isAndroid;

  /// 应用启动时调用。有新版且今天未提示过时弹出更新对话框。
  static Future<void> checkOnStartup(BuildContext context) async {
    if (!_supported) return;
    if (!await _shouldPromptToday()) return;

    final release = await _fetchLatestRelease();
    if (release == null) return; // 网络失败：静默跳过
    if (!release.isNewerThan(context)) return;
    if (!context.mounted) return;

    await showUpdateDialog(context, release);
  }

  /// 「关于」里手动点「检测新版本」时调用。
  ///
  /// 与启动检查的区别：不读取“今天已提示过”的记录，用户主动触发就必须给结果。
  static Future<void> checkManually(BuildContext context) async {
    if (!_supported) {
      _toast(context, '当前平台暂不支持应用内更新');
      return;
    }
    final progress = ValueNotifier<String>('正在检查更新…');
    if (!context.mounted) return;
    unawaited(
      showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => AlertDialog(
          content: Row(
            children: [
              const SizedBox(
                width: 22,
                height: 22,
                child: CircularProgressIndicator(strokeWidth: 2.5),
              ),
              const SizedBox(width: 16),
              Expanded(
                child: ValueListenableBuilder<String>(
                  valueListenable: progress,
                  builder: (_, value, _) => Text(value),
                ),
              ),
            ],
          ),
        ),
      ),
    );

    final release = await _fetchLatestRelease();
    progress.value = '正在检查更新…';
    if (!context.mounted) return;
    Navigator.of(context, rootNavigator: true).pop(); // 关闭进度框
    if (!context.mounted) return;

    if (release == null) {
      _toast(context, '检查失败，请检查网络后重试');
      return;
    }
    if (!release.isNewerThan(context)) {
      final current = AppLayout.versionOf(context);
      _toast(context, '当前已是最新版本（$current）');
      return;
    }
    await showUpdateDialog(context, release);
  }

  /// 查询最新 Release。失败返回 null（调用方静默处理）。
  ///
  /// 0.2.27 修正（用户真机反馈第 4 条："开了 VPN 后无法检查更新"）：
  /// 原来只直连 `api.github.com`，VPN 下常因分流/速度慢而超时，
  /// 表现为"检查不到新版本"。现在改为**多路依次尝试**：
  /// 1. GitHub API 直连；
  /// 2. `ghfast.top` 镜像代理（该镜像此前已在下载 APK 时验证可用）。
  /// 任意一路成功即返回；全部失败才返回 null。
  static Future<AppRelease?> _fetchLatestRelease() async {
    for (final url in _apiCandidates) {
      final release = await _fetchReleaseFrom(url);
      if (release != null) return release;
    }
    return null;
  }

  /// 更新检查候选地址：直连优先，失败后走镜像。
  static List<String> get _apiCandidates => [
    _apiUrl,
    '${_mirrorPrefix}https://api.github.com/repos/$_owner/$_repo/releases/latest',
  ];

  /// 从单个地址解析 Release；失败（网络/解析）返回 null。
  static Future<AppRelease?> _fetchReleaseFrom(String url) async {
    try {
      final body = await _get(url);
      if (body == null) return null;
      final json = jsonDecode(body);
      if (json is! Map) return null;

      final tag = json['tag_name']?.toString() ?? '';
      final version = _parseVersion(tag);
      if (version == null) return null;

      // 从附件中挑选 arm64-v8a 的 APK（本项目只发这一种架构）。
      String? apkName;
      String? apkUrl;
      final assets = json['assets'];
      if (assets is List) {
        for (final asset in assets) {
          if (asset is! Map) continue;
          final name = asset['name']?.toString() ?? '';
          if (!name.toLowerCase().endsWith('.apk')) continue;
          if (!name.contains('arm64')) continue;
          apkName = name;
          apkUrl = asset['browser_download_url']?.toString();
          break;
        }
      }
      if (apkUrl == null || apkUrl.isEmpty) return null;

      return AppRelease(
        version: version,
        tag: tag,
        apkName: apkName ?? 'update.apk',
        apkUrl: apkUrl,
        notes: json['body']?.toString() ?? '',
      );
    } catch (_) {
      return null;
    }
  }

  /// 简单的 HTTP GET，返回正文；任何失败均返回 null。
  ///
  /// 0.2.27：连接/响应超时放宽（8s/12s → 10s/20s），
  /// 兼顾 VPN/弱网环境；但仍远低于用户的等待上限。
  static Future<String?> _get(String url) async {
    final client = HttpClient();
    client.connectionTimeout = const Duration(seconds: 10);
    try {
      final request = await client.getUrl(Uri.parse(url));
      request.headers.set(HttpHeaders.userAgentHeader, 'zhenguojian-updater');
      request.headers.set(
        HttpHeaders.acceptHeader,
        'application/vnd.github+json',
      );
      final response = await request.close().timeout(
        const Duration(seconds: 20),
      );
      if (response.statusCode != 200) return null;
      return await response.transform(utf8.decoder).join();
    } catch (_) {
      return null;
    } finally {
      client.close(force: true);
    }
  }

  /// 解析 tag，如 "v0.2.17" -> 0.2.17；(0,2,17)；无法解析返回 null。
  static List<int>? _parseVersion(String tag) {
    final match = RegExp(r'(\d+)\.(\d+)\.(\d+)').firstMatch(tag);
    if (match == null) return null;
    return [
      int.parse(match.group(1)!),
      int.parse(match.group(2)!),
      int.parse(match.group(3)!),
    ];
  }

  /// 弹出的更新对话框。
  static Future<void> showUpdateDialog(
    BuildContext context,
    AppRelease release,
  ) async {
    final action = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: const Text('发现新版本'),
        content: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('新版本：${release.version.join('.')}'),
              const SizedBox(height: 6),
              Text('当前版本：${AppLayout.versionOf(context)}'),
              // 0.2.27 新增（用户反馈第 3 条）：展示 GitHub Release 的更新日志。
              // 内容来自 GitHub Release 的 body 字段，就是发布时填写的更新说明。
              if (release.notes.trim().isNotEmpty) ...[
                const SizedBox(height: 12),
                const Text(
                  '更新日志（GitHub）',
                  style: TextStyle(fontWeight: FontWeight.bold),
                ),
                const SizedBox(height: 4),
                Text(release.notes.trim()),
              ],
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop('later'),
            child: const Text('稍后'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop('update'),
            child: const Text('更新'),
          ),
        ],
      ),
    );

    if (action == 'later') {
      await _snoozeToday();
      return;
    }
    if (action != 'update') return;
    if (!context.mounted) return;

    await _downloadAndInstall(context, release);
  }

  /// 下载 APK 并拉起系统安装界面。
  static Future<void> _downloadAndInstall(
    BuildContext context,
    AppRelease release,
  ) async {
    final progress = ValueNotifier<double?>(null);
    if (!context.mounted) return;

    // 下载进度对话框（不可取消，避免半途残留）。
    unawaited(
      showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => AlertDialog(
          title: const Text('正在下载更新'),
          content: ValueListenableBuilder<double?>(
            valueListenable: progress,
            builder: (_, value, _) => Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                LinearProgressIndicator(value: value),
                const SizedBox(height: 12),
                Text(
                  value == null
                      ? '正在连接…'
                      : '${(value * 100).clamp(0, 100).toStringAsFixed(0)}%',
                ),
              ],
            ),
          ),
        ),
      ),
    );

    try {
      final file = await _download(
        release,
        onProgress: (v) => progress.value = v,
      );
      if (!context.mounted) return;
      Navigator.of(context, rootNavigator: true).pop(); // 关闭进度框

      if (file == null) {
        _toast(context, '下载失败，请稍后重试');
        return;
      }
      final status = await _install(file);
      if (!context.mounted) return;
      if (status == 'ok') return;
      if (status == 'no_permission') {
        await _promptInstallPermission(context);
        return;
      }
      if (status == 'missing') {
        _toast(context, '安装包已失效，请重新下载后重试');
        return;
      }
      _toast(context, '安装未完成（$status），可尝试在文件管理器中手动安装');
    } catch (_) {
      if (context.mounted) {
        Navigator.of(context, rootNavigator: true).pop();
        _toast(context, '下载失败，请稍后重试');
      }
    } finally {
      progress.dispose();
    }
  }

  /// 下载到应用缓存目录。失败返回 null。
  ///
  /// 必须放在应用缓存目录内，否则系统安装器拿不到文件（FileProvider 白名单）。
  static Future<File?> _download(
    AppRelease release, {
    void Function(double?)? onProgress,
  }) async {
    Directory directory;
    try {
      directory = await getTemporaryDirectory();
    } catch (_) {
      directory = Directory.systemTemp;
    }
    final target = File(
      '${directory.path}/zhenguojian-update-${release.apkName}',
    );

    for (final url in [release.apkUrl, '$_mirrorPrefix${release.apkUrl}']) {
      final ok = await _downloadFrom(url, target, onProgress: onProgress);
      if (ok) return target;
    }
    return null;
  }

  static Future<bool> _downloadFrom(
    String url,
    File target, {
    void Function(double?)? onProgress,
  }) async {
    final client = HttpClient();
    client.connectionTimeout = const Duration(seconds: 10);
    try {
      var request = await client.getUrl(Uri.parse(url));
      request.headers.set(HttpHeaders.userAgentHeader, 'zhenguojian-updater');
      var response = await request.close().timeout(const Duration(seconds: 15));

      // GitHub Releases 会 302 到对象存储，镜像站点通常直接返回。
      var redirects = 0;
      while (response.isRedirect && redirects < 5) {
        final location = response.headers.value(HttpHeaders.locationHeader);
        if (location == null) break;
        request = await client.getUrl(Uri.parse(location));
        request.headers.set(HttpHeaders.userAgentHeader, 'zhenguojian-updater');
        response = await request.close().timeout(const Duration(seconds: 15));
        redirects++;
      }

      if (response.statusCode != 200) return false;

      final total = response.contentLength;
      var received = 0;
      final sink = target.openWrite();
      await for (final chunk in response) {
        sink.add(chunk);
        received += chunk.length;
        if (onProgress != null && total > 0) {
          onProgress(received / total);
        }
      }
      await sink.close();

      // 基本校验：APK 必须是有效 ZIP（PK 开头），避免把错误页当成安装包。
      if (target.lengthSync() < 1024 * 1024) return false;
      final raf = target.openSync();
      final header = raf.readSync(2);
      raf.closeSync();
      if (header.length < 2 || header[0] != 0x50 || header[1] != 0x4B) {
        return false;
      }
      return true;
    } catch (_) {
      return false;
    } finally {
      client.close(force: true);
    }
  }

  /// 拉起系统安装界面。使用 MethodChannel 调用 MainActivity。
  ///
  /// 返回 'ok' / 'missing' / 'no_permission' / 'failed:原因'，
  /// 便于调用方区分“真没权限”和“其它失败”。
  static Future<String> _install(File file) async {
    try {
      const channel = MethodChannel('duanju/updater');
      final result = await channel.invokeMethod<String>('installApk', {
        'path': file.path,
      });
      return result ?? 'failed:无返回';
    } catch (error) {
      return 'failed:$error';
    }
  }

  /// 系统确实没给「安装未知应用」权限时的引导：一键跳到设置页。
  static Future<void> _promptInstallPermission(BuildContext context) async {
    final go = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('需要安装权限'),
        content: const Text(
          '系统尚未允许本应用安装应用。点「去设置」打开「允许来自此来源的应用」，'
          '然后回到软件重新点一次更新即可。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('稍后'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('去设置'),
          ),
        ],
      ),
    );
    if (go != true) return;
    try {
      const channel = MethodChannel('duanju/updater');
      final opened = await channel.invokeMethod<bool>('openInstallSettings');
      if (opened == true) return;
    } catch (_) {
      // 忽略：下面统一提示。
    }
    if (context.mounted) {
      _toast(context, '请手动在系统设置中允许本应用安装应用');
    }
  }

  static void _toast(BuildContext context, String message) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }
}

/// 一次可用的更新信息。
class AppRelease {
  const AppRelease({
    required this.version,
    required this.tag,
    required this.apkName,
    required this.apkUrl,
    required this.notes,
  });

  final List<int> version;
  final String tag;
  final String apkName;
  final String apkUrl;
  final String notes;

  /// 与当前版本比较，是否更新。
  bool isNewerThan(BuildContext context) {
    return _isNewer(version, _currentVersion(context));
  }

  static List<int> _currentVersion(BuildContext context) {
    final text = AppLayout.versionOf(context);
    final match = RegExp(r'(\d+)\.(\d+)\.(\d+)').firstMatch(text);
    if (match == null) return const [0, 0, 0];
    return [
      int.parse(match.group(1)!),
      int.parse(match.group(2)!),
      int.parse(match.group(3)!),
    ];
  }

  /// 字典序比较：remote > local 返回 true。
  static bool _isNewer(List<int> remote, List<int> local) {
    for (var i = 0; i < 3; i++) {
      final r = i < remote.length ? remote[i] : 0;
      final l = i < local.length ? local[i] : 0;
      if (r != l) return r > l;
    }
    return false;
  }
}
