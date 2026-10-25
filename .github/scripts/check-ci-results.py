#!/usr/bin/env python3
"""Fail closed on missing detection, failed callees, or skipped selected CI."""
import json
import os
import sys

GROUPS = (
    'system', 'room_image_prompt', 'menu_image_generator', 'video_maker_simulator',
    'figma_plugin', 'youtube_monitor', 'mypage', 'docs_site', 'aws_cdk', 'gcp_terraform',
    'node_projects', 'firestore_integration', 'formal_spec', 'all',
)
DOMAIN_GROUPS = {
    'system': ('system', 'firestore_integration'),
    'frontend': ('youtube_monitor', 'mypage', 'docs_site', 'node_projects'),
    'tools': ('room_image_prompt', 'menu_image_generator', 'video_maker_simulator',
              'figma_plugin', 'formal_spec', 'node_projects'),
    'infra': ('aws_cdk', 'node_projects'),
    'gcp-terraform-validate': ('gcp_terraform',),
}
DEPENDENCIES = {'changes', 'repository', 'gcp-user-activity-schema-audit', 'mypage-foundation-authenticated', 'mypage-hosting-authenticated', *DOMAIN_GROUPS}


def validate(needs, audit_requested, foundation_requested='false', hosting_requested='false'):
    if set(needs) != DEPENDENCIES:
        raise ValueError('CI Gate must receive every expected dependency')
    if audit_requested not in ('true', 'false'):
        raise ValueError('Schema audit selection must be an explicit boolean')
    if foundation_requested not in ('true', 'false'):
        raise ValueError('MyPage foundation selection must be an explicit boolean')
    if hosting_requested not in ('true', 'false'):
        raise ValueError('MyPage Hosting selection must be an explicit boolean')
    for name, job in needs.items():
        result = job.get('result')
        if result not in ('success', 'skipped'):
            raise ValueError(f'{name}: rejected result {result!r}')
    for name in ('changes', 'repository'):
        if needs[name]['result'] != 'success':
            raise ValueError(f'{name}: must succeed')
    outputs = needs['changes'].get('outputs', {})
    if set(outputs) != set(GROUPS) or any(value not in ('true', 'false') for value in outputs.values()):
        raise ValueError('Change detection outputs are missing or malformed')
    for name, groups in DOMAIN_GROUPS.items():
        selected = outputs['all'] == 'true' or any(outputs[group] == 'true' for group in groups)
        if selected and needs[name]['result'] != 'success':
            raise ValueError(f'{name}: selected validation must succeed')
    if audit_requested == 'true' and needs['gcp-user-activity-schema-audit']['result'] != 'success':
        raise ValueError('Requested schema audit must succeed')
    if foundation_requested == 'true' and needs['mypage-foundation-authenticated']['result'] != 'success':
        raise ValueError('Requested MyPage foundation must succeed')
    if hosting_requested == 'true' and needs['mypage-hosting-authenticated']['result'] != 'success':
        raise ValueError('Requested MyPage Hosting release must succeed')


def main():
    try:
        validate(json.loads(os.environ['CI_NEEDS']), os.environ['CI_SCHEMA_AUDIT_REQUESTED'], os.environ.get('CI_MYPAGE_FOUNDATION_REQUESTED', 'false'), os.environ.get('CI_MYPAGE_HOSTING_REQUESTED', 'false'))
    except (KeyError, TypeError, ValueError, AttributeError) as error:
        print(f'::error::{error}', file=sys.stderr)
        return 1
    print('All selected CI validations succeeded; unselected validations may be skipped.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
