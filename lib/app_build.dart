const allSourcesEnabled = bool.fromEnvironment('ALL_SOURCES');

/// 0.8.10：全源版本的应用名从「真果鉴」改为「真果鉴TV」。
/// 红果鉴是另一条只含红果站源的分发线，品牌名不动。
const appName = allSourcesEnabled ? '真果鉴TV' : '红果鉴';
const appSlug = allSourcesEnabled ? 'zhenguojian' : 'hongguojian';
