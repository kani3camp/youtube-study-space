#!/usr/bin/env python3
"""Public dummy bytes only; never an admitted current-state issuer."""
import os
from pathlib import Path

import terraform_current_state_attestation as current_state
from terraform_ownership_receipt import canonical, need, REPOSITORY
from test_terraform_current_state_attestation import dummy_attestation


def prepare(env):
    need(env.get('GITHUB_EVENT_NAME') == 'pull_request' and env.get('GITHUB_REPOSITORY') == REPOSITORY
         and env.get('GITHUB_JOB') == 'current-state-artifact-fixture' and env.get('GITHUB_RUN_ATTEMPT') == '1')
    root = Path(env['RUNNER_TEMP'])
    need(root.is_absolute() and root.is_dir() and not root.is_symlink()
         and not root.resolve().is_relative_to(Path(__file__).resolve().parents[2]))
    directory = root / 'current-state-fixture-public'
    directory.mkdir(mode=0o700)
    value = current_state.validate_attestation(dummy_attestation())
    fd = os.open(directory / current_state.PUBLIC_FILE, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(canonical(value) + b'\n')


if __name__ == '__main__':
    prepare(dict(os.environ))
    print('Public dummy current-state fixture prepared; this is not adoption proof.')
