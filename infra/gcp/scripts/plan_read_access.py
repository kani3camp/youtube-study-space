"""DEV Plan専用の共通roleを検証・差分生成する。既定はread-only。"""
import argparse
from copy import deepcopy
from datetime import datetime, timezone, timedelta
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / '.github/ci-validation'))
import plan_policy as policy

MANIFEST = ROOT / 'infra/gcp/policies/plan-metadata-read.json'
PROJECT = policy.PROJECT
ROLE_ID = policy.ROLE_ID
ROLE = f'projects/{PROJECT}/roles/{ROLE_ID}'
LEGACY = {
    'terraform-dev-normal-plan': {'terraformNormalPlan', 'monitorAuthConfigRead'},
    'mypage-foundation-plan': {'mypageFoundationRead', 'mypageRegistryRead', 'mypageRuntimeRead'},
}
TARGETS = {f'serviceAccount:{name}@{PROJECT}.iam.gserviceaccount.com':
           {f'projects/{PROJECT}/roles/{role}' for role in roles}
           for name, roles in LEGACY.items()}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def definition():
    document = policy.load(MANIFEST)
    return {'title': 'Terraform DEV Plan metadata read',
            'description': 'Reviewed metadata only; configuration secret hygiene required.',
            'stage': 'GA', 'includedPermissions': policy.validate(document)}


def gcloud(*args, missing=False):
    # No exception prints stderr/stdout: IAM inputs and identifiers stay private.
    result = subprocess.run(['gcloud', *args, '--project=' + PROJECT, '--format=json', '--quiet'],
                            capture_output=True, text=True, timeout=120,
                            env=os.environ | {'CLOUDSDK_CORE_DISABLE_PROMPTS': '1'})
    if missing and result.returncode and 'NOT_FOUND' in result.stderr:
        return None
    policy.require(result.returncode == 0, 'cloud-command-failed')
    return json.loads(result.stdout) if result.stdout.strip() else {}


def collect():
    """Metadata only. This path never calls a mutating API, secret or State read."""
    result = {'project': gcloud('projects', 'describe', PROJECT),
              'policy': gcloud('projects', 'get-iam-policy', PROJECT),
              'roles': {}, 'accounts': {}, 'analysis': {}}
    role_names = {ROLE} | set.union(*TARGETS.values())
    for name in sorted(role_names):
        result['roles'][name] = gcloud('iam', 'roles', 'describe', name.split('/')[-1], missing=name == ROLE)
    for member in TARGETS:
        email = member.removeprefix('serviceAccount:')
        result['accounts'][member] = gcloud('iam', 'service-accounts', 'describe', email)
        result['accounts'][member]['keys'] = gcloud('iam', 'service-accounts', 'keys', 'list',
                                                   '--iam-account=' + email)
        result['analysis'][member] = gcloud('asset', 'analyze-iam-policy', '--identity=' + member,
                                           '--expand-roles', '--expand-groups', '--show-response')
    result['testable_permissions'] = gcloud('iam', 'list-testable-permissions',
        '//cloudresourcemanager.googleapis.com/projects/' + str(result['project']['projectNumber']))
    return result


def grants(document, member):
    result = []
    for binding in document['bindings']:
        if member in binding['members']:
            policy.require(not binding.get('condition'), 'conditional-target-grant')
            result.append(binding['role'])
    policy.require(len(result) == len(set(result)), 'duplicate-target-grant')
    return set(result)


def normalized(document):
    """Compare every binding/audit config, ignoring only etag and array order."""
    result = deepcopy(document)
    result.pop('etag', None)
    for binding in result['bindings']:
        binding['members'].sort()
    result['bindings'].sort(key=lambda b: json.dumps(b, sort_keys=True))
    return result


