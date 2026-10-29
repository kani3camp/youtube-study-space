#!/usr/bin/env python3
"""FSLモデルの回帰操作列と安全性変異を検証し、実アダプターは検証しない。"""

import argparse
import json
from pathlib import Path
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fslc', required=True)
    parser.add_argument('--oauth-depth', required=True, type=int)
    args = parser.parse_args()
    specs = Path(__file__).resolve().parents[1] / 'specs'

    def call(*params):
        result = subprocess.run([args.fslc, *map(str, params)], capture_output=True, text=True, timeout=60)
        data = json.loads(result.stdout)
        return result.returncode, data

    with tempfile.TemporaryDirectory(prefix='mypage-models-') as temp:
        trace_file = Path(temp) / 'trace.json'

        def step(action, **params):
            return {'action': action, 'params': params}

        def replay(name, spec, trace, expected=None, rejected=False):
            trace_file.write_text(json.dumps(trace))
            code, data = call('replay', specs / f'{spec}.fsl', '--trace', trace_file)
            if rejected:
                assert code == 1 and data['result'] == 'nonconformant', (name, data)
                assert data['failed_at_event'] == len(trace) - 1, (name, data)
            else:
                assert code == 0 and data['result'] == 'conformant', (name, data)
                for key, value in (expected or {}).items():
                    assert data['final_state'][key] == value, (name, key, data)
            print(f'PASS {name}')

        session = 'mypage_session'
        enter = step('enter_maintenance')
        login = step('set_identity', next=1, fresh_login=False)
        hide = step('suspend', fresh_login=False)
        fetch = step('begin_request', r=1)
        replay('logout preserves active maintenance', session, [enter, step('logout')], {'maintenance_active': True, 'phase': 'Anonymous'})
        replay('no fetch after maintenance logout and identity restore', session, [enter, step('logout'), login, fetch], rejected=True)
        replay('pagehide from anonymous', session, [hide], {'phase': 'Suspended'})
        replay('pagehide from maintenance keeps outer monitor', session, [enter, hide], {'phase': 'Suspended', 'maintenance_active': True})
        replay('no fetch after maintenance pagehide and identity restore', session, [enter, hide, login, fetch], rejected=True)
        replay('bootstrapping can receive a restriction', session, [hide, step('restrict_identity', target=1)], {'phase': 'Restricted', 'uid': 1})
        replay('no fetch after bootstrapping restriction', session, [hide, step('restrict_identity', target=1), fetch], rejected=True)
        replay('explicit maintenance exit permits fetch', session, [enter, step('logout'), login, step('resume_maintenance'), fetch], {'maintenance_active': False})

        oauth = 'mypage_oauth'
        verified = [step('start', selected='Login'), step('claim', state_matches=True), step('verify_channel', unique_channel=True)]
        confirm = step('confirm_login', confirmation_matches=True)
        mint = step('attempt_mint', succeeds=True)
        replay('access recovery before confirm permits login', oauth, verified + [step('set_access', allowed=False), step('set_access', allowed=True), confirm, mint, step('finish_mint')], {'mint': 'Delivered'})
        replay('access recovery cannot renew consumed checkpoint', oauth, verified + [confirm, mint, step('set_access', allowed=False), step('set_access', allowed=True), step('finish_mint')], {'mint': 'Rejected', 'checkpoint_current': False})

        access = 'mypage_access'
        block = step('set_moderation', active=True, code=1, reference=1)
        refresh_ref = step('set_moderation', active=True, code=1, reference=2)
        refresh_code = step('set_moderation', active=True, code=2, reference=2)
        replay('reference and code changes advance checkpoint; identical replay does not', access, [block, refresh_ref, refresh_code, refresh_code], {'checkpoint': 3, 'moderation': True})
        deletion = step('set_deletion', active=True, reference=1)
        deletion_refresh = step('set_deletion', active=True, reference=2)
        replay('deletion reference advances checkpoint and preserves moderation', access, [block, deletion, deletion_refresh, deletion_refresh], {'checkpoint': 3, 'moderation': True, 'deletion': True})

        # 各変異モデルでは、構文エラーではなく、名前の付いた安全性条件の違反を検出する。
        mutants = [
            ('logout erases maintenance', session, '  action logout() {', '  action logout() {\n    maintenance_active = false', 'MaintenancePersistsUntilExplicitExit'),
            ('pagehide erases maintenance', session, '  action suspend(fresh_login: Bool) {', '  action suspend(fresh_login: Bool) {\n    maintenance_active = false', 'MaintenancePersistsUntilExplicitExit'),
            ('fetch ignores outer maintenance', session, '    requires not maintenance_active\n', '', None),
            ('created purpose is swapped', oauth, '  trans ConsumedNeverReopens {', '  action swap_purpose() {\n    requires status == Pending\n    requires purpose == Support\n    purpose = Login\n  }\n\n  trans ConsumedNeverReopens {', 'PurposeNeverChangesAfterCreation'),
            ('moderation reference does not advance checkpoint', access, 'active != moderation or code != moderation_code or reference != moderation_ref', 'active != moderation or code != moderation_code', 'ChangedReasonDetailsAdvanceCheckpoint'),
            ('moderation code does not advance checkpoint', access, 'active != moderation or code != moderation_code or reference != moderation_ref', 'active != moderation or reference != moderation_ref', 'ChangedReasonDetailsAdvanceCheckpoint'),
            ('deletion reference does not advance checkpoint', access, 'active != deletion or reference != deletion_ref', 'active != deletion', 'ChangedReasonDetailsAdvanceCheckpoint'),
        ]
        for name, spec, before, after, prop in mutants:
            text = (specs / f'{spec}.fsl').read_text()
            assert text.count(before) == 1, (name, 'mutation site changed')
            path = Path(temp) / f'{spec}.fsl'
            path.write_text(text.replace(before, after))
            code, data = call('verify', path, '--depth', 8, '--vacuity', 'error')
            assert code == 1 and data['result'] == 'violated', (name, data)
            if prop:
                assert data.get('trans') == prop, (name, data)
            print(f'PASS rejects {name}')

        code, data = call('verify', specs / 'mypage_oauth.fsl', '--depth', args.oauth_depth, '--property', 'InFlightMintCanBeRejected')
        assert code == 0 and data['result'] == 'verified', data
        steps = data['reachables']['InFlightMintCanBeRejected']['witnessed_at_step']
        assert steps + 4 <= args.oauth_depth, (steps, args.oauth_depth, 'need at least four steps of headroom')
        print(f'PASS OAuth reachability headroom: {steps}/{args.oauth_depth}')


if __name__ == '__main__':
    main()
