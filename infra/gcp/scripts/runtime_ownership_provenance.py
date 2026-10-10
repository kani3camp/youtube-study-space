#!/usr/bin/env python3
"""Private cumulative scope joins to source-pinned GitHub receipts, GET only."""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / '.github/scripts'))
import terraform_ownership_receipt as receipt
from prepare_runtime_ownership import prepare
from prepare_user_activity_history_adoption import private_json
from runtime_ownership_packet import validate_packet
from validate_runtime_ownership_plan import identities
import terraform_history_plan_receipt as private

CATALOG = ROOT / '.github/terraform/runtime-receipt-sources.json'
VERIFIED = 'runtime-ownership-receipts.json'


def normalized(value):
    return dict(pool=value['pool'], provider=value['provider'], grants=sorted(value['grants']), apis=sorted(value['apis']))


def scope(value, candidate, selected):
    base = receipt.history_scope(value)
    if not selected['pool']:
        return base
    selected_candidate = dict(candidate, own_runtime_wif_pool=selected['pool'], own_runtime_wif_provider=selected['provider'],
                              runtime_wif_grant_keys=sorted(selected['grants']), owned_api_keys=sorted(selected['apis']))
    return dict(history=base, runtime=identities(selected_candidate),
                required_grants=candidate['runtime_wif_inventory']['grants'],
                own_apis={key: {field: candidate['api_classification'][key][field]
                               for field in ('classification', 'state', 'dependency_addresses')}
                          for key in sorted(selected['apis'])})


def verify_chain(packet, catalog, *, request, now=None):
    receipt.need(packet.get('schema_version') == 2)
    candidate = validate_packet(packet, wave=packet['wave'], git_sha=packet['git_sha'], now=now)
    value = packet['receipt_binding']
    history, previous_time = receipt.authenticate(packet['history_receipt'], catalog, request=request)
    previous_scope = receipt.commitment(value, receipt.history_scope(value))
    receipt.need(history['wave'] == 'history12' and history['scope_commitment'] == previous_scope)
    previous_digest = receipt.digest(history)
    previous_state = history['state_after_commitment']
    previous = dict(pool=False, provider=False, grants=[], apis=[])
    seen = {packet['history_receipt']['run_id']}
    for item in packet['adoption_receipts']:
        ref, wave, selected = item['reference'], item['wave'], item['selected']
        receipt.need(ref['run_id'] not in seen)
        seen.add(ref['run_id'])
        # Reuse exactly the packet's one-object/ordering validator for each edge.
        edge = dict(packet, wave=wave, adopted=previous, selected=selected)
        validate_packet(edge, wave=wave, git_sha=packet['git_sha'], now=now, check_chain_shape=False)
        current, timing = receipt.authenticate(ref, catalog, request=request)
        selected_scope = receipt.commitment(value, scope(value, candidate, selected))
        receipt.need(current['wave'] == wave and current['prior_receipt_sha256'] == previous_digest
                     and current['prior_scope_commitment'] == previous_scope and current['scope_commitment'] == selected_scope
                     and current['prior_state_commitment'] == previous_state
                     and current['resource_count'] == 12 + int(selected['pool']) + int(selected['provider'])
                         + len(selected['grants']) + len(selected['apis'])
                     and receipt.timestamp(previous_time['completed_at']) <= receipt.timestamp(timing['started_at']))
        previous, previous_digest, previous_scope, previous_time = selected, receipt.digest(current), selected_scope, timing
        previous_state = current['state_after_commitment']
    receipt.need(normalized(previous) == normalized(packet['adopted']))
    return {'last_receipt_sha256': previous_digest, 'adopted_scope_commitment': previous_scope, 'last_state_after_commitment': previous_state}


def record(packet, env, catalog, chain):
    receipt.context(env)
    return dict(schema_version=1, consumer_sha=env['GITHUB_SHA'], consumer_run_id=int(env['GITHUB_RUN_ID']),
                consumer_attempt=1, consumer_job=env['GITHUB_JOB'], consumer_operation=env['MODE'], packet_sha256=receipt.digest(packet), catalog_sha256=receipt.digest(catalog), **chain)


def load_inputs(env):
    receipt.context(env)
    receipt.need(env.get('GITHUB_JOB') in {'plan', 'apply'} and env.get('MODE') in {'plan', 'apply'}
                 and (env.get('GITHUB_JOB') != 'apply' or env.get('MODE') == 'apply')
                 and env.get('HISTORY_POST_NOOP', 'false') == 'false' and env.get('OWNERSHIP_WAVE') in {'pool', 'provider', 'grant', 'api', 'noop'})
    root = Path(env['RUNNER_TEMP'])
    receipt.need(root.is_absolute() and root.is_dir() and not root.is_symlink() and not root.resolve().is_relative_to(ROOT))
    packet = private_json(str(root / 'runtime-ownership-packet.json'))
    receipt.need(packet.get('schema_version') == 2 and packet['git_sha'] == env['GITHUB_SHA'] and packet['wave'] == env['OWNERSHIP_WAVE'])
    receipt.check_current_binding(packet['receipt_binding'], env)
    catalog = receipt.load_catalog(receipt.decode(CATALOG.read_bytes()))
    return root, packet, catalog


def verify_workflow(env, *, request=None, recheck=False, now=None):
    root, packet, catalog = load_inputs(env)
    # An empty/unapproved source catalog is rejected before constructing transport.
    for ref in [packet['history_receipt']] + [i['reference'] for i in packet['adoption_receipts']]:
        receipt.reference(ref)
        receipt.need(ref['source_sha'] in catalog['issuers'])
    transport = request if request is not None else receipt.GitHubRead(env.get('GH_TOKEN'))
    chain = verify_chain(packet, catalog, request=transport, now=now)
    verified = record(packet, env, catalog, chain)
    if recheck:
        receipt.need(private_json(str(root / VERIFIED)) == verified)
    else:
        private.write_private(root / VERIFIED, verified)
    return verified


def require_verified_inputs(env, packet, metadata, *, now=None):
    """Consumer binding within this trusted job; never accepts packet-supplied proof."""
    root, staged, catalog = load_inputs(env)
    receipt.need(staged == packet)
    validate_packet(packet, wave=packet['wave'], git_sha=env['GITHUB_SHA'], now=now)
    receipt.check_current_binding(packet['receipt_binding'], env, metadata)
    candidate = prepare(packet['inventory'])
    refs = [packet['history_receipt']] + [i['reference'] for i in packet['adoption_receipts']]
    receipt.need(all(ref['source_sha'] in catalog['issuers'] for ref in refs))
    stored = private_json(str(root / VERIFIED))
    receipt.need(type(stored) is dict and receipt.hex_value(stored.get('last_state_after_commitment'), 64))
    chain = dict(last_receipt_sha256=refs[-1]['receipt_sha256'], last_state_after_commitment=stored['last_state_after_commitment'],
                 adopted_scope_commitment=receipt.commitment(packet['receipt_binding'], scope(packet['receipt_binding'], candidate, packet['adopted'])))
    receipt.need(stored == record(packet, env, catalog, chain))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--recheck', action='store_true')
    args = parser.parse_args()
    try:
        verify_workflow(dict(os.environ), recheck=args.recheck)
    except Exception:
        print('STOP: ownership provenance rejected; diagnostic suppressed.', file=sys.stderr)
        return 3
    print('Exact protected ownership provenance checked; cloud execution is not authorized.')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
