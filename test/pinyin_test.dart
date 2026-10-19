import 'package:duanju_app/catalog_sort.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/pinyin.dart';
import 'package:flutter_test/flutter_test.dart';

Drama drama(
  String title, {
  String description = '',
  List<String> tags = const [],
}) => Drama(
  id: title,
  source: 'hongguo',
  title: title,
  description: description,
  tags: tags,
);

void main() {
  group('pinyin keys', () {
    test('词组感知首字母与全拼', () {
      expect(pinyinInitials('黄金大逆转'), 'hjdnz');
      expect(pinyinFull('黄金大逆转'), 'huangjindanizhuan');
      // 多音字应按词组实际读音处理：重生 = chong sheng。
      expect(pinyinInitials('重生'), 'cs');
      expect(pinyinFull('重生'), 'chongsheng');
    });

    test('ASCII 保留，标点空格 emoji 被清理', () {
      expect(pinyinInitials('TV版'), 'tvb');
      expect(pinyinInitials('第2季'), 'd2j');
      expect(pinyinInitials('黄金·大逆转'), 'hjdnz');
      expect(pinyinInitials('🔥重生 2024！'), 'cs2024');
    });

    test('空串与纯标点返回空串', () {
      expect(pinyinInitials(''), '');
      expect(pinyinInitials('《》【】'), '');
    });
  });

  group('isPinyinQuery', () {
    test('2–48 个纯字母可作为拼音', () {
      expect(isPinyinQuery('hjdnz'), isTrue);
      expect(isPinyinQuery('HUANGJIN'), isTrue);
      expect(isPinyinQuery('huangjindanizhuan'), isTrue);
      expect(isPinyinQuery('a'), isFalse);
      expect(isPinyinQuery('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'), isFalse);
    });

    test('中文、数字、混合不算纯拼音', () {
      expect(isPinyinQuery('黄金'), isFalse);
      expect(isPinyinQuery('hj2'), isFalse);
      expect(isPinyinQuery('hj dnz'), isFalse);
      expect(isPinyinQuery(''), isFalse);
    });
  });

  group('matchesPinyin', () {
    test('完整首字母与缩写命中', () {
      expect(matchesPinyin('黄金大逆转', 'hjdnz'), isTrue);
      expect(matchesPinyin('黄金大逆转', 'hdz'), isTrue);
    });

    test('完整拼音和部分全拼命中', () {
      expect(matchesPinyin('黄金大逆转', 'huangjin'), isTrue);
      expect(matchesPinyin('黄金大逆转', 'huangjindanizhuan'), isTrue);
      expect(matchesPinyin('黄金大逆转', 'HUANGJIN'), isTrue);
    });

    test('多音字使用词组读音，同时兼容旧逐字首字母', () {
      expect(matchesPinyin('重生', 'cs'), isTrue);
      expect(matchesPinyin('重生', 'chongsheng'), isTrue);
      // 旧版逐字码表可能把「重」按 zhong 处理，升级后仍允许 zs。
      expect(matchesPinyin('重生', 'zs'), isTrue);
    });

    test('顺序错误或不存在字母不命中', () {
      expect(matchesPinyin('黄金大逆转', 'znh'), isFalse);
      expect(matchesPinyin('黄金大逆转', 'hjdq'), isFalse);
    });

    test('非拼音关键词不进入拼音匹配', () {
      expect(matchesPinyin('黄金大逆转', '黄金'), isFalse);
      expect(matchesPinyin('黄金大逆转', 'a'), isFalse);
    });
  });

  group('matchesDramaQuery 拼音兜底', () {
    test('首字母和全拼都能搜到中文标题', () {
      expect(matchesDramaQuery(drama('黄金大逆转'), 'hjdnz'), isTrue);
      expect(matchesDramaQuery(drama('黄金大逆转'), 'huangjin'), isTrue);
      expect(matchesDramaQuery(drama('总裁的替身新娘'), 'zcdtsxn'), isTrue);
    });

    test('原有中文、描述和标签搜索行为不变', () {
      final item = drama('黄金大逆转', description: '都市逆袭', tags: ['爽剧']);
      expect(matchesDramaQuery(item, '黄金'), isTrue);
      expect(matchesDramaQuery(item, '都市'), isTrue);
      expect(matchesDramaQuery(item, '爽剧'), isTrue);
      expect(matchesDramaQuery(item, '白银'), isFalse);
    });

    test('拼音只对标题生效', () {
      final item = drama('黄金大逆转', description: '都市逆袭');
      expect(matchesDramaQuery(item, 'ds'), isFalse);
    });

    test('单字母仍按普通子串搜索', () {
      expect(matchesDramaQuery(drama('黄金大逆转'), 'a'), isFalse);
      expect(matchesDramaQuery(drama('Abc剧'), 'a'), isTrue);
    });
  });
}
