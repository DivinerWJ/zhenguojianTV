import 'dart:async';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'app_layout.dart';
import 'core_bridge.dart';
import 'local_store.dart';
import 'widgets.dart';

/// 模式 → 展示文案（设置页入口的 subtitle 用）。
String MirrorConfigScreenModeLabel(String mode) => switch (mode) {
  'builtin' => '当前：使用内置配置',
  'custom' => '当前：自定义配置',
  _ => '当前：远程热更新',
};

/// 站源镜像配置页（0.8.11）。
///
/// 三种模式：
/// - **远程热更新**（默认）：核心从 jsDelivr 拉取 `config/source_mirrors.json`，
///   站点换域名时无需重新出包；
/// - **使用内置配置**：随 APK 打包的同一份文件，离线/远程不可达时的兜底；
/// - **自定义**：用户指定的本地 JSON 文件（可用下方下载按钮先拉一份基础
///   文件再修改，也可用文件选择器挑选）或远程 JSON 链接。
class MirrorConfigScreen extends StatefulWidget {
  const MirrorConfigScreen({
    super.key,
    required this.repository,
    required this.store,
  });
  final AppRepository repository;
  final LocalStore store;

  @override
  State<MirrorConfigScreen> createState() => _MirrorConfigScreenState();
}

class _MirrorConfigScreenState extends State<MirrorConfigScreen> {
  late final TextEditingController _source;
  late String _mode;
  bool _busy = false;
  String? _message;
  String? _error;
  String? _downloadedPath;

  @override
  void initState() {
    super.initState();
    _mode = widget.store.mirrorConfigMode;
    _source = TextEditingController(text: widget.store.mirrorConfigSource);
  }

  @override
  void dispose() {
    _source.dispose();
    super.dispose();
  }

  Future<void> _apply(String mode, {String? source}) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _message = null;
      _error = null;
    });
    try {
      await (source == null
          ? widget.repository.applyMirrorConfig(mode, _source.text)
          : widget.repository.applyMirrorConfig(mode, source));
      if (mounted) {
        setState(() {
          _message = switch (mode) {
            'builtin' => '已应用内置配置',
            'custom' => '已应用自定义配置',
            _ => '已切换到远程热更新',
          };
        });
      }
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _selectMode(String mode) async {
    await saveUserChange(context, () => widget.store.setMirrorConfigMode(mode));
    if (!mounted) return;
    setState(() => _mode = mode);
    await _apply(mode);
  }

  Future<void> _saveAndApplyCustom() async {
    await saveUserChange(
      context,
      () => widget.store.setMirrorConfigSource(_source.text),
    );
    await _apply('custom');
  }

  Future<void> _pickLocalFile() async {
    try {
      final file = await FilePicker.pickFile(
        type: FileType.custom,
        allowedExtensions: ['json'],
        dialogTitle: '选择镜像配置 JSON',
      );
      if (file == null || !mounted) return;
      final filePath = file.path;
      if (filePath == null) return;
      setState(() => _source.text = filePath);
      await saveUserChange(
        context,
        () => widget.store.setMirrorConfigSource(filePath),
      );
    } on PlatformException catch (error) {
      if (mounted) setState(() => _error = '选择文件失败：${error.message ?? ''}');
    }
  }

  /// 下载远程配置到设备本地，作为自定义模式的基础文件。
  Future<void> _downloadRemote() async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _message = null;
      _error = null;
    });
    try {
      final repository = widget.repository;
      final Future<String> future = repository is NativeRepository
          ? repository.downloadRemoteMirrorConfig()
          : Future.error(AppFailure('当前环境不支持下载远程配置'));
      final savedPath = await future;
      if (!mounted) return;
      setState(() {
        _downloadedPath = savedPath;
        _message = '已下载到：$savedPath\n用文件管理器修改后，'
            '选「自定义」并填入该路径即可生效';
      });
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// 把刚下载的基础文件路径填入自定义源并应用（可选的快捷步骤）。
  Future<void> _useDownloadedPath() async {
    final savedPath = _downloadedPath;
    if (savedPath == null || _busy) return;
    setState(() => _source.text = savedPath);
    await saveUserChange(
      context,
      () => widget.store.setMirrorConfigSource(savedPath),
    );
    await _apply('custom');
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final tv = AppLayout.isTelevision(context);
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: !tv,
        title: const Text('站源镜像配置'),
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 680),
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                '站源镜像池决定各站源使用哪些备用域名。三种模式按需切换，'
                '切换后立即生效，无需重启应用。',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 8),
              RadioListTile<String>(
                value: 'remote',
                groupValue: _mode,
                title: const Text('远程热更新（推荐）'),
                subtitle: const Text('跟随项目仓库在线更新，站点换域名无需出包'),
                onChanged: (value) {
                  if (value != null) unawaited(_selectMode(value));
                },
              ),
              RadioListTile<String>(
                value: 'builtin',
                groupValue: _mode,
                title: const Text('使用内置配置'),
                subtitle: const Text('随安装包固化的镜像列表，离线可用'),
                onChanged: (value) {
                  if (value != null) unawaited(_selectMode(value));
                },
              ),
              RadioListTile<String>(
                value: 'custom',
                groupValue: _mode,
                title: const Text('自定义配置'),
                subtitle: const Text('本地 JSON 文件或远程 JSON 链接'),
                onChanged: (value) {
                  if (value != null) unawaited(_selectMode(value));
                },
              ),
              if (_mode == 'custom') ...[
                const Divider(),
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
                  child: TextField(
                    controller: _source,
                    decoration: const InputDecoration(
                      labelText: '本地文件路径或 JSON 链接',
                      hintText: '/sdcard/.../source_mirrors.json 或 https://…',
                      border: OutlineInputBorder(),
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Wrap(
                    spacing: 12,
                    runSpacing: 8,
                    children: [
                      FilledButton(
                        onPressed: _busy ? null : _saveAndApplyCustom,
                        child: const Text('保存并应用'),
                      ),
                      OutlinedButton(
                        onPressed: _busy ? null : _pickLocalFile,
                        child: const Text('选择文件…'),
                      ),
                    ],
                  ),
                ),
              ],
              const Divider(),
              ListTile(
                title: const Text('下载远程配置到本地'),
                subtitle: const Text(
                  '保存到系统「下载」目录（Download/source_mirrors.json），'
                  '作为自定义模式的基础文件，修改后即可生效',
                ),
                trailing: _busy
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : FilledButton.tonal(
                        onPressed: _downloadRemote,
                        child: const Text('下载'),
                      ),
              ),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    _error!,
                    style: TextStyle(color: theme.colorScheme.error),
                  ),
                ),
              if (_message != null) ...[
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(_message!),
                ),
                if (_downloadedPath != null)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                    child: OutlinedButton(
                      onPressed: _busy ? null : _useDownloadedPath,
                      child: const Text('已改好，直接使用该文件'),
                    ),
                  ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
