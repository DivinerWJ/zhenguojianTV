#!/usr/bin/env python3
import sys
import zipfile

if len(sys.argv) < 3:
    raise SystemExit("usage: test_apk_abis.py APK ABI [ABI ...]")

apk = sys.argv[1]
abis = sys.argv[2:]
required = [
    "libduanju_core.so",
    "libflutter.so",
    "libapp.so",
    "libmpv.so",
    "libffmpegkit.so",
]
with zipfile.ZipFile(apk) as archive:
    names = set(archive.namelist())
for abi in abis:
    for library in required:
        path = f"lib/{abi}/{library}"
        if path not in names:
            raise SystemExit(f"missing {path} in {apk}")
print(apk, ",".join(abis), "OK")
