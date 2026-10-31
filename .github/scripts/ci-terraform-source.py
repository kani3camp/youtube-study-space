#!/usr/bin/env python3
"""Pin credentialless CI and authenticated Terraform to the same source commit."""
import os
from pathlib import Path
import re
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'infra/gcp/scripts'))
import terraform_normal_ops as ops
import terraform_source as source


def resolve(env):
    sha = env.get('GITHUB_SHA', '')
    ops.need(re.fullmatch('[0-9a-f]{40}', sha) is not None)
    if (env.get('GITHUB_EVENT_NAME') == 'workflow_dispatch'
            and env.get('CI_EXECUTION_MODE') in ('検査+plan', '検査+apply')):
        # The controller checkout supplies this resolver; never import target code.
        # Reuse owner/ref/selector checks before any target checkout or credentials.
        ops.context(env)
        if env['NORMAL_TARGET'] == 'dev':
            selected = source.resolve(env)
            source.publish(env, selected)
            sha = selected['sha']
    return sha


def main():
    try:
        env = dict(os.environ)
        sha = resolve(env)
        with open(env['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
            output.write('validation_sha=' + sha + '\n')
    except Exception:
        print('::error::Could not pin the CI/Terraform source commit.')
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