def analyze(result, member, expected, number):
    policy.require(result.get('fullyExplored') is True, 'partial-iam-analysis')
    main = result['mainAnalysis']
    policy.require(main.get('fullyExplored') is True and not main.get('nonCriticalErrors'), 'iam-analysis-errors')
    query = main['analysisQuery']
    policy.require(query.get('identitySelector') == {'identity': member}, 'analysis-identity')
    policy.require(query.get('scope') in {'projects/' + PROJECT, 'projects/' + number}, 'analysis-scope')
    policy.require(not query.get('resourceSelector') and not query.get('accessSelector'), 'filtered-analysis')
    policy.require(query.get('options', {}).get('expandRoles') is True, 'unexpanded-analysis')
    found = set()
    for entry in main.get('analysisResults', []):
        policy.require(entry.get('fullyExplored') is True, 'partial-binding-analysis')
        policy.require(entry['attachedResourceFullName'] in {
            '//cloudresourcemanager.googleapis.com/projects/' + number,
            '//cloudresourcemanager.googleapis.com/projects/' + PROJECT}, 'unexpected-resource-grant')
        binding = entry['iamBinding']
        policy.require(member in binding['members'] and not binding.get('condition'), 'indirect-or-conditional-grant')
        policy.require(binding['role'] in expected, 'unexpected-analyzed-role')
        found.add(binding['role'])
    policy.require(found == expected, 'analysis-project-policy-mismatch')


def prepare(snapshot):
    desired = definition()
    project = snapshot['project']
    policy.require(project.get('projectId') == PROJECT and project.get('lifecycleState') == 'ACTIVE', 'wrong-project')
    # This contract targets the current standalone DEV project. A new parent
    # requires an inventory/review rather than silently ignoring inherited IAM.
    policy.require(not project.get('parent'), 'parent-inventory-required')
    number = str(project['projectNumber'])
    policy.require(number.isdigit(), 'project-number')
    before = snapshot['policy']
    policy.require(before.get('etag') and before.get('bindings') and before.get('version', 1) in (1, 3), 'policy-etag')
    allowed = set(desired['includedPermissions'])
    testable = {p['name']: p for p in snapshot['testable_permissions']}
    for permission in allowed:
        p = testable.get(permission, {})
        policy.require(p.get('customRolesSupportLevel', 'SUPPORTED') == 'SUPPORTED'
                       and p.get('stage') == 'GA', 'unsupported-permission')
    after = deepcopy(before)
    changes = {}
    for member, legacy in TARGETS.items():
        current = grants(before, member)
        policy.require(current in (legacy, {ROLE}), 'unexpected-project-grant')
        account = snapshot['accounts'][member]
        policy.require(account.get('email') == member.removeprefix('serviceAccount:')
                       and account.get('disabled', False) is False, 'account-identity')
        policy.require(all(k.get('keyType') == 'SYSTEM_MANAGED' for k in account['keys']), 'user-managed-key')
        permissions = set()
        for name in current:
            role = snapshot['roles'][name]
            policy.require(role and role.get('name') == name and not role.get('deleted')
                           and role.get('stage') == 'GA', 'role-state')
            permissions.update(role['includedPermissions'])
        policy.require(permissions <= allowed, 'unexpected-existing-permission')
        analyze(snapshot['analysis'][member], member, current, number)
        changes[member.split(':', 1)[1].split('@')[0]] = {
            'add_permissions': sorted(allowed - permissions), 'remove_permissions': sorted(permissions - allowed)}
        for binding in after['bindings']:
            if member in binding['members']:
                binding['members'].remove(member)
    # A custom role may be shared on child resources too; the separate coverage
    # review is mandatory before reflection. Never update a shared legacy role.
    for binding in before['bindings']:
        if binding['role'] == ROLE:
            policy.require(not binding.get('condition') and set(binding['members']) <= set(TARGETS), 'shared-plan-role')
    existing = snapshot['roles'][ROLE]
    if existing is not None:
        policy.require(existing.get('name') == ROLE and existing.get('stage') == 'GA'
                       and not existing.get('deleted')
                       and set(existing['includedPermissions']) <= allowed, 'unexpected-plan-role')
    after['bindings'] = [b for b in after['bindings'] if b['members']]
    after['bindings'].append({'role': ROLE, 'members': sorted(TARGETS)})
    # Prove only target edges changed, including shared-role members and Apply.
    def without_targets(p):
        value = deepcopy(p)
        for b in value['bindings']:
            b['members'] = [m for m in b['members'] if m not in TARGETS]
        value['bindings'] = [b for b in value['bindings'] if b['members']]
        return normalized(value)
    policy.require(without_targets(before) == without_targets(after), 'non-plan-delta')
    return {'role': desired, 'project_policy': after, 'changes': changes,
            'confirmation': digest({'snapshot': snapshot, 'manifest': policy.load(MANIFEST)})}


