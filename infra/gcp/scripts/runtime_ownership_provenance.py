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
import terraform_current_state_attestation as bootstrap
from prepare_runtime_ownership import prepare
from prepare_user_activity_history_adoption import private_json
from runtime_ownership_packet import validate_packet, root_reference, require_root_basis
from validate_runtime_ownership_plan import identities
import terraform_history_plan_receipt as private

CATALOG = ROOT / '.github/terraform/runtime-receipt-sources.json'
BOOTSTRAP_CATALOG = ROOT / '.github/terraform/current-state-attestation-sources.json'
VERIFIED = 'runtime-ownership-receipts.json'


def normalized(value):
    return dict(pool=value['pool'], provider=value['provider'], grants=sorted(value['grants']), apis=sorted(value['apis']))


def scope(value, candidate, selected, *, version=2):
    receipt.need(version in {2, 3})
    base = bootstrap.scope(value) if version == 3 else receipt.history_scope(value)
    if not selected['pool']:
        return base
    selected_candidate = dict(candidate, own_runtime_wif_pool=selected['pool'], own_runtime_wif_provider=selected['provider'],
                              runtime_wif_grant_keys=sorted(selected['grants']), owned_api_keys=sorted(selected['apis']))
    return dict(history=base, runtime=identities(selected_candidate),
                required_grants=candidate['runtime_wif_inventory']['grants'],
                own_apis={key: {field: candidate['api_classification'][key][field]
                               for field in ('classification', 'state', 'dependency_addresses')}
                          for key in sorted(selected['apis'])})


def verify_chain(packet, catalog, *, request, now=None, bootstrap_catalog=None):
    receipt.need(packet.get('schema_version') in {2, 3})
    candidate = validate_packet(packet, wave=packet['wave'], git_sha=packet['git_sha'], now=now)
    value = packet['receipt_binding']
    version = packet['schema_version']
    root_ref = root_reference(packet)
    if version == 3:
        history, previous_time = bootstrap.authenticate(root_ref, bootstrap_catalog, request=request, now=now)
        previous_scope = receipt.commitment(value, bootstrap.scope(value))
        receipt.need(history['scope_commitment'] == previous_scope)
        deadline = receipt.timestamp(history['expires_utc'])
        # Every unadopted consumer (both initial plan and apply) must be fresh.
        if not packet['adoption_receipts']:
            from datetime import datetime, timezone
            current_time = now or datetime.now(timezone.utc)
            receipt.need(receipt.timestamp(previous_time['completed_at']) <= current_time < deadline)
    else:
        history, previous_time = receipt.authenticate(root_ref, catalog, request=request)
        previous_scope = receipt.commitment(value, receipt.history_scope(value))
        receipt.need(history['wave'] == 'history12' and history['scope_commitment'] == previous_scope)
    previous_digest = receipt.digest(history)
    previous_state = history['state_after_commitment']
    previous = dict(pool=False, provider=False, grants=[], apis=[])
    seen = {root_ref['run_id']}
    for index, item in enumerate(packet['adoption_receipts']):
        ref, wave, selected = item['reference'], item['wave'], item['selected']
        receipt.need(ref['run_id'] not in seen)
        seen.add(ref['run_id'])
        # Reuse exactly the packet's one-object/ordering validator for each edge.
        edge = dict(packet, wave=wave, adopted=previous, selected=selected)
        validate_packet(edge, wave=wave, git_sha=packet['git_sha'], now=now, check_chain_shape=False)
        observed_jobs = []
        def observe(path, **kwargs):
            response = request(path, **kwargs)
            if '/attempts/1/jobs?' in path:
                observed_jobs.extend(response['jobs'])
            return response
        current, timing = receipt.authenticate(ref, catalog, request=observe)
        if version == 3 and index == 0:
            # The legacy authenticator already verifies this unique protected
            # plan; inspect its verified timing without changing apply checks.
            plan_name = catalog['issuers'][ref['source_sha']]['job_names']['plan']
            plans = [j for j in observed_jobs if j.get('name') == plan_name]
            receipt.need(len(plans) == 1 and receipt.timestamp(previous_time['completed_at'])
                         <= receipt.timestamp(plans[0]['started_at']) < deadline
                         and receipt.timestamp(timing['started_at']) < deadline)
        selected_scope = receipt.commitment(value, scope(value, candidate, selected, version=version))
        receipt.need(current['wave'] == wave and current['prior_receipt_sha256'] == previous_digest
                     and current['prior_scope_commitment'] == previous_scope and current['scope_commitment'] == selected_scope
                     and current['prior_state_commitment'] == previous_state
                     and current['resource_count'] == 12 + int(selected['pool']) + int(selected['provider'])
                         + len(selected['grants']) + len(selected['apis'])
                     and receipt.timestamp(previous_time['completed_at']) <= receipt.timestamp(timing['started_at']))
        previous, previous_digest, previous_scope, previous_time = selected, receipt.digest(current), selected_scope, timing
        previous_state = current['state_after_commitment']
    receipt.need(normalized(previous) == normalized(packet['adopted']))
    chain = {'last_receipt_sha256': previous_digest, 'adopted_scope_commitment': previous_scope, 'last_state_after_commitment': previous_state}
    if version == 3:
        chain.update(root_state_observation_commitment=history['state_observation_commitment'],
                     root_expires_utc=history['expires_utc'])
    return chain


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
    require_root_basis(packet, env)
    receipt.need(packet['git_sha'] == env['GITHUB_SHA'] and packet['wave'] == env['OWNERSHIP_WAVE'])
    receipt.check_current_binding(packet['receipt_binding'], env)
    catalog = receipt.load_catalog(receipt.decode(CATALOG.read_bytes()))
    return root, packet, catalog


