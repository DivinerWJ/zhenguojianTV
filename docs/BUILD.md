# 开发与构建

产品介绍见 [README](../README.md)，版本变化见 [更新日志](../CHANGELOG.md)。

## 环境

Flutter `3.47.4`、Dart `3.12+`、Go `1.24.1+`、Python `3.10+`；Android 需 JDK 17、SDK 36、NDK `28.2.13676358`。

## 本地构建

```sh
# 全站源版（真果鉴TV）
python3 scripts/build_android.py --abi arm64-v8a --all-sources
# 仅红果版（红果鉴）
python3 scripts/build_android.py --abi arm64-v8a
# 电视版（arm64-v8a + armeabi-v7a 分包，并额外产出 universal/all APK）
python3 scripts/build_android.py --tv --universal --all-sources
```

- **必须在纯英文路径下构建**（中文路径会导致工具链失败）。
- 构建时不要设 HTTP 代理；访问 GitHub 时才需要代理。
- 产物在 `dist/android/`，`SHA256SUMS.txt` 为校验清单。

## 走 Actions 构建

**Actions → Build APK → Run workflow**：

- `all_sources` 决定真果鉴TV / 红果鉴；
- `tv` 产出 arm64-v8a + armeabi-v7a 分包；
- `universal` 额外产出合并 APK（只含 ARM 两套 ABI）；
- 跑完在本次运行的 Artifacts 里下载 `apk-<run_number>.zip`。

工作流固定 Flutter `3.47.4`、Go `1.24.1`、NDK `28.2.13676358`，与本机脚本一致。

### 出包校验

CI 会用官方工具逐个验，任何一项不过直接失败：

- `apksigner verify` — 签名（v2 必须为 true）；
- `zipalign -c 4` — 对齐；
- `aapt2 dump badging` — minSdk、versionCode 与 ABI。

打包脚本本身还有两道硬检查：通用包**只允许** ARM 两套 ABI（依赖 AAR 会带半套 `lib/x86_64/`，安装器按它校验会报「解析软件包失败」），并且通用包 versionCode 抬高 4000，避免低于 `--split-per-abi` 的 1085/2085 被判版本降级。

### 签名

未配置 `android/key.properties` 时，工作流会生成并缓存 `android/app/ci-debug.keystore` 作为固定调试签名（Gradle 回落顺序 `release` → `ciDebug` → `debug`）。要正式上架请自行放置 `key.properties` 指向真实发布密钥。

## 源码结构

| 目录 | 内容 |
| --- | --- |
| `lib/` | 页面、播放器、本地用户、FFI、下载与媒体处理 |
| `native/core/`、`native/bridge/` | Go 站源核心、缓存、下载与 C ABI |
| `android/`、`windows/`、`ios/` | 平台工程与必要资源 |
| `scripts/`、`.github/workflows/` | 构建、签名、验证与版本快照 |
| `test/`、`integration_test/` | 自动化与设备回归 |

仓库只提交业务源码，不包含构建产物、依赖缓存、本地机器配置、签名文件与任何密钥。

## 第三方组件

播放器使用 [media_kit](https://github.com/media-kit/media-kit) / libmpv，合并与导出使用 [FFmpegKit min-gpl](https://github.com/sk3llo/ffmpeg_kit_flutter)（含 FFmpeg、x264 / x265 等 GPL 组件，适用上游许可证）。

## 更新日志的写法

`Build APK` 的 release 步骤会从 `CHANGELOG.md` 里抓出**当前版本**的所有 `### <版本> · <主题>` 小节，生成 Release 说明。新增版本时请沿用这个标题格式，否则 Release 只会有一句兜底文案。
