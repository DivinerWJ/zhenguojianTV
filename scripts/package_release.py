import argparse
import hashlib
import re
import shutil
import zipfile
from pathlib import Path

from app_build import BuildVariant, add_variant_argument

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--platform', choices=['android', 'windows'], required=True)
parser.add_argument('--abi', action='append', choices=['arm64-v8a', 'armeabi-v7a', 'x86_64'])
parser.add_argument('--universal', action='store_true',
                    help='Android：同时打包 build/app/outputs/flutter-apk/app-release.apk 为 -all.apk')
add_variant_argument(parser)
options = parser.parse_args()
variant = BuildVariant(options.all_sources)
match = re.search(r'^version:\s*([\w.+-]+)\s*$', (root / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE)
if not match:
    raise SystemExit('pubspec.yaml 缺少合法版本号。')
version = match.group(1)
output = root / 'dist' / options.platform
output.mkdir(parents=True, exist_ok=True)
artifacts = []

if options.platform == 'android':
    abis = options.abi or ['arm64-v8a', 'armeabi-v7a', 'x86_64']
    libraries = ['libduanju_core.so', 'libflutter.so', 'libapp.so', 'libmpv.so', 'libffmpegkit.so']

    def verify_android_apk(source, required_abis):
        if not source.is_file():
            raise SystemExit('缺少 APK：' + str(source))
        with zipfile.ZipFile(source) as archive:
            names = set(archive.namelist())
            required = [f'lib/{abi}/{library}' for abi in required_abis for library in libraries]
            missing = set(required) - names
            if missing:
                raise SystemExit('APK 缺少原生库：' + ', '.join(sorted(missing)))

    for abi in abis:
        source = root / 'build' / 'app' / 'outputs' / 'flutter-apk' / f'app-{abi}-release.apk'
        verify_android_apk(source, [abi])
        target = output / f'{variant.slug}-{version}-{abi}.apk'
        shutil.copy2(source, target)
        artifacts.append(target)

    if options.universal:
        # 0.8.10 第三批：通用包由 build_android.py 先构建并单独拷到
        # build/universal-release.apk（避免被随后的 --split-per-abi 覆盖）。
        # 老路径保留为兜底，方便单独跑打包脚本的场景。
        source = root / 'build' / 'universal-release.apk'
        if not source.is_file():
            source = root / 'build' / 'app' / 'outputs' / 'flutter-apk' / 'app-release.apk'
        verify_android_apk(source, abis)
        # 通用包里不允许出现"没编译过的 ABI"：那种目录只有依赖库的 .so、
        # 缺 libapp.so / libflutter.so / libduanju_core.so，安装器按它校验
        # 会判"解析软件包失败"（0.8.10 用户反馈）。这里直接拦死。
        with zipfile.ZipFile(source) as archive:
            present = {name.split('/')[1] for name in archive.namelist() if name.startswith('lib/')}
        extra = present - set(abis)
        if extra:
            raise SystemExit('通用 APK 含未编译的 ABI：' + ', '.join(sorted(extra)))
        target = output / f'{variant.slug}-{version}-all.apk'
        shutil.copy2(source, target)
        artifacts.append(target)
else:
    bundle = root / 'build' / 'windows' / 'x64' / 'runner' / 'Release'
    required = ['zhenguojian.exe', 'duanju_core.dll', 'flutter_windows.dll', 'libffmpegkit.dll',
                'libmpv-2.dll', 'msvcp140.dll', 'vcruntime140.dll',
                'data/icudtl.dat', 'data/app.so']
    missing = [name for name in required if not (bundle / name).is_file()]
    if missing:
        raise SystemExit('Windows 安装包缺少文件：' + ', '.join(missing))
    target = output / f'{variant.slug}-{version}-windows-x64.zip'
    with zipfile.ZipFile(target, 'w', zipfile.ZIP_DEFLATED) as archive:
        for source in sorted(bundle.rglob('*')):
            if source.is_file():
                relative = source.relative_to(bundle).as_posix()
                if relative == 'zhenguojian.exe':
                    relative = variant.slug + '.exe'
                archive.write(source, relative)
                print('  ' + relative)
    artifacts.append(target)

checksums = []
for artifact in sorted(output.glob(f'*-{version}-*')):
    digest = hashlib.sha256()
    with artifact.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    checksums.append(f'{digest.hexdigest()}  {artifact.name}')
    print(artifact)
(output / 'SHA256SUMS.txt').write_text('\n'.join(checksums) + '\n', encoding='ascii')
