"""Validate a data-only, reviewed DEV Plan permission manifest; no cloud calls."""
import argparse
import json
from pathlib import Path
import re

PROJECT = 'test-youtube-study-space'
ROLE_ID = 'terraformDevPlanMetadataReadV1'
# These reviewed resource families are a second boundary against an unclassified
# permission being added to the manifest. Never infer access from get/list alone.
FAMILIES = {'runtime': {'roots': ['run.configurations',
                       'run.jobs',
                       'run.locations',
                       'run.operations',
                       'run.revisions',
                       'run.routes',
                       'run.services',
                       'cloudfunctions.functions',
                       'cloudfunctions.locations',
                       'cloudfunctions.operations',
                       'cloudscheduler.jobs',
                       'cloudscheduler.locations'],
             'risk': 'configuration'},
 'storage': {'roots': ['storage.buckets', 'bigquery.datasets', 'bigquery.tables', 'bigquery.routines'],
             'risk': 'configuration'},
 'operations': {'roots': ['logging.buckets',
                          'logging.logMetrics',
                          'logging.settings',
                          'logging.sinks',
                          'logging.views',
                          'monitoring.alertPolicies',
                          'monitoring.dashboards',
                          'monitoring.notificationChannels',
                          'monitoring.services',
                          'monitoring.slos',
                          'monitoring.uptimeCheckConfigs'],
                'risk': 'configuration'},
 'messaging': {'roots': ['pubsub.schemas', 'pubsub.snapshots', 'pubsub.subscriptions', 'pubsub.topics'],
               'risk': 'configuration'},
 'iam': {'roots': ['iam.serviceAccounts',
                   'iam.workloadIdentityPoolProviders',
                   'iam.workloadIdentityPools',
                   'iam.roles',
                   'resourcemanager.projects',
                   'serviceusage.operations',
                   'serviceusage.services'],
         'risk': 'metadata'},
 'secrets': {'roots': ['secretmanager.locations', 'secretmanager.secrets', 'secretmanager.versions'],
             'risk': 'metadata'},
 'registry': {'roots': ['artifactregistry.locations',
                        'artifactregistry.repositories',
                        'artifactregistry.tags'],
              'risk': 'metadata'},
 'auth': {'roots': ['firebaseauth.configs'], 'risk': 'configuration'},
 'firestore': {'roots': ['datastore.backupSchedules',
                         'datastore.backups',
                         'datastore.databases',
                         'datastore.indexes',
                         'datastore.locations'],
               'risk': 'metadata'},
 'compute': {'roots': ['compute.addresses',
                       'compute.globalAddresses',
                       'compute.disks',
                       'compute.images',
                       'compute.instances',
                       'compute.instanceTemplates',
                       'compute.instanceGroups',
                       'compute.instanceGroupManagers',
                       'compute.machineTypes',
                       'compute.regions',
                       'compute.zones'],
             'risk': 'configuration'},
 'network': {'roots': ['compute.backendServices', 'compute.regionBackendServices', 'compute.networks',
                       'compute.subnetworks',
                       'compute.firewalls',
                       'compute.routes',
                       'compute.routers',
                       'compute.forwardingRules',
                       'compute.globalForwardingRules',
                       'compute.backendBuckets',
                       'compute.healthChecks',
                       'compute.urlMaps',
                       'compute.targetHttpProxies',
                       'compute.targetHttpsProxies',
                       'compute.sslPolicies',
                       'compute.networkEndpointGroups'],
             'risk': 'configuration'},
 'dns': {'roots': ['dns.managedZones',
                   'dns.policies',
                   'dns.resourceRecordSets',
                   'dns.responsePolicies',
                   'dns.responsePolicyRules'],
         'risk': 'configuration'},
 'gke': {'roots': ['container.clusters'], 'risk': 'configuration'},
 'kms': {'roots': ['cloudkms.keyRings', 'cloudkms.cryptoKeys', 'cloudkms.cryptoKeyVersions'],
         'risk': 'metadata'}}
VERBS = {'get', 'list', 'getIamPolicy', 'getAttestationRules', 'getMetadata'}
DENIED = {'datastore.databases.get', 'firebaseauth.configs.getSecret',
          'firebaseauth.configs.getHashConfig', 'container.clusters.getCredentials'}


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def no_duplicates(pairs):
    value = {}
    for key, child in pairs:
        require(key not in value, 'duplicate-json-key')
        value[key] = child
    return value


def load(path):
    return json.loads(Path(path).read_text(), object_pairs_hook=no_duplicates)


def validate(document):
    require(set(document) == {'schema_version', 'project', 'role_id', 'families', 'exclusions'}, 'manifest-fields')
    require(type(document['schema_version']) is int and document['schema_version'] == 1, 'schema-version')
    require(document['project'] == PROJECT and document['role_id'] == ROLE_ID, 'dev-plan-scope')
    families = document['families']
    require(type(families) is list and len(families) == len(FAMILIES), 'family-coverage')
    seen, permissions = set(), set()
    for family in families:
        require(set(family) == {'id', 'risk', 'returns', 'sources', 'permissions'}, 'family-fields')
        name = family['id']
        require(name in FAMILIES and name not in seen, 'unclassified-family')
        seen.add(name)
        require(family['risk'] == FAMILIES[name]['risk'], 'return-risk')
        require(type(family['returns']) is str and len(family['returns']) >= 20, 'return-review')
        require(type(family['sources']) is list and family['sources'], 'missing-source')
        require(all(type(s) is str and re.fullmatch(r'https://(?:docs\.cloud\.google\.com|firebase\.google\.com)/[^\s]+', s)
                    for s in family['sources']), 'primary-source-required')
        items = family['permissions']
        require(type(items) is list and items and all(type(p) is str for p in items), 'permission-list')
        require(items == sorted(set(items)), 'unique-sorted-permissions')
        for p in items:
            require(p not in permissions and p not in DENIED and '*' not in p, 'forbidden-permission')
            parts = p.rsplit('.', 1)
            require(len(parts) == 2 and parts[0] in FAMILIES[name]['roots'] and parts[1] in VERBS,
                    'unclassified-permission')
            require(parts[1] != 'getMetadata' or p == 'datastore.databases.getMetadata', 'metadata-verb')
            require(parts[1] != 'getAttestationRules' or p == 'iam.workloadIdentityPools.getAttestationRules', 'attestation-verb')
            permissions.add(p)
    excluded = set()
    require(type(document['exclusions']) is list and document['exclusions'], 'missing-exclusions')
    for item in document['exclusions']:
        require(set(item) == {'permissions', 'reason'} and type(item['reason']) is str and item['reason'], 'exclusion-review')
        require(type(item['permissions']) is list and item['permissions'], 'excluded-permissions')
        for p in item['permissions']:
            require(type(p) is str and p not in permissions and p not in excluded, 'excluded-overlap')
            excluded.add(p)
    require(DENIED <= excluded, 'required-exclusions')
    require(0 < len(permissions) <= 300, 'custom-role-size')
    return sorted(permissions)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, required=True)
    args = parser.parse_args()
    try:
        permissions = validate(load(args.manifest))
    except (ValueError, TypeError, KeyError, OSError):
        raise SystemExit('PLAN_POLICY: STOP; manifest invalid')
    print(f'PLAN_POLICY: PASS; permissions={len(permissions)}; cloud_calls=0')
