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

当前源码版本：`0.3.1+38`。

> 因为不含预编译原生库，克隆源码后**不能直接构建出可用的安装包**，需要先用 Go 工具链编译 `native/core` 生成 `libduanju_core.so`。

构建命令（必须在**纯英文路径**下执行，中文路径会导致构建失败）：

```powershell
py scripts\build_android.py --abi arm64-v8a --all-sources
```

## 说明

- 安装包目标架构：`arm64-v8a`（适配极米 Z8X 的 64 位 ARM 处理器）。
- `duanju_app-update.json` 为应用内更新检查所用的版本信息文件。
