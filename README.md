## 之前一直没注意有人关注我这个仓库，本来只是打算自己用的，今天突然发现居然有Issues和Star，感谢各位的关注。有需求可以在Issues提，能满足的尽量满足，只要不过分吗，更新时间随缘，毕竟不是正业哈哈

# 真果鉴（Zhenguojian）

本仓库用于发布「真果鉴」Android 应用的构建产物（APK），供应用内更新检查使用。

## 原项目与致谢

本项目基于以下开源项目构建，**版权归原作者所有**：

- **原项目**：[Topkill/guoapp3](https://github.com/Topkill/guoapp3)
- **原作者**：Topkill
- **许可**：遵循原项目仓库的许可协议

本仓库仅用于存放针对个人设备（极米 Z8X 投影仪）重新构建的安装包，未对原项目主张任何权利。
如有侵权，请联系删除。

## 构建版本说明

原项目有两套编译版本（编译期决定，应用内无法切换）：

| 构建参数 | 应用名 | 站源 |
|---|---|---|
| 默认 | 红果鉴 | 仅红果 |
| `--all-sources` | **真果鉴** | 红果、黄豆、黄剧、黄果视频、黄果 AI、黄果旧版 |

本仓库发布的为 **真果鉴（全站源）** 版本。

## 下载

请前往 [Releases](https://github.com/kuoluoss/zhenguojian/releases) 页面下载最新 APK。

## 安装说明

- 目标架构：`arm64-v8a`（64 位 ARM，适配极米 Z8X 等设备）。
- 最低系统：Android 8.0（API 26）。
- 包名：`com.duanju.duanju_app`，可直接覆盖安装旧版本，无需卸载。
- 仅提供 Android 手机版（TV / 电脑版代码保留，暂停适配与验证）。

## 源码

本仓库同时提供业务源码，供查阅与自行构建。**只提交业务源码**，以下内容属于本机 / 隐私信息，一律不上传：

- 构建产物：`build/`、`dist/`、`*.apk`、`*.zip`；
- 依赖与缓存：`.dart_tool/`、`android/.gradle/`、`android/build/`、`windows/flutter/ephemeral/`、`ios/Pods/`；
- 本地环境与机器配置：`android/local.properties`、IDE 配置；
- 签名与凭据：`*.jks`、`*.keystore`、`key.properties`，以及任何 Token / 密钥 / 账号信息；
- 原生预编译产物：`android/app/src/main/jniLibs/` 下的 `.so`。

**最新源码版本：`0.6.10+58`（2026-10-16）。** 本版为缺陷修复：

- **修复红果源“目录和详情都正常、一取播放地址就返回空”** —— 根因是播放接口少了一个 `X-Argus` 请求头。
  红果 App 的网关对该头只做**存在性判断**，不校验其内容；缺头时**静默丢弃**请求
  （HTTP 200 + 空 body，无任何错误码）。而本项目的播放签名函数当时只设了
  `X-Khronos` / `X-Gorgon` / `X-SS-Req-Ticket` / `X-SS-STUB` 四个头，**唯独漏了 `X-Argus`**。
  目录接口不校验该头，所以出现“目录正常、播放为空”，被误读成“需要登录”。
- **修法**：在播放签名函数中补上 `X-Argus`（1 个常量 + 1 行）。
- **修复后实测**：目录 `code=0`；剧集详情 `code=0`，**集数与剧集列表条数一致**（实测 28 集 / 28 条）；
  播放地址 `code=0`，5 个清晰度档位齐备；直链 Range 下载 `HTTP 206`，
  `Content-Range` 的字节数与所选档位**逐字节一致**；用 `spade_a` 推出的 16 字节密钥
  可**完整解密并解码**（hevc 1080x1920 / aac，解码零报错）。
- **不涉及登录**：全程使用随机生成的设备标识，**不带任何 token / cookie**。


> 因为不含预编译原生库，克隆源码后**不能直接构建出可用的安装包**，需要先用 Go 工具链编译 `native/core` 生成 `libduanju_core.so`。

构建命令（必须在**纯英文路径**下执行，中文路径会导致构建失败）：

```powershell
py scripts\build_android.py --abi arm64-v8a --all-sources
```

## 说明

- 安装包目标架构：`arm64-v8a`（适配极米 Z8X 的 64 位 ARM 处理器）。
- 现阶段主要针对安卓手机维护，TV版暂停
- `duanju_app-update.json` 为应用内更新检查所用的版本信息文件。
