#!/usr/bin/env python3
"""Pin the one commit that terraform.yml validates and then plans or applies."""
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
    # The controller checkout supplies this resolver; never import target code.
    # Reuse owner/ref/caller/selector checks before any target checkout or credentials.
    # Production requires the dispatched main commit itself (ops.context).
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
        print('::error::Could not pin the Terraform source commit.')
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