def verify_workflow(env, *, request=None, recheck=False, now=None):
    root, packet, catalog = load_inputs(env)
    # An empty/unapproved source catalog is rejected before constructing transport.
    bootstrap_catalog = None
    if packet['schema_version'] == 3:
        bootstrap_catalog = bootstrap.load_catalog(receipt.decode(BOOTSTRAP_CATALOG.read_bytes()))
        bootstrap.reference(root_reference(packet))
        receipt.need(root_reference(packet)['source_sha'] in bootstrap_catalog['issuers'])
        refs = [i['reference'] for i in packet['adoption_receipts']]
    else:
        refs = [root_reference(packet)] + [i['reference'] for i in packet['adoption_receipts']]
    for ref in refs:
        receipt.reference(ref)
        receipt.need(ref['source_sha'] in catalog['issuers'])
    transport = request if request is not None else receipt.GitHubRead(env.get('GH_TOKEN'))
    chain = verify_chain(packet, catalog, request=transport, now=now, bootstrap_catalog=bootstrap_catalog)
    catalogs = catalog if bootstrap_catalog is None else dict(runtime=catalog, bootstrap=bootstrap_catalog)
    verified = record(packet, env, catalogs, chain)
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
    refs = [root_reference(packet)] + [i['reference'] for i in packet['adoption_receipts']]
    catalogs = catalog
    if packet['schema_version'] == 3:
        bootstrap_catalog = bootstrap.load_catalog(receipt.decode(BOOTSTRAP_CATALOG.read_bytes()))
        receipt.need(refs[0]['source_sha'] in bootstrap_catalog['issuers'])
        catalogs = dict(runtime=catalog, bootstrap=bootstrap_catalog)
        runtime_refs = refs[1:]
    else:
        runtime_refs = refs
    receipt.need(all(ref['source_sha'] in catalog['issuers'] for ref in runtime_refs))
    stored = private_json(str(root / VERIFIED))
    receipt.need(type(stored) is dict and receipt.hex_value(stored.get('last_state_after_commitment'), 64))
    chain = dict(last_receipt_sha256=refs[-1]['receipt_sha256'], last_state_after_commitment=stored['last_state_after_commitment'],
                 adopted_scope_commitment=receipt.commitment(packet['receipt_binding'], scope(packet['receipt_binding'], candidate, packet['adopted'], version=packet['schema_version'])))
    if packet['schema_version'] == 3:
        receipt.need(receipt.hex_value(stored.get('root_state_observation_commitment'), 64))
        receipt.timestamp(stored.get('root_expires_utc'))
        chain.update(root_state_observation_commitment=stored['root_state_observation_commitment'],
                     root_expires_utc=stored['root_expires_utc'])
    receipt.need(stored == record(packet, env, catalogs, chain))


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
