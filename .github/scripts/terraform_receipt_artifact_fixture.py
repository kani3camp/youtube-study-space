#!/usr/bin/env python3
"""Publishable public dummy bytes only; never a trusted ownership receipt."""
import os
from pathlib import Path

from terraform_ownership_receipt import canonical, need, PUBLIC_FILE, REPOSITORY
from test_terraform_ownership_receipt import dummy_receipt


def prepare(env):
    need(env.get('GITHUB_EVENT_NAME') == 'pull_request' and env.get('GITHUB_REPOSITORY') == REPOSITORY
         and env.get('GITHUB_JOB') == 'receipt-artifact-fixture' and env.get('GITHUB_RUN_ATTEMPT') == '1')
    root = Path(env['RUNNER_TEMP'])
    need(root.is_absolute() and root.is_dir() and not root.is_symlink()
         and not root.resolve().is_relative_to(Path(__file__).resolve().parents[2]))
    path = root / 'receipt-fixture-public'
    path.mkdir(mode=0o700)
    fd = os.open(path / PUBLIC_FILE, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(canonical(dummy_receipt()) + b'\n')


if __name__ == '__main__':
    prepare(dict(os.environ))
    print('Public dummy receipt fixture prepared; this is not ownership proof.')
