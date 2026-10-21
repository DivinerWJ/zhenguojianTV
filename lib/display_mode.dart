import 'package:flutter/material.dart';

import 'local_store.dart';
import 'widgets.dart';

const displayModeLabels = {
  'auto': '自动识别设备',
  'television': '电视 / 遥控器',
  'standard': '手机 / 电脑',
};

String displayModeLabel(String mode) => displayModeLabels[mode] ?? mode;

/// 界面模式选择。
///
/// 0.8.7 起电视版没有顶部栏，这个入口从「更多」菜单挪到设置页，
/// 手机、电脑、电视共用同一份实现。
Future<void> chooseDisplayMode(BuildContext context, LocalStore store) async {
  final selection = await showDialog<String>(
    context: context,
    builder: (dialogContext) => SimpleDialog(
      title: const Text('界面模式'),
      children: [
        RadioGroup<String>(
          groupValue: store.displayMode,
          onChanged: (value) => Navigator.pop(dialogContext, value),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final mode in displayModeLabels.entries)
                RadioListTile<String>(
                  value: mode.key,
                  autofocus: mode.key == store.displayMode,
                  title: Text(mode.value),
                ),
            ],
          ),
        ),
      ],
    ),
  );
  if (selection == null || !context.mounted) return;
  await saveUserChange(context, () => store.setDisplayMode(selection));
}
