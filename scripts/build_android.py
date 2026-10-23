import argparse
import os
import platform
import shutil
import subprocess
import sys
from pathlib import Path

from build_mirrors import china_mirror_environment, mirrored_pub_lockfile
from app_build import BuildVariant, add_variant_argument

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--abi', action='append', choices=['arm64-v8a', 'armeabi-v7a', 'x86_64'])
parser.add_argument('--tv', action='store_true',
                    help='电视版：同时产出 arm64-v8a 与 armeabi-v7a（电视盒子常见 32 位，只发 64 位会装不上）')
parser.add_argument('--universal', action='store_true',
                    help='额外产出一个包含所选 ABI 的 all/universal APK')
parser.add_argument('--cn-mirrors', action='store_true', help='使用 Flutter 中国镜像和阿里云 Maven 镜像')
add_variant_argument(parser)
options = parser.parse_args()
if options.tv:
    for abi in ('arm64-v8a', 'armeabi-v7a'):
        if abi not in (options.abi or []):
            options.abi = [*(options.abi or []), abi]
variant = BuildVariant(options.all_sources)
environment = os.environ.copy()
if platform.system() == 'Darwin':
    environment['LANG'] = 'en_US.UTF-8'
    environment['LC_ALL'] = 'en_US.UTF-8'
environment.setdefault('GOPROXY', 'https://goproxy.cn,direct')
environment.setdefault('GOSUMDB', 'off')
flutter = shutil.which('flutter')
if not flutter:
    raise SystemExit('请先将 Flutter SDK 的 bin 目录加入 PATH。')
abi_args = [item for abi in options.abi or [] for item in ['--abi', abi]]
with china_mirror_environment(environment, options.cn_mirrors) as env, mirrored_pub_lockfile(root, env):
    if options.cn_mirrors:
        print('本次构建启用国内依赖镜像，保留官方 Maven 仓库备用。', flush=True)
    subprocess.run([sys.executable, str(root / 'scripts' / 'build_native.py'), '--platform', 'android', *abi_args, *variant.arguments],
                   cwd=root, env=env, check=True)
    subprocess.run([flutter, 'pub', 'get', '--enforce-lockfile'], cwd=root, env=env, check=True)
    targets = {'arm64-v8a': 'android-arm64', 'armeabi-v7a': 'android-arm', 'x86_64': 'android-x64'}
    target_platform = []
    if options.abi:
        target_platform = ['--target-platform', ','.join(targets[abi] for abi in options.abi)]

    # 0.8.10 第三批：通用（all）包改成**先**构建，并且只打 ARM 两套 ABI。
    #
    # 两个原因：
    # 1) 依赖的 AAR 会带上 x86_64 的 .so，而 app 自身的 libapp.so / libflutter.so /
    #    libduanju_core.so 只有 ARM 两份 —— 通用包里于是留着一个半套的
    #    lib/x86_64/，既白占体积，也可能让安装器按错误 ABI 校验后判"解析失败"。
    #    这里用 Gradle 项目属性注入白名单（见 android/app/build.gradle.kts）。
    # 2) 先通用后分包：`--split-per-abi` 只写 app-<abi>-release.apk，不会覆盖
    #    app-release.apk；顺序固定下来，打包阶段拿到的就一定是这次刚出的通用包。
    if options.universal:
        universal_env = dict(env)
        universal_env['ORG_GRADLE_PROJECT_universalAbiFilters'] = ','.join(
            options.abi or ['arm64-v8a', 'armeabi-v7a'])
        # 通用包 versionCode 抬高 4000：Flutter 给分包加的是 1000/2000/3000，
        # 通用包不加偏移时（85）会被当成"版本降级"，装过再装就报解析失败。
        universal_env['ORG_GRADLE_PROJECT_universalVersionCodeOffset'] = '4000'
        universal_args = [flutter, 'build', 'apk', '--release', '--no-pub',
                          *variant.flutter_arguments, *target_platform]
        subprocess.run(universal_args, cwd=root, env=universal_env, check=True)

    build_args = [flutter, 'build', 'apk', '--release', '--split-per-abi', '--no-pub',
                  *variant.flutter_arguments, *target_platform]
    subprocess.run(build_args, cwd=root, env=env, check=True)

    package_args = [sys.executable, str(root / 'scripts' / 'package_release.py'),
                    '--platform', 'android', *abi_args, *variant.arguments]
    if options.universal:
        package_args.append('--universal')
    subprocess.run(package_args, cwd=root, env=env, check=True)
