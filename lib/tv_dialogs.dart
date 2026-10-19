import 'package:flutter/material.dart';

import 'app_layout.dart';
import 'remote_widgets.dart';

/// 电视友好的确认框（0.8.1）。
///
/// 为什么不直接用 `AlertDialog` + `TextButton`：
/// - 电视上没有触摸，`TextButton` 虽然能获得焦点，但焦点环在深色对话框上
///   不够醒目，10 英尺外看不清"当前选的是哪个"；
/// - 这里两个按钮都用 [RemoteButton]（带缩放 + 描边 + 光晕的焦点样式），
///   方向键左右移动、确定键触发，与全局遥控导航一致。
///
/// 默认焦点落在**取消**上：电视遥控容易连按，误按确定不该直接退出播放/退出应用。
/// 返回键等价于取消（`showDialog` 的 barrier 被 DismissIntent 关闭，返回 null）。
///
/// 非电视模式同样可用，只是按钮变成了可点击的遥控按钮，视觉与全局一致。
Future<bool> showTvConfirmDialog(
  BuildContext context, {
  required String title,
  String? message,
  String confirmLabel = '确定',
  String cancelLabel = '取消',
}) async {
  final television = AppLayout.isTelevision(context);
  final result = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      insetPadding: EdgeInsets.symmetric(
        horizontal: television ? 72 : 40,
        vertical: 24,
      ),
      title: Text(
        title,
        style: TextStyle(
          fontSize: television ? 22 : 18,
          fontWeight: FontWeight.w600,
        ),
      ),
      content: message == null
          ? null
          : ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 460),
              child: Text(
                message,
                style: TextStyle(
                  fontSize: television ? 16 : 14,
                  height: 1.6,
                  color: Theme.of(dialogContext).colorScheme.onSurfaceVariant,
                ),
              ),
            ),
      actionsPadding: EdgeInsets.fromLTRB(16, 4, 16, television ? 16 : 8),
      actions: [
        RemoteButton(
          label: cancelLabel,
          autofocus: true,
          onPressed: () => Navigator.pop(dialogContext, false),
        ),
        RemoteButton(
          label: confirmLabel,
          onPressed: () => Navigator.pop(dialogContext, true),
        ),
      ],
    ),
  );
  return result ?? false;
}
