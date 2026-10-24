# 真果鉴开发约定

## 项目目标

构建 Flutter 多端独立应用。站源请求、解析、在线播放及后续下载、合并均在设备端完成；成品不依赖旧短剧库程序、旧项目路径或远程自建服务。可以迁移已经验证的业务逻辑，并将依赖一并纳入本项目。

## 优先级与交付

1. Android 手机和 Windows 电脑同为第一优先级。
2. Android TV 第二优先级，适配遥控焦点与电视布局。
3. iOS 最后实施，单独处理原生媒体库和后台任务限制。
4. 首版优先完成可用的在线播放闭环：浏览、搜索、剧集详情、选集、播放、错误重试。
5. 其他功能按需求方给出的反馈顺序逐项实现。不要为了未到优先级的功能推迟首版。

## 实施与验证

按用户 2026-09-21 的要求，目前完成功能优先，暂停测试与回归，待用户安排后集中执行。源码格式整理及任务收尾同步仍执行；未验证的实现使用带 `unverified` 的开发快照 tag，不作为通过验收的版本。

恢复集中验证后，先完整实现当前交付范围，再集中执行静态检查、构建、自动化和设备验证；不要每改一个小功能就测试。集中验证发现问题后，修复并执行相关复验。未验证的平台和行为必须明确标注，不得将生成源码等同于可用成品。

直接在本项目中维护源码。临时目录仅放工具、构建缓存和验证产物，不在临时目录维护另一份正式实现。不要修改旧短剧库和果果剧库。

## 任务收尾同步

每次完成开发、修复或文档修改后，在必要检查通过、最终回复之前，必须执行 `python3 scripts/finish_task.py --message "本次实际完成的变更"`。脚本将干净源码同步到本项目同级的 `../guoapp`，创建本地 Git 提交和带说明的 tag，作为可恢复的版本。有后续修改时重新执行收尾，并用 `python3 scripts/sync_source.py --check` 确认一致；失败先处理原因，不要声称已完成。

`guoapp` 是面向 GitHub 的源码镜像。保留目标已有的 `.git` 和历史；其余文件与导出源码保持一致，包含源码、必要资源、构建配置、锁文件、测试和 Actions，排除 SDK、第三方依赖目录、缓存、编译产物、签名文件和个人配置。新增源码根目录或构建输入时同步维护脚本的收录规则。用户已授权每次实施结束后本地提交并打 tag，不再重复询问；不要自动推送 Git。

功能版本递增 `pubspec.yaml` 的版本号和构建号。tag 默认使用 `v<版本号>`；同一版本的多批改动共用同一个 tag，发布前把该 tag 移到最新提交，对外发布之后不再改动。最终回复说明本轮 tag；未通过检查的代码不得标为已完成版本。

如果正在 `guoapp` 本身工作，不向自身同步；脚本会跳过此情况。不要为了同步另建临时源码副本。

## Android 构建约定（Windows 本机）

1. 正式出包一律走脚本完整流水线：`python scripts/build_android.py --abi arm64-v8a --all-sources [--application-id-suffix .dev]`（内部依次完成 Go so 编译、pub get、split-per-abi 构建、收包到 `dist/android/` 并生成 SHA256SUMS）。临时验证可直跑 `flutter build apk`，产物留在 `build/app/outputs/flutter-apk/`，不进 dist。
2. 带 `--application-id-suffix` 的共存构建 versionCode 自动 +2000（如 0.8.11+86 → 2086），保证永远高于正式版；与直跑 `flutter build apk`（无偏移）的产物混装会出现 `INSTALL_FAILED_VERSION_DOWNGRADE`，此时用 `adb install -r -d` 降级安装。同一项目禁止并发跑两个构建。
3. 构建环境必须设置 `FLUTTER_STORAGE_BASE_URL=https://storage.flutter-io.cn`；置空会导致 Gradle 连 Google 拉引擎产物并永久挂起（本机不通 Google）。后缀注入经 `ORG_GRADLE_PROJECT_applicationIdSuffix` / `ORG_GRADLE_PROJECT_appLabelSuffix` 传入，`build.gradle.kts` 已做属性/环境变量三重兜底。
4. 构建脚本会临时改写 `pubspec.lock` 切镜像、正常退出时还原；后台构建被强杀会残留镜像地址导致 `--enforce-lockfile` 报错，先 `git checkout -- pubspec.lock` 再重试。
5. `flutter clean` 会连带删除 `build/media_kit_libs_android_video/` 里的预下载 jar，导致 Gradle 卡 GitHub 下载；clean 前先备份该目录，误删后用 gh-proxy.com 按插件 `build.gradle` 内置 MD5 重下 4 个 jar 恢复。
6. 改过依赖版本（pubspec.yaml / pubspec.lock）后必须清 Dart 编译缓存再构建，否则 AOT 产物与原生插件 AAR 版本错配，启动即抛 `ClassNotFoundException`（实际案例：shared_preferences_android 2.4.28 与 2.5.3 混用）。仅改 Dart 源码无需清缓存。
7. 新增本地偏好键必须同步登记 `local_snapshot.dart` 的 `globalKeys` 白名单，否则保存时抛「无效的本地配置字段」并被通用错误提示掩盖。
8. 站源镜像配置支持三模式：远程 jsDelivr 热更新（默认）/ APK 内置资产 / 自定义（本地文件路径或 JSON 链接），入口在设置页「站源镜像配置」；「下载」按钮经 MainActivity `saveToDownloads`（MediaStore）存到公共 Download 目录。镜像列表是故障转移候选池，仅首位（或上次成功主机）被主动使用，新增域名放首位才能验证。

## 代码与资料

README.md 只做**产品介绍**（品牌、特性、站源、安装、平台与声明），不写构建细节与版本流水；版本更新说明放 `CHANGELOG.md`（`Build APK` 的 Release 说明从那里抓 `### <版本> · <主题>` 小节），使用说明与构建说明放 `docs/USAGE.md`、`docs/BUILD.md`。AGENTS.md 仅记录开发约定。

不新增解释性代码注释，必要的编译或工具指令除外。采用清晰命名和模块边界。不要提交密钥、个人配置、下载的视频、SDK 或构建缓存。

不要查看、下载、分析或处理站源图片。页面可正常显示海报地址；开发验证使用合成数据或拦截图片请求。用户直接提供的界面截图可以查看。

遵守先实现后集中验证的节奏；未经用户明确要求，不启动子 AGENT。
