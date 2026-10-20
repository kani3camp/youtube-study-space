"""Shared permission catalog and private HTTP transport.

The current controller verifies OIDC/roles/State independently. The retired
history smoke CLI is no longer an authenticated execution route.
"""
from __future__ import annotations

import json
import urllib.error
import urllib.request

WIF_MUTATION_PERMISSIONS = (
    "iam.workloadIdentityPools.create", "iam.workloadIdentityPools.update",
    "iam.workloadIdentityPools.delete", "iam.workloadIdentityPools.undelete",
    "iam.workloadIdentityPoolProviders.create", "iam.workloadIdentityPoolProviders.update",
    "iam.workloadIdentityPoolProviders.delete", "iam.workloadIdentityPoolProviders.undelete",
)

FORBIDDEN_PERMISSIONS = (
    "resourcemanager.projects.update", "resourcemanager.projects.delete",
    "resourcemanager.projects.setIamPolicy", "iam.serviceAccounts.create",
    "iam.serviceAccounts.setIamPolicy", "iam.serviceAccounts.getAccessToken",
    *WIF_MUTATION_PERMISSIONS,
    "secretmanager.versions.access", "storage.buckets.create", "bigquery.datasets.create",
    "storage.buckets.update", "storage.objects.get", "storage.objects.list",
    "storage.objects.create", "storage.objects.delete",
    "monitoring.notificationChannels.create", "monitoring.notificationChannels.update",
    "monitoring.notificationChannels.delete", "monitoring.alertPolicies.create",
    "monitoring.alertPolicies.update", "monitoring.alertPolicies.delete",
    "monitoring.alertPolicies.list", "monitoring.notificationChannels.list",
    "serviceusage.services.enable", "serviceusage.services.disable",
    "bigquery.datasets.update", "bigquery.datasets.delete", "bigquery.tables.create",
    "bigquery.tables.update", "bigquery.tables.delete", "bigquery.tables.getData",
    "bigquery.jobs.create",
)


FUNCTION_FORBIDDEN_PERMISSIONS = tuple("cloudfunctions.functions." + suffix for suffix in (
    "list", "create", "update", "delete", "call", "invoke", "sourceCodeGet", "sourceCodeSet", "getIamPolicy", "setIamPolicy"))


def google(path: str, token: str, body: dict | None = None, *, host: str = "cloudresourcemanager.googleapis.com",
           method: str | None = None) -> tuple[int, dict]:
    request = urllib.request.Request(
        f"https://{host}/{path}",
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        # Consume privately; never print dependency error text or resource identifiers.
        return error.code, json.loads(error.read())
