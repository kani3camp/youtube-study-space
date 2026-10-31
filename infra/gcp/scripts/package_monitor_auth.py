#!/usr/bin/env python3
"""Terraformが配備する認証用ソースを、端末・時刻によらず同じZIPへまとめる。"""
from pathlib import Path
import hashlib
import zipfile

ROOT = Path(__file__).resolve().parents[1]
FILES = ('guard.cjs', 'index.cjs', 'package.json', 'package-lock.json')


def package(root=ROOT):
    directory = root / 'modules/monitor-auth-registration/function'
    output = directory / 'source.zip'
    with zipfile.ZipFile(output, 'w', compression=zipfile.ZIP_STORED) as archive:
        for name in FILES:
            path = directory / name
            if not path.is_file() or path.is_symlink():
                raise ValueError('認証用ソースが不足しています')
            info = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
            info.create_system = 3
            info.external_attr = 0o100644 << 16
            archive.writestr(info, path.read_bytes())
    return hashlib.sha256(output.read_bytes()).hexdigest()


if __name__ == '__main__':
    print('MONITOR_AUTH_SOURCE: sha256=' + package())
