import 'package:pinyin/pinyin.dart';

/// TV 端拼音搜索工具。
///
/// 使用纯 Dart 的 `pinyin` 词库，不依赖 Python、系统输入法或 Android 原生 API。
/// 词组优先转换，因此「重生」「重庆」「长安」这类多音字场景比逐字码表准确。
///
/// 搜索同时支持：
/// - 首字母：`hjdnz` -> 「黄金大逆转」；
/// - 首字母模糊缩写：`hdz` -> 「黄金大逆转」；
/// - 完整/部分全拼：`huangjin` / `huangjindanizhuan`；
/// - 大小写混输。
const int _maxCacheEntries = 4000;
final Map<String, _PinyinKey> _cache = <String, _PinyinKey>{};

class _PinyinKey {
  const _PinyinKey({
    required this.initials,
    required this.full,
    required this.singleCharInitials,
  });

  /// 词组感知的首字母，例如「重生」-> `cs`。
  final String initials;

  /// 词组感知的完整拼音，例如「黄金大逆转」-> `huangjindanizhuan`。
  final String full;

  /// 逐字首选读音的首字母，仅作为旧行为兼容兜底。
  ///
  /// 旧版使用逐字码表，多音字可能得到另一种读音，例如「重生」可能是 `zs`。
  /// 保留这个键可以避免升级后用户原来的遥控器输入习惯突然失效。
  final String singleCharInitials;
}

bool _isAsciiLetter(int unit) =>
    (unit >= 0x41 && unit <= 0x5a) || (unit >= 0x61 && unit <= 0x7a);

bool _isAsciiDigit(int unit) => unit >= 0x30 && unit <= 0x39;

/// 只保留 ASCII 字母和数字并转成小写。
///
/// pinyin 包会原样保留标点和非汉字内容；搜索键不需要这些字符。
String _compactAscii(String value) {
  final buffer = StringBuffer();
  for (final unit in value.codeUnits) {
    if (_isAsciiLetter(unit)) {
      buffer.writeCharCode(unit >= 0x41 && unit <= 0x5a ? unit + 0x20 : unit);
    } else if (_isAsciiDigit(unit)) {
      buffer.writeCharCode(unit);
    }
  }
  return buffer.toString();
}

String _singleCharInitials(String text) {
  final buffer = StringBuffer();
  for (final rune in text.runes) {
    if (rune < 0x80) {
      if (_isAsciiLetter(rune) || _isAsciiDigit(rune)) {
        buffer.writeCharCode(
          rune >= 0x41 && rune <= 0x5a ? rune + 0x20 : rune,
        );
      }
      continue;
    }

    final char = String.fromCharCode(rune);
    if (!ChineseHelper.isChinese(char)) continue;
    final values = PinyinHelper.convertToPinyinArray(
      char,
      PinyinFormat.WITHOUT_TONE,
    );
    if (values.isNotEmpty && values.first.isNotEmpty) {
      buffer.write(values.first[0].toLowerCase());
    }
  }
  return buffer.toString();
}

_PinyinKey _keyOf(String text) {
  final cached = _cache[text];
  if (cached != null) return cached;

  // getShortPinyin / getPinyin 都会优先使用词组字典处理多音字。
  final initials = _compactAscii(PinyinHelper.getShortPinyin(text));
  String full;
  try {
    full = _compactAscii(
      PinyinHelper.getPinyin(
        text,
        separator: '',
        format: PinyinFormat.WITHOUT_TONE,
      ),
    );
  } on PinyinException {
    // 极少数扩展区汉字不在词库中时，不让搜索因为一个标题崩掉。
    // 首字母仍可用；全拼退化为空串。
    full = '';
  }

  final key = _PinyinKey(
    initials: initials,
    full: full,
    singleCharInitials: _singleCharInitials(text),
  );
  if (_cache.length >= _maxCacheEntries) _cache.clear();
  _cache[text] = key;
  return key;
}

/// 把文本转换成词组感知的拼音首字母串。
///
/// 汉字按实际词组读音转换；ASCII 字母/数字保留并转小写；标点和 emoji 丢弃。
String pinyinInitials(String text) => _keyOf(text).initials;

/// 把文本转换成不带声调、无分隔符的完整拼音。
String pinyinFull(String text) => _keyOf(text).full;

/// 关键词是否适合作为拼音搜索词。
///
/// 只认 2–48 个 ASCII 字母。上限放宽到 48 是为了支持完整拼音，旧版 12 个字符
/// 会把稍长的全拼误判成普通英文搜索。
bool isPinyinQuery(String query) {
  final value = query.trim();
  if (value.length < 2 || value.length > 48) return false;
  for (final unit in value.codeUnits) {
    if (!_isAsciiLetter(unit)) return false;
  }
  return true;
}

bool _isSubsequence(String haystack, String needle) {
  var from = 0;
  for (var i = 0; i < needle.length; i++) {
    final found = haystack.indexOf(needle[i], from);
    if (found < 0) return false;
    from = found + 1;
  }
  return true;
}

/// [query] 是否按拼音命中 [text]。
///
/// 优先做精确度更高的全拼和首字母包含匹配，最后才使用首字母子序列模糊匹配。
/// 例如 `hjdnz`、`hdz`、`huangjin` 都能命中「黄金大逆转」。
bool matchesPinyin(String text, String query) {
  if (!isPinyinQuery(query)) return false;
  final needle = query.trim().toLowerCase();
  final key = _keyOf(text);
  if (key.initials.isEmpty && key.full.isEmpty) return false;

  if (key.initials.contains(needle) || key.full.contains(needle)) return true;
  if (_isSubsequence(key.initials, needle)) return true;

  // 兼容旧版逐字码表对多音字形成的首字母，避免升级后历史输入习惯失效。
  final legacy = key.singleCharInitials;
  return legacy.isNotEmpty &&
      (legacy.contains(needle) || _isSubsequence(legacy, needle));
}

/// 兼容 0.8.4 已有调用名。
@Deprecated('Use matchesPinyin instead.')
bool matchesPinyinInitials(String text, String query) =>
    matchesPinyin(text, query);