def review_evidence(evidence, confirmation, now=None):
    """Operator review record; deliberately not represented as an automatic proof."""
    now = now or datetime.now(timezone.utc)
    policy.require(evidence.get('confirmation') == confirmation, 'review-snapshot-mismatch')
    reviewed = datetime.fromisoformat(evidence['reviewed_at'])
    policy.require(reviewed.tzinfo is not None and timedelta(0) <= now - reviewed <= timedelta(hours=24), 'stale-review')
    required = {'resource_iam_review_complete', 'cross_project_grants_review_complete',
                'plaintext_secrets_absent', 'no_other_members_of_plan_role', 'legacy_paths_reviewed'}
    policy.require(all(evidence.get(k) is True for k in required), 'incomplete-operator-review')
    policy.require(type(evidence.get('evidence_reference')) is str and evidence['evidence_reference'].strip(), 'missing-review-reference')


def private_json(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as f:
        json.dump(value, f, sort_keys=True, indent=2)
        f.write('\n')


def apply(snapshot, packet, backup):
    # Call only after CLI's live collection + confirmation + evidence checks.
    backup.mkdir(mode=0o700, parents=False, exist_ok=False)
    private_json(backup / 'before.json', snapshot)
    private_json(backup / 'proposed.json', packet)
    role = deepcopy(packet['role'])
    old = snapshot['roles'][ROLE]
    if old:
        policy.require(old.get('etag'), 'role-etag')
        role['etag'] = old['etag']
    with tempfile.TemporaryDirectory() as temp:
        path = Path(temp) / 'role.json'
        private_json(path, role)
        gcloud('iam', 'roles', 'update' if old else 'create', ROLE_ID, '--file=' + str(path))
        actual = gcloud('iam', 'roles', 'describe', ROLE_ID)
        policy.require(set(actual['includedPermissions']) == set(role['includedPermissions'])
                       and actual.get('stage') == 'GA' and not actual.get('deleted'), 'role-readback')
        path = Path(temp) / 'policy.json'
        private_json(path, packet['project_policy'])
        gcloud('projects', 'set-iam-policy', PROJECT, str(path))
    actual = gcloud('projects', 'get-iam-policy', PROJECT)
    policy.require(normalized(actual) == normalized(packet['project_policy']), 'policy-readback')


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group()
    source.add_argument('--live', action='store_true', help='fresh read-only IAM collection')
    source.add_argument('--snapshot', type=Path, help='offline snapshot; cannot apply')
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--confirm')
    parser.add_argument('--review', type=Path)
    parser.add_argument('--backup', type=Path)
    parser.add_argument('--export', type=Path, help='private local snapshot for review, never commit')
    args = parser.parse_args(argv)
    policy.require(not args.apply or (args.live and args.confirm and args.review and args.backup), 'live-confirm-review-backup-required')
    if not args.live and not args.snapshot:
        policy.require(not any((args.apply, args.confirm, args.review, args.backup, args.export)), 'source-required')
        print(json.dumps({'permissions': len(definition()['includedPermissions']), 'cloud_calls': 0}))
        return
    snapshot = collect() if args.live else policy.load(args.snapshot)
    packet = prepare(snapshot)
    if args.export:
        private_json(args.export, snapshot)
    reviewed = False
    if args.review:
        review_evidence(policy.load(args.review), packet['confirmation'])
        reviewed = True
    if args.apply:
        policy.require(args.confirm == packet['confirmation'] and reviewed, 'confirmation-mismatch')
        apply(snapshot, packet, args.backup)
    print(json.dumps({'project': PROJECT, 'role': ROLE_ID, 'changes': packet['changes'],
                      'confirmation': packet['confirmation'], 'operator_review_verified': reviewed,
                      'environment_updated': args.apply, 'blocked_pending_operator_review': not reviewed}, sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError):
        raise SystemExit('PLAN_READ_ACCESS: STOP; inventory, classification, review or readback incomplete')
