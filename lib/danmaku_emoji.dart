import 'package:flutter/painting.dart';

const Map<String, String> danmakuEmojis = {
  '[微笑]': '🙂',
  '[大笑]': '😄',
  '[呲牙]': '😁',
  '[憨笑]': '😆',
  '[嘿哈]': '😄',
  '[偷笑]': '🤭',
  '[笑哭]': '😂',
  '[尬笑]': '😅',
  '[奸笑]': '😏',
  '[坏笑]': '😈',
  '[得意]': '😎',
  '[愉快]': '😊',
  '[可爱]': '🥰',
  '[爱慕]': '😍',
  '[色]': '😍',
  '[星星眼]': '🤩',
  '[亲亲]': '😘',
  '[吻]': '😘',
  '[飞吻]': '😘',
  '[调皮]': '😜',
  '[吐舌]': '😛',
  '[做鬼脸]': '😝',
  '[舔屏]': '🤤',
  '[思考]': '🤔',
  '[机智]': '🧐',
  '[懵]': '😳',
  '[呆无辜]': '😳',
  '[发呆]': '😐',
  '[惊呆]': '😲',
  '[惊讶]': '😮',
  '[震惊]': '😱',
  '[惊恐]': '😱',
  '[恐惧]': '😨',
  '[石化]': '😵',
  '[晕]': '😵',
  '[宕机]': '😵',
  '[翻白眼]': '🙄',
  '[白眼]': '🙄',
  '[傲慢]': '😤',
  '[酷拽]': '😏',
  '[冷漠]': '😐',
  '[无语流汗]': '😓',
  '[尴尬流汗]': '😅',
  '[流汗]': '😅',
  '[汗]': '😅',
  '[擦汗]': '😅',
  '[尴尬]': '😬',
  '[黑线]': '😑',
  '[撇嘴]': '😒',
  '[鄙视]': '😒',
  '[斜眼]': '😒',
  '[嘘]': '🤫',
  '[闭嘴]': '🤐',
  '[哈欠]': '🥱',
  '[困]': '😴',
  '[睡]': '😴',
  '[鼾睡]': '😴',
  '[捂脸]': '🤦',
  '[打脸]': '🤦',
  '[脸疼]': '🤦',
  '[我想静静]': '😔',
  '[委屈]': '🥺',
  '[可怜]': '🥺',
  '[求求了]': '🥺',
  '[快哭了]': '🥹',
  '[眼含热泪]': '🥹',
  '[泣不成声]': '😭',
  '[大哭]': '😭',
  '[流泪]': '😢',
  '[哭]': '😢',
  '[没看够]': '🥺',
  '[哭哭]': '😭',
  '[泪奔]': '😭',
  '[苦涩]': '😖',
  '[难过]': '🙁',
  '[叹气]': '😮‍💨',
  '[裂开]': '🤯',
  '[抓狂]': '😫',
  '[发怒]': '😡',
  '[愤怒]': '😡',
  '[咒骂]': '🤬',
  '[骂]': '🤬',
  '[左哼哼]': '😤',
  '[右哼哼]': '😤',
  '[吐]': '🤮',
  '[吐血]': '🤮',
  '[生病]': '🤒',
  '[赞]': '👍',
  '[点赞]': '👍',
  '[强]': '👍',
  '[给力]': '👍',
  '[666]': '👍',
  '[6]': '👍',
  '[V5]': '👍',
  '[绝]': '👍',
  '[弱]': '👎',
  '[你不大行]': '👎',
  '[小鼓掌]': '👏',
  '[鼓掌]': '👏',
  '[击掌]': '🙌',
  '[举手]': '🙋',
  '[投降]': '🙌',
  '[碰拳]': '👊',
  '[拳头]': '✊',
  '[握拳]': '✊',
  '[抱拳]': '🙏',
  '[握手]': '🤝',
  '[感谢]': '🙏',
  '[祈祷]': '🙏',
  '[黄脸祈祷]': '🙏',
  '[敲木鱼]': '🙏',
  '[鞠躬]': '🙇',
  '[点头哈腰]': '🙇',
  '[比心]': '🫰',
  '[爱心手]': '🫶',
  '[抱抱你]': '🤗',
  '[求抱抱]': '🤗',
  '[拥抱]': '🤗',
  '[摸头]': '🫳',
  '[戳手手]': '👉',
  '[耶]': '✌️',
  '[胜利]': '✌️',
  '[OK]': '👌',
  '[okk]': '👌',
  '[yes]': '👌',
  '[no]': '🙅',
  '[达咩]': '🙅',
  '[一起加油]': '💪',
  '[加油]': '💪',
  '[强壮]': '💪',
  '[奋斗]': '💪',
  '[爱心]': '❤️',
  '[送心]': '❤️',
  '[心碎]': '💔',
  '[玫瑰]': '🌹',
  '[送花]': '🌹',
  '[递花]': '💐',
  '[手花]': '💐',
  '[撒花]': '🎉',
  '[庆祝]': '🎉',
  '[礼物]': '🎁',
  '[红包]': '🧧',
  '[钱]': '💰',
  '[金牌]': '🥇',
  '[奖牌]': '🏅',
  '[夺金]': '🥇',
  '[足球]': '⚽',
  '[啤酒]': '🍺',
  '[咖啡]': '☕',
  '[蛋糕]': '🎂',
  '[吃瓜群众]': '🍉',
  '[狗头]': '🐶',
  '[看]': '👀',
  '[暗中观察]': '👀',
  '[凝视]': '👀',
  '[不看]': '🙈',
  '[听歌]': '🎧',
  '[拜拜]': '👋',
  '[再见]': '👋',
  '[what]': '❓',
  '[疑问]': '❓',
};

String? danmakuEmojiOf(String tag) => danmakuEmojis[tag];

/// 把弹幕文字按表情标签切成「文字 / Emoji」交替的片段。
/// 未收录的方括号标签保持原样，避免误伤普通文字。
List<DanmakuTextPart> splitDanmakuEmojis(String text) {
  if (text.isEmpty) return const [];
  final parts = <DanmakuTextPart>[];
  final buffer = StringBuffer();
  var index = 0;
  while (index < text.length) {
    final open = text.indexOf('[', index);
    if (open < 0) {
      buffer.write(text.substring(index));
      break;
    }
    if (open > index) buffer.write(text.substring(index, open));
    final close = text.indexOf(']', open + 1);
    if (close < 0) {
      buffer.write(text.substring(open));
      break;
    }
    final emoji = danmakuEmojis[text.substring(open, close + 1)];
    if (emoji == null) {
      buffer.write(text.substring(open, close + 1));
      index = close + 1;
      continue;
    }
    if (buffer.isNotEmpty) {
      parts.add(DanmakuTextPart(buffer.toString(), false));
      buffer.clear();
    }
    parts.add(DanmakuTextPart(emoji, true));
    index = close + 1;
  }
  if (buffer.isNotEmpty) parts.add(DanmakuTextPart(buffer.toString(), false));
  return parts;
}

class DanmakuTextPart {
  const DanmakuTextPart(this.text, this.emoji);
  final String text;
  final bool emoji;
}

int danmakuEmojiCount(String text) =>
    splitDanmakuEmojis(text).where((part) => part.emoji).length;

/// Emoji 相对弹幕字号的放大比例，让表情更接近官方观感。
const double danmakuEmojiScale = 1.35;

TextSpan danmakuTextSpan(
  String text,
  TextStyle style, {
  TextScaler? textScaler,
}) {
  final parts = splitDanmakuEmojis(text);
  if (parts.isEmpty) return TextSpan(text: '', style: style);
  if (parts.length == 1 && !parts.first.emoji) {
    return TextSpan(text: parts.first.text, style: style);
  }
  return TextSpan(
    style: style,
    children: [
      for (final part in parts)
        TextSpan(
          text: part.text,
          style: part.emoji
              ? style.copyWith(
                  fontSize: (style.fontSize ?? 16) * danmakuEmojiScale,
                  fontWeight: FontWeight.w400,
                )
              : null,
        ),
    ],
  );
}
