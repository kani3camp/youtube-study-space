#!/usr/bin/env python3
"""Offline controller boundaries; every external process and HTTP call is mocked."""
from __future__ import annotations

import ast
import base64
from contextlib import ExitStack, redirect_stderr, redirect_stdout
from copy import deepcopy
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import Mock, patch
import urllib.error

sys.dont_write_bytecode = True
import terraform_migration as migration

CANARY = "PRIVATE_CONTROLLER_CANARY_NEVER_PUBLIC"
SHA = "a" * 40
PROJECT_NUMBER = "111111111111"


def manifest(target="dev"):
    return {"schema_version": "gcp-migration-manifest-v1", "environment": target,
            "project_id": migration.PROJECTS[target], "resources": []}


def environment(*, target="dev", role="plan", mode=None):
    return {
        "MIGRATION_TARGET": target, "MIGRATION_MODE": mode or ("migration-apply" if role == "apply" else "migration-plan"),
        "MIGRATION_ROLE": role, "MIGRATION_PRODUCTION_READY": "false",
        "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
        "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
        "GITHUB_ACTOR_ID": "54093651", "GITHUB_ACTOR": "kani3camp", "GITHUB_TRIGGERING_ACTOR": "kani3camp",
        "GITHUB_RUN_ATTEMPT": "1", "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac",
        "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
        "GITHUB_SHA": SHA, "GITHUB_JOB": "migration_" + role, "TF_WORKSPACE": "default",
        "STATE_AWS_REGION": "ap-northeast-1", "AWS_MAX_ATTEMPTS": "1", "AWS_RETRY_MODE": "standard",
        "STATE_BUCKET": "fixture-state-bucket", "STATE_ACCOUNT_ID": "111111111111",
        "MIGRATION_INPUTS_JSON": json.dumps({"schema_version": "gcp-migration-inputs-v1",
            "tfvars": {"project_id": migration.PROJECTS[target]}, "manifest": manifest(target),
            "plan_integrity_key": "b" * 64}),
    }


class OfflineTest(unittest.TestCase):
    def setUp(self):
        self.stack = ExitStack()
        self.addCleanup(self.stack.close)
        self.stack.enter_context(patch.object(migration.subprocess, "run", side_effect=AssertionError("External process forbidden")))
        self.stack.enter_context(patch.object(migration.urllib.request, "urlopen", side_effect=AssertionError("External HTTP forbidden")))
        self.temp = tempfile.TemporaryDirectory(prefix="migration-controller-contract-")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)


class ContextAndPrivateInputTest(OfflineTest):
    def test_only_owner_initial_dispatch_and_exact_job_are_admitted(self):
        for role in ("plan", "apply"):
            self.assertEqual(migration.context(environment(role=role), role), "dev")
            for key, value in {
                "GITHUB_ACTOR_ID": "1", "GITHUB_ACTOR": "another-user", "GITHUB_TRIGGERING_ACTOR": "another-user",
                "GITHUB_RUN_ATTEMPT": "2", "GITHUB_EVENT_NAME": "push", "GITHUB_REPOSITORY_ID": "1",
                "GITHUB_REPOSITORY_OWNER_ID": "1", "GITHUB_REF": "refs/heads/dev", "GITHUB_SHA": "bad",
                "GITHUB_WORKFLOW_REF": "other/workflow.yml", "GITHUB_JOB": "legacy_" + role,
                "TF_WORKSPACE": "other", "STATE_AWS_REGION": "other", "AWS_MAX_ATTEMPTS": "2",
                "AWS_RETRY_MODE": "adaptive", "MIGRATION_MODE": "apply", "MIGRATION_TARGET": "unknown",
            }.items():
                with self.subTest(role=role, field=key), self.assertRaises(ValueError):
                    migration.context(environment(role=role) | {key: value}, role)
        with self.assertRaises(ValueError):
            migration.context(environment(role="apply", mode="migration-plan"), "apply")

    def test_production_requires_independent_measured_readiness_gate(self):
        for role in ("plan", "apply"):
            env = environment(target="prod", role=role)
            for flag in (None, "false", "1", "TRUE"):
                with self.subTest(role=role, flag=flag), self.assertRaises(ValueError):
                    migration.context(env | {"MIGRATION_PRODUCTION_READY": flag}, role)
            self.assertEqual(migration.context(env | {"MIGRATION_PRODUCTION_READY": "true"}, role), "prod")

    def test_stage_rejects_bad_private_baseline_before_git_or_credentials(self):
        env = environment() | {"RUNNER_TEMP": str(self.directory), "MIGRATION_INPUTS_JSON": "invalid-private-" + CANARY}
        with patch.object(migration, "run") as command, patch.object(migration, "verify_identity") as identity:
            with self.assertRaises(ValueError):
                migration.stage(env)
            command.assert_not_called()
            identity.assert_not_called()
        self.assertFalse((self.directory / migration.PRIVATE).exists())

    def test_stage_validates_reviewed_source_and_creates_private_directory_without_auth(self):
        env = environment() | {"RUNNER_TEMP": str(self.directory)}
        responses = [subprocess.CompletedProcess([], 0, SHA.encode(), b""),
                     subprocess.CompletedProcess([], 0, b"", b"")]
        with patch.object(migration, "run", side_effect=responses) as command, patch.object(migration, "verify_identity") as identity:
            migration.stage(env)
        self.assertEqual(command.call_count, 2)
        identity.assert_not_called()
        root = self.directory / migration.PRIVATE
        self.assertTrue(root.is_dir())
        self.assertEqual(root.stat().st_mode & 0o777, 0o700)

    def test_stage_rejects_api_without_owned_consumer_before_git_or_credentials(self):
        for target in ("dev", "prod"):
            env = environment(target=target) | {"RUNNER_TEMP": str(self.directory),
                                                 "MIGRATION_PRODUCTION_READY": "true"}
            value = json.loads(env["MIGRATION_INPUTS_JSON"])
            service = "storage.googleapis.com"
            value["tfvars"].update(owned_api_keys=[service], api_classification={service: {
                "classification": "Own", "state": "ENABLED", "reason": CANARY,
                "dependency_addresses": ["module.backup.google_storage_bucket.retained[0]"],
            }})
            value["manifest"]["resources"] = [{
                "address": 'module.owned_project_apis.google_project_service.owned["storage.googleapis.com"]',
                "type": "google_project_service", "import_id": migration.PROJECTS[target] + "/" + service,
            }]
            env["MIGRATION_INPUTS_JSON"] = json.dumps(value)
            with self.subTest(target=target), patch.object(migration, "run") as command, patch.object(migration, "verify_identity") as identity:
                with self.assertRaises(ValueError) as rejected:
                    migration.stage(env)
                command.assert_not_called()
                identity.assert_not_called()
                self.assertNotIn(CANARY, str(rejected.exception))
            self.assertFalse((self.directory / migration.PRIVATE).exists())

    def test_private_inputs_reject_crossproject_duplicate_keys_and_unreviewed_shape(self):
        env = environment()
        self.assertEqual(migration.inputs(env)["manifest"], manifest())
        for edit in (
            lambda value: value["tfvars"].update(project_id=migration.PROJECTS["prod"]),
            lambda value: value["manifest"].update(project_id=migration.PROJECTS["prod"]),
            lambda value: value.update(unreviewed={}),
            lambda value: value.update(plan_integrity_key="short"),
        ):
            value = json.loads(env["MIGRATION_INPUTS_JSON"])
            edit(value)
            with self.subTest(edit=edit), self.assertRaises(ValueError):
                migration.inputs(env | {"MIGRATION_INPUTS_JSON": json.dumps(value)})
        for raw in ('{"duplicate":1,"duplicate":2}', '{"value":NaN}', "x" * (migration.MAX_BYTES + 1)):
            with self.assertRaises(ValueError):
                migration.inputs(env | {"MIGRATION_INPUTS_JSON": raw})

    def test_oidc_owner_environment_workflows_and_same_sha_are_exact(self):
        env = environment() | {"ACTIONS_ID_TOKEN_REQUEST_URL": "https://fixture.actions.invalid/token?request=fixture",
                               "ACTIONS_ID_TOKEN_REQUEST_TOKEN": CANARY}
        claims = {
            "repository_id": "340900071", "repository_owner_id": "54093651", "environment": "terraform-dev-plan",
            "ref": "refs/heads/feature/gcp-terraform-iac",
            "workflow_ref": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac",
            "event_name": "workflow_dispatch",
        }
        claims["sub"] = ":".join(f'{key}:{value.replace(":", "%3A")}' for key, value in claims.items())
        claims.update(actor_id="54093651", actor="kani3camp", run_attempt="1", workflow_sha=SHA, job_workflow_sha=SHA)

        def response(value):
            token = "header." + base64.urlsafe_b64encode(migration.canonical(value)).decode().rstrip("=") + ".signature"
            return io.BytesIO(json.dumps({"value": token}).encode())

        with patch.object(migration.urllib.request, "urlopen", return_value=response(claims)) as request:
            migration.verify_oidc(env)
        self.assertEqual(request.call_count, 1)
        self.assertEqual(request.call_args.args[0].full_url, env["ACTIONS_ID_TOKEN_REQUEST_URL"] + "&audience=sts.amazonaws.com")
        for key, value in {"actor_id": "1", "actor": "other", "run_attempt": "2", "sub": "different",
                           "environment": "terraform-dev-apply", "workflow_sha": "b" * 40,
                           "job_workflow_sha": "b" * 40, "repository_id": "1"}.items():
            with self.subTest(field=key), patch.object(migration.urllib.request, "urlopen", return_value=response(claims | {key: value})) as request, self.assertRaises(ValueError):
                migration.verify_oidc(env)
            self.assertEqual(request.call_count, 1)

    def test_process_failure_and_oversized_output_are_bounded_without_retry(self):
        for result in (subprocess.CompletedProcess([], 1, CANARY.encode(), CANARY.encode()),
                       subprocess.CompletedProcess([], 0, b"x" * 129, b""),
                       subprocess.CompletedProcess([], 0, b"", b"x" * 9)):
            with self.subTest(code=result.returncode, size=len(result.stdout)), patch.object(migration, "MAX_BYTES", 8), patch.object(migration.subprocess, "run", return_value=result) as command, self.assertRaises(ValueError) as rejected:
                migration.run(["fixture"])
            self.assertEqual(command.call_count, 1)
            self.assertNotIn(CANARY, str(rejected.exception))
        with patch.object(migration.subprocess, "run", side_effect=subprocess.TimeoutExpired([CANARY], 1)) as command, self.assertRaises(subprocess.TimeoutExpired):
            migration.run(["fixture"])
        self.assertEqual(command.call_count, 1)

    def test_identity_requires_actual_project_number_to_equal_private_manifest(self):
        role_id = "AROA" + "A" * 17
        for target in ("dev", "prod"):
            for role in ("plan", "apply"):
                project = migration.PROJECTS[target]
                env = environment(target=target, role=role) | {
                    "MIGRATION_PRODUCTION_READY": "true", "BACKEND_ROLE_ID": role_id,
                    "BACKEND_ROLE_ARN": f"arn:aws:iam::{PROJECT_NUMBER}:role/fixture-{role}",
                    "GCP_SERVICE_ACCOUNT": f"terraform-{target}-{role}@{project}.iam.gserviceaccount.com",
                }
                value = json.loads(env["MIGRATION_INPUTS_JSON"])
                value["manifest"]["project_number"] = PROJECT_NUMBER
                env["MIGRATION_INPUTS_JSON"] = json.dumps(value)
                actual_aws = {"Account": PROJECT_NUMBER, "UserId": role_id + ":fixture-session",
                              "Arn": f"arn:aws:sts::{PROJECT_NUMBER}:assumed-role/fixture-{role}/fixture-session"}
                actual_project = {"projectId": project, "state": "ACTIVE", "name": "projects/" + PROJECT_NUMBER}
                with self.subTest(target=target, role=role), patch.object(migration, "aws", return_value=actual_aws) as aws, patch.object(migration, "google", return_value=actual_project) as google:
                    migration.verify_identity(env)
                    aws.assert_called_once_with(env, "sts", "get-caller-identity")
                    google.assert_called_once_with(env, "cloudresourcemanager.googleapis.com", "v3/projects/" + project)
                for edit in ({"name": "projects/222222222222"}, {"name": project}, {"name": ""},
                             {"projectId": "foreign-project"}, {"state": "DELETE_REQUESTED"}):
                    with self.subTest(target=target, role=role, fields=list(edit)), patch.object(migration, "aws", return_value=actual_aws), patch.object(migration, "google", return_value=actual_project | edit), self.assertRaises(ValueError):
                        migration.verify_identity(env)
                value["manifest"].pop("project_number")
                with self.subTest(target=target, role=role, missing_number=True), patch.object(migration, "aws", return_value=actual_aws), patch.object(migration, "google", return_value=actual_project), self.assertRaises(ValueError):
                    migration.verify_identity(env | {"MIGRATION_INPUTS_JSON": json.dumps(value)})

    def test_import_only_permission_probe_checks_project_other_environment_and_scoped_runtime_authorities(self):
        env = environment()
        value = json.loads(env['MIGRATION_INPUTS_JSON'])
        value['manifest']['project_number'] = PROJECT_NUMBER
        env['MIGRATION_INPUTS_JSON'] = json.dumps(value)
        allowed = {'permissions': ['resourcemanager.projects.get']}
        with patch.object(migration, 'google', side_effect=[allowed, {}, {}, {}]) as google:
            migration.verify_permissions(env)
        self.assertEqual(google.call_count, 4)
        own, other, sa, pool = google.call_args_list
        self.assertIn('/' + migration.PROJECTS['dev'] + ':testIamPermissions', own.args[2])
        self.assertIn('/' + migration.PROJECTS['prod'] + ':testIamPermissions', other.args[2])
        self.assertTrue(sa.args[2].endswith('@appspot.gserviceaccount.com:testIamPermissions'))
        self.assertTrue(pool.args[2].endswith('/aws-runtime:testIamPermissions'))
        permissions = own.args[3]['permissions']
        self.assertLessEqual(len(permissions), 100)
        for forbidden in ('monitoring.alertPolicies.create', 'serviceusage.services.enable',
                          'cloudfunctions.functions.call', 'pubsub.topics.publish', 'cloudscheduler.jobs.run',
                          'cloudscheduler.jobs.enable'):
            self.assertIn(forbidden, permissions)
        self.assertNotIn('cloudscheduler.jobs.resume', permissions)
        with patch.object(migration, 'google', return_value={'permissions': ['resourcemanager.projects.get', 'cloudscheduler.jobs.enable']}), self.assertRaises(ValueError):
            migration.verify_permissions(env)
        for index in range(4):
            responses = [allowed, {}, {}, {}]
            responses[index] = {'permissions': ['monitoring.alertPolicies.create']}
            with self.subTest(boundary=index), patch.object(migration, 'google', side_effect=responses), self.assertRaises(ValueError):
                migration.verify_permissions(env)

    def test_other_environment_only_explicit_permission_denial_is_accepted(self):
        env = environment()
        value = json.loads(env['MIGRATION_INPUTS_JSON'])
        value['manifest']['project_number'] = PROJECT_NUMBER
        env['MIGRATION_INPUTS_JSON'] = json.dumps(value)
        allowed = {'permissions': ['resourcemanager.projects.get']}
        for status, body, accepted in ((403, {'error': {'status': 'PERMISSION_DENIED'}}, True),
                                       (404, {'error': {'status': 'NOT_FOUND'}}, False),
                                       (500, {'error': {'status': 'INTERNAL'}}, False),
                                       (403, {'error': {'status': CANARY}}, False)):
            error = urllib.error.HTTPError('https://fixture.invalid/' + CANARY, status, CANARY, {},
                                           io.BytesIO(json.dumps(body).encode()))
            self.addCleanup(error.close)
            with self.subTest(status=status, accepted=accepted), patch.object(migration, 'google', side_effect=[allowed, error, {}, {}]):
                if accepted:
                    migration.verify_permissions(env)
                else:
                    with self.assertRaises(ValueError):
                        migration.verify_permissions(env)


class MetadataRouteTest(OfflineTest):
    def test_supported_resource_types_use_only_fixed_hosts_and_exact_metadata_paths(self):
        project = migration.PROJECTS["dev"]
        cases = (
            ("google_storage_bucket", "fixture-bucket", "storage.googleapis.com", "storage/v1/b/fixture-bucket"),
            ("google_bigquery_dataset", f"projects/{project}/datasets/fixture", "bigquery.googleapis.com", f"bigquery/v2/projects/{project}/datasets/fixture"),
            ("google_bigquery_table", f"projects/{project}/datasets/fixture/tables/history", "bigquery.googleapis.com", f"bigquery/v2/projects/{project}/datasets/fixture/tables/history"),
            ("google_project_service", f"{project}/firestore.googleapis.com", "serviceusage.googleapis.com", f"v1/projects/{PROJECT_NUMBER}/services/firestore.googleapis.com"),
            ("google_firestore_backup_schedule", f"projects/{project}/databases/(default)/backupSchedules/12345678-1234-1234-1234-123456789abc", "firestore.googleapis.com", None),
            ("google_monitoring_notification_channel", f"projects/{project}/notificationChannels/123", "monitoring.googleapis.com", None),
            ("google_monitoring_alert_policy", f"projects/{project}/alertPolicies/123", "monitoring.googleapis.com", None),
            ("google_pubsub_topic", f"projects/{project}/topics/fixture", "pubsub.googleapis.com", None),
            ("google_cloud_scheduler_job", f"projects/{project}/locations/asia-southeast2/jobs/fixture", "cloudscheduler.googleapis.com", None),
            ("google_cloudfunctions_function", f"projects/{project}/locations/asia-southeast2/functions/fixture", "cloudfunctions.googleapis.com", None),
            ("google_iam_workload_identity_pool", f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime", "iam.googleapis.com", None),
            ("google_iam_workload_identity_pool_provider", f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider", "iam.googleapis.com", None),
        )
        for kind, identifier, host, path in cases:
            if path is None:
                path = ("v3/" if kind.startswith("google_monitoring_") else "v1/") + identifier
                if kind.startswith("google_iam_workload_identity_pool"):
                    path = path.replace("projects/" + project + "/", "projects/" + PROJECT_NUMBER + "/")
            with self.subTest(kind=kind):
                self.assertEqual(migration.metadata_route({"type": kind, "import_id": identifier}, project, PROJECT_NUMBER), (host, path, None))
        account = f"projects/{project}/serviceAccounts/{project}@appspot.gserviceaccount.com"
        self.assertEqual(migration.metadata_route({"type": "google_service_account_iam_member", "import_id": account + " roles/iam.workloadIdentityUser fixture-member"}, project, PROJECT_NUMBER),
                         ("iam.googleapis.com", "v1/" + account + ":getIamPolicy", {"options": {"requestedPolicyVersion": 3}}))

    def test_crossproject_injection_and_wrong_resource_path_are_rejected_before_http(self):
        project = migration.PROJECTS["dev"]
        topic = f"projects/{project}/topics/fixture"
        invalid = [topic.replace(project, migration.PROJECTS["prod"]), "https://attacker.invalid/path",
                   f"projects/{project}/notificationChannels/123", f"projects/{project}//topics/fixture",
                   topic + "/extra", topic + "\nheader", topic + "\tvalue", topic + " space"]
        invalid += [topic + suffix for suffix in ("?x=1", "#fragment", "%2f", "/../other", ":getIamPolicy")]
        for identifier in invalid:
            with self.subTest(identifier=identifier), self.assertRaises(ValueError):
                migration.metadata_route({"type": "google_pubsub_topic", "import_id": identifier}, project, PROJECT_NUMBER)
        for kind, identifier in (
            ("google_bigquery_dataset", f"projects/{project}/datasets/fixture/tables/history"),
            ("google_bigquery_table", f"projects/{project}/datasets/fixture"),
            ("google_project_service", migration.PROJECTS["prod"] + "/firestore.googleapis.com"),
            ("google_storage_bucket", "fixture-bucket/path"),
            ("google_project_iam_policy", topic),
        ):
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                migration.metadata_route({"type": kind, "import_id": identifier}, project, PROJECT_NUMBER)

    def test_wif_and_service_routes_require_explicit_numeric_project(self):
        project = migration.PROJECTS["dev"]
        descriptors = (
            {"type": "google_project_service", "import_id": project + "/firestore.googleapis.com"},
            {"type": "google_iam_workload_identity_pool", "import_id": f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime"},
            {"type": "google_iam_workload_identity_pool_provider", "import_id": f"projects/{project}/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider"},
        )
        for descriptor in descriptors:
            for number in ("", "invalid", "12345", PROJECT_NUMBER + "/other"):
                with self.subTest(kind=descriptor["type"], number=number), self.assertRaises(ValueError):
                    migration.metadata_route(descriptor, project, number)


class SnapshotTest(OfflineTest):
    def setup_snapshot(self, *, target="dev", absent=False):
        env = environment(target=target)
        key = f"youtube-study-space/{target}/terraform.tfstate"
        state = {"version": 4, "terraform_version": migration.VERSION, "serial": 2,
                 "lineage": "12345678-1234-1234-1234-123456789abc", "resources": [], "outputs": {}}
        raw = migration.canonical(state)
        head = {"ContentLength": len(raw), "VersionId": "fixture-version", "ETag": '"fixture-etag"', "ServerSideEncryption": "AES256"}
        settings = {"head": head, "get": deepcopy(head), "after_head": deepcopy(head), "raw": raw,
                    "listing_edit": None, "get_error": None}
        heads = [0]

        def aws(_env, service, action, *args):
            self.assertEqual(service, "s3api")
            if action == "list-objects-v2":
                prefix = args[args.index("--prefix") + 1]
                listing = {"Name": env["STATE_BUCKET"], "Prefix": prefix, "MaxKeys": 2,
                           "KeyCount": 0, "IsTruncated": False}
                if prefix == key and not absent:
                    listing.update(KeyCount=1, Contents=[{"Key": key, "ETag": head["ETag"], "Size": len(raw)}])
                if settings["listing_edit"]:
                    settings["listing_edit"](prefix, listing)
                return listing
            if action == "head-object":
                heads[0] += 1
                return deepcopy(settings["head"] if heads[0] == 1 else settings["after_head"])
            if action == "get-object":
                if settings["get_error"]:
                    raise settings["get_error"]
                Path(args[-1]).write_bytes(settings["raw"])
                return deepcopy(settings["get"])
            raise AssertionError("Unexpected offline AWS operation")

        return env, settings, Mock(side_effect=aws)

    def test_current_object_get_is_bounded_and_version_etag_head_get_head_must_match(self):
        env, settings, aws = self.setup_snapshot()
        with patch.object(migration, "aws", aws):
            result = migration.snapshot(env, self.directory, "before")
        self.assertEqual(result["head"], settings["head"])
        self.assertEqual(result["raw"], settings["raw"].decode())
        actions = [call.args[2] for call in aws.call_args_list]
        self.assertEqual(actions, ["list-objects-v2"] * 3 + ["head-object", "get-object", "head-object"])
        arguments = aws.call_args_list[4].args
        self.assertNotIn("--version-id", arguments)
        self.assertEqual(arguments[arguments.index("--range") + 1], f"bytes=0-{migration.MAX_BYTES - 1}")
        self.assertEqual((self.directory / "before.tfstate").stat().st_mode & 0o777, 0o600)

    def test_only_positive_production_absence_can_initialize(self):
        for target in ("dev", "prod"):
            env, _, aws = self.setup_snapshot(target=target, absent=True)
            with self.subTest(target=target), patch.object(migration, "aws", aws):
                if target == "prod":
                    self.assertEqual(migration.snapshot(env, self.directory, "empty"), {"head": None, "raw": None, "state": None})
                else:
                    with self.assertRaises(ValueError):
                        migration.snapshot(env, self.directory, "empty")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))

    def test_forbidden_and_unknown_aws_failures_never_become_absence(self):
        for error in (urllib.error.HTTPError("https://fixture.invalid", 403, CANARY, {}, None),
                      subprocess.TimeoutExpired([CANARY], 1, output=CANARY), ValueError(CANARY)):
            if isinstance(error, urllib.error.HTTPError):
                self.addCleanup(error.close)
            aws = Mock(side_effect=error)
            with self.subTest(error=type(error)), patch.object(migration, "aws", aws), self.assertRaises(type(error)):
                migration.snapshot(environment(target="prod"), self.directory, "absent")
            self.assertEqual(aws.call_count, 1)

    def test_lock_workspace_or_malformed_absence_listing_stops_before_download(self):
        edits = (
            lambda prefix, value: value.update(KeyCount=1) if prefix.endswith(".tflock") else None,
            lambda prefix, value: value.update(KeyCount=1) if prefix.endswith("workspaces/") else None,
            lambda prefix, value: value.update(IsTruncated=True),
            lambda prefix, value: value.update(Contents=[{"Key": "unexpected"}]),
            lambda prefix, value: value.pop("IsTruncated"),
            lambda prefix, value: value.update(KeyCount=False),
        )
        for number, edit in enumerate(edits):
            env, settings, aws = self.setup_snapshot(target="prod", absent=True)
            settings["listing_edit"] = edit
            with self.subTest(case=number), patch.object(migration, "aws", aws), self.assertRaises(ValueError):
                migration.snapshot(env, self.directory, "invalid")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))

    def test_size_encryption_version_and_races_are_rejected(self):
        for field, bad in (("ContentLength", migration.MAX_BYTES + 1), ("ContentLength", 0),
                           ("ContentLength", True), ("ServerSideEncryption", "aws:kms"), ("VersionId", "null")):
            env, settings, aws = self.setup_snapshot()
            settings["head"][field] = bad
            with self.subTest(field=field, bad=bad), patch.object(migration, "aws", aws), self.assertRaises(ValueError):
                migration.snapshot(env, self.directory, "invalid")
            self.assertFalse(any(call.args[2] == "get-object" for call in aws.call_args_list))
        for name, edit in (
            ("version", lambda settings: settings["get"].update(VersionId="changed")),
            ("etag", lambda settings: settings["get"].update(ETag="changed")),
            ("length", lambda settings: settings["get"].update(ContentLength=1)),
            ("body", lambda settings: settings.update(raw=b"{}")),
            ("head-race", lambda settings: settings["after_head"].update(VersionId="changed")),
        ):
            env, settings, aws = self.setup_snapshot()
            edit(settings)
            with self.subTest(case=name), patch.object(migration, "aws", aws), self.assertRaises(ValueError):
                migration.snapshot(env, self.directory, name)


class ExecuteTest(OfflineTest):
    def setup_execution(self, *, role="apply", apply_error=None, commitment=None):
        root = self.directory / role
        root.mkdir(mode=0o700)
        env = environment(role=role) | {"RUNNER_TEMP": str(self.directory), "GITHUB_OUTPUT": str(self.directory / "output")}
        summary = {"managed": 0, "imports": 0, "raw_observation_drift": 0}
        before = {"head": {"VersionId": "fixed-version"}, "raw": "private-state-" + CANARY,
                  "state": {"fixture": CANARY}}
        source_name = "infra/gcp/environments/dev/main.tf"
        source_copy = root / "checkout" / source_name
        source_copy.parent.mkdir(parents=True)
        source_copy.write_bytes(b"fixture-reviewed-source")
        closure = {source_name: migration.hashlib.sha256(source_copy.read_bytes()).hexdigest()}
        calls = []

        def terraform(_env, directory, _tfroot, name, args, **kwargs):
            calls.append((name, args))
            if name in {"plan", "post-plan"}:
                Path(next(arg.split("=", 1)[1] for arg in args if arg.startswith("-out="))).write_bytes(b"private-plan-" + CANARY.encode())
            if name == "apply" and apply_error:
                raise apply_error
            return subprocess.CompletedProcess(args, 0, migration.canonical({"fixture": CANARY}), b"")

        self.stack.enter_context(patch.object(migration, "verify_identity"))
        self.stack.enter_context(patch.object(migration, "verify_permissions"))
        self.stack.enter_context(patch.object(migration, "private_root", return_value=root))
        self.stack.enter_context(patch.object(migration, "collect_metadata", return_value={"fixture": CANARY}))
        self.stack.enter_context(patch.object(migration, "snapshot", return_value=before))
        self.stack.enter_context(patch.object(migration, "copy_source", return_value=(root, closure)))
        self.stack.enter_context(patch.object(migration, "terraform", side_effect=terraform))
        self.stack.enter_context(patch.object(migration.contract, "validate_plan", return_value=summary))
        self.stack.enter_context(patch.object(migration.contract, "validate_state_delta"))
        self.stack.enter_context(patch.object(migration.contract, "plan_commitment_payload", return_value={"fixture": CANARY}))
        self.stack.enter_context(patch.object(migration, "run", return_value=subprocess.CompletedProcess([], 0, SHA.encode(), b"")))
        payload = {"source": SHA, "closure": closure, "inputs": migration.inputs(env), "state": before,
                   "metadata": {"fixture": CANARY}, "plan": {"fixture": CANARY}}
        self.fixture_payload = payload
        expected = migration.hmac.new(bytes.fromhex("b" * 64), b"gcp-migration-execution-v1\0" + migration.canonical(payload), migration.hashlib.sha256).hexdigest()
        env["MIGRATION_EXPECTED_COMMITMENT"] = expected if commitment is None else commitment
        _, components = migration.execution_commitment(migration.inputs(env), payload)
        env['MIGRATION_EXPECTED_COMPONENTS'] = json.dumps(components)
        return env, root, calls

    def main(self, env):
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, env, clear=True), patch.object(sys, "argv", ["controller", "execute"]), redirect_stdout(stdout), redirect_stderr(stderr):
            result = migration.main()
        self.assertNotIn(CANARY, stdout.getvalue() + stderr.getvalue())
        self.assertEqual(stderr.getvalue(), "")
        return result, stdout.getvalue()

    def test_successful_plan_never_applies_and_publishes_only_keyed_commitments(self):
        env, _, calls = self.setup_execution(role="plan")
        result, output = self.main(env)
        self.assertEqual(result, 0)
        self.assertIn("GCP_MIGRATION: PASS; role=plan", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        public = Path(env["GITHUB_OUTPUT"]).read_text()
        rows = dict(line.split('=', 1) for line in public.splitlines())
        self.assertEqual(set(rows), {'commitment', 'components'})
        self.assertEqual(rows['commitment'], env["MIGRATION_EXPECTED_COMMITMENT"])
        self.assertEqual(json.loads(rows['components']), json.loads(env['MIGRATION_EXPECTED_COMPONENTS']))
        self.assertNotIn(CANARY, public)

    def test_successful_apply_uses_saved_plan_once_and_checks_post_plan(self):
        env, root, calls = self.setup_execution()
        result, output = self.main(env)
        self.assertEqual(result, 0)
        self.assertIn("GCP_MIGRATION: PASS; role=apply", output)
        self.assertEqual([name for name, _ in calls], ["init", "schema", "plan", "show", "apply", "post-plan", "post-show"])
        self.assertEqual((root / "apply-attempt").read_bytes(), b"1\n")
        self.assertEqual(next(args for name, args in calls if name == "apply")[-1], str(root / "saved.tfplan"))
        backend = (root / "backend.hcl").read_text()
        self.assertIn('allowed_account_ids = ["111111111111"]\n', backend)
        self.assertNotIn("expected_bucket_owner", backend)

    def test_unknown_apply_failure_has_no_retry_or_raw_exception_leak(self):
        error = subprocess.TimeoutExpired([CANARY], 1, output=CANARY.encode(), stderr=CANARY.encode())
        env, root, calls = self.setup_execution(apply_error=error)
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=APPLY; reason=EXECUTION", output)
        self.assertEqual(sum(name == "apply" for name, _ in calls), 1)
        self.assertFalse(any(name == "post-plan" for name, _ in calls))
        self.assertEqual((root / "apply-attempt").read_bytes(), b"1\n")
        result, _ = self.main(env)
        self.assertEqual(result, 1)
        self.assertEqual(sum(name == "apply" for name, _ in calls), 1)

    def test_rejected_plan_is_not_applied_and_failure_readback_proves_unchanged_boundaries(self):
        env, root, calls = self.setup_execution()
        migration.contract.validate_plan.side_effect = migration.contract.ContractError('ACTION')
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn('stage=VALIDATE; reason=ACTION', output)
        self.assertIn('"apply_attempted":false,"metadata":"SAME","state":"SAME"', output)
        self.assertEqual([call.args[2] for call in migration.snapshot.call_args_list], ['before', 'failure'])
        self.assertFalse(any(name == 'apply' for name, _ in calls))
        self.assertFalse((root / 'apply-attempt').exists())

    def test_failure_readback_reports_changed_and_unobserved_without_masking_original_failure(self):
        env, _, calls = self.setup_execution()
        before = migration.snapshot.return_value
        migration.snapshot.side_effect = [before, {'private': CANARY}]
        migration.collect_metadata.side_effect = [{'fixture': CANARY}, ValueError(CANARY)]
        migration.contract.validate_plan.side_effect = migration.contract.ContractError('VALUE')
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn('stage=VALIDATE; reason=VALUE', output)
        self.assertIn('"metadata":"UNOBSERVED","state":"CHANGED"', output)
        self.assertEqual(migration.snapshot.call_count, 2)
        self.assertEqual(migration.collect_metadata.call_count, 2)
        self.assertFalse(any(name == 'apply' for name, _ in calls))

    def test_failure_before_state_snapshot_never_claims_state_safety(self):
        env, _, _ = self.setup_execution()
        migration.verify_identity.side_effect = ValueError(CANARY)
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn('stage=IDENTITY; reason=EXECUTION', output)
        self.assertIn('"metadata":"UNOBSERVED","state":"UNOBSERVED"', output)
        migration.snapshot.assert_not_called()
        migration.collect_metadata.assert_not_called()

    def test_different_commitment_prevents_any_apply(self):
        env, root, calls = self.setup_execution(commitment="0" * 64)
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=REPLAN_MATCH", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        self.assertFalse((root / "apply-attempt").exists())

    def test_old_unprefixed_hmac_does_not_authorize_apply(self):
        env, root, calls = self.setup_execution()
        env["MIGRATION_EXPECTED_COMMITMENT"] = migration.hmac.new(
            bytes.fromhex("b" * 64), migration.canonical(self.fixture_payload), migration.hashlib.sha256).hexdigest()
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=REPLAN_MATCH", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        self.assertFalse((root / "apply-attempt").exists())

    def test_section_mismatch_is_finite_and_stops_before_apply_even_when_overall_value_was_passed(self):
        env, root, calls = self.setup_execution()
        components = json.loads(env['MIGRATION_EXPECTED_COMPONENTS'])
        components['metadata'] = '0' * 64
        env['MIGRATION_EXPECTED_COMPONENTS'] = json.dumps(components)
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn('GCP_MIGRATION_COMPARISON: {"changed_components":["metadata"],"changed_details":[],"key_match":true}', output)
        self.assertFalse(any(name == 'apply' for name, _ in calls))
        self.assertFalse((root / 'apply-attempt').exists())

    def test_each_private_section_and_key_is_independently_bound_without_unkeyed_hashes(self):
        env, _, _ = self.setup_execution()
        original = self.fixture_payload
        value = migration.inputs(env)
        overall, components = migration.execution_commitment(value, original)
        for name in original:
            changed = deepcopy(original)
            changed[name] = {'private': CANARY} if name != 'inputs' else deepcopy(original['inputs']) | {'private': CANARY}
            next_overall, next_components = migration.execution_commitment(value, changed)
            result = migration.compare_commitments(json.dumps(components), next_components)
            self.assertNotEqual(overall, next_overall)
            self.assertEqual(result['changed_components'], [name])
            self.assertEqual(result['key_match'], name != 'source')
            self.assertNotIn(CANARY, json.dumps(result) + json.dumps(next_components))
        _, changed_key = migration.execution_commitment(value | {'plan_integrity_key': 'c' * 64}, original)
        self.assertFalse(migration.compare_commitments(json.dumps(components), changed_key)['key_match'])
        for invalid in ('', '{}', json.dumps(components | {CANARY: 'd' * 64}),
                        json.dumps(components | {'state': CANARY})):
            with self.subTest(invalid_length=len(invalid)), self.assertRaises(ValueError):
                migration.compare_commitments(invalid, components)
        with self.assertRaises(ValueError):
            migration.compare_commitments(json.dumps(components | {CANARY: 'd' * 64}), components | {CANARY: 'd' * 64})

    def test_subsection_diagnostic_names_are_fixed_and_each_detail_still_requires_equality(self):
        env, root, calls = self.setup_execution()
        components = json.loads(env['MIGRATION_EXPECTED_COMPONENTS'])
        components['plan.configuration'] = '0' * 64
        env['MIGRATION_EXPECTED_COMPONENTS'] = json.dumps(components)
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn('"changed_components":[],"changed_details":["plan.configuration"]', output)
        self.assertFalse(any(name == 'apply' for name, _ in calls))
        self.assertFalse((root / 'apply-attempt').exists())

    def test_changed_source_copy_stops_before_apply_is_reserved(self):
        env, root, calls = self.setup_execution()
        before = migration.snapshot.return_value
        source_name = next(iter(self.fixture_payload["closure"]))

        def snapshot(_env, _root, name):
            if name == "pre-apply":
                (root / "checkout" / source_name).write_bytes(b"changed-unreviewed-source-" + CANARY.encode())
            return before

        migration.snapshot.side_effect = snapshot
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=SEAL; reason=EXECUTION", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        self.assertFalse((root / "apply-attempt").exists())

    def test_changed_metadata_after_plan_prevents_any_apply(self):
        env, root, calls = self.setup_execution()
        migration.collect_metadata.side_effect = [{"fixture": CANARY}, {"fixture": "changed"}]
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=FRESH_RECHECK", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        self.assertFalse((root / "apply-attempt").exists())

    def test_multi_import_wave_is_rejected_before_any_apply(self):
        env, root, calls = self.setup_execution()
        migration.contract.validate_plan.return_value = {"managed": 2, "imports": 2, "raw_observation_drift": 0}
        result, output = self.main(env)
        self.assertEqual(result, 1)
        self.assertIn("stage=VALIDATE", output)
        self.assertFalse(any(name == "apply" for name, _ in calls))
        self.assertFalse((root / "apply-attempt").exists())

    def test_raw_terraform_output_is_private_and_unknown_failure_text_is_suppressed(self):
        root = self.directory / "raw"
        root.mkdir(mode=0o700)
        result = subprocess.CompletedProcess([], 0, CANARY.encode(), CANARY.encode())
        stream = io.StringIO()
        with patch.object(migration, "run", return_value=result) as command, redirect_stdout(stream):
            migration.terraform(environment(), root, root, "fixture", ["plan"])
        self.assertEqual(stream.getvalue(), "")
        self.assertEqual(command.call_count, 1)
        for path in (root / "fixture.stdout", root / "fixture.stderr"):
            self.assertEqual(path.read_bytes(), CANARY.encode())
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)


def evaluate_condition(expression, mode):
    """Evaluate the small routing expression grammar without executing workflow code."""
    expression = expression.removeprefix("${{").removesuffix("}}").strip()
    expression = expression.replace("&&", " and ").replace("||", " or ")
    expression = re.sub(r"!(?!=)", " not ", expression).strip()

    def visit(node):
        if isinstance(node, ast.Constant) and type(node.value) in (str, bool):
            return node.value
        if isinstance(node, ast.Attribute) and isinstance(node.value, ast.Name) and node.value.id == "inputs" and node.attr == "mode":
            return mode
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, ast.Not):
            return not visit(node.operand)
        if isinstance(node, ast.BoolOp) and isinstance(node.op, (ast.And, ast.Or)):
            return all(map(visit, node.values)) if isinstance(node.op, ast.And) else any(map(visit, node.values))
        if isinstance(node, ast.Compare) and len(node.ops) == len(node.comparators) == 1 and isinstance(node.ops[0], (ast.Eq, ast.NotEq)):
            same = visit(node.left) == visit(node.comparators[0])
            return same if isinstance(node.ops[0], ast.Eq) else not same
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id == "startsWith" and len(node.args) == 2 and not node.keywords:
            return visit(node.args[0]).startswith(visit(node.args[1]))
        raise AssertionError("Unreviewed workflow routing expression")

    return visit(ast.parse(expression, mode="eval").body)


class WorkflowRoutingTest(OfflineTest):
    def test_new_and_legacy_execution_jobs_are_mutually_exclusive(self):
        source = (migration.ROOT / ".github/workflows/gcp-terraform-authenticated.yml").read_text()
        jobs = {}
        for job in ("plan", "apply", "migration_plan", "migration_apply"):
            block = re.search(r"^  " + job + r":\n(.*?)(?=^  [A-Za-z_]+:|\Z)", source, re.MULTILINE | re.DOTALL).group(1)
            jobs[job] = re.search(r"^    if: (.+)$", block, re.MULTILINE).group(1)
        for mode in ("migration-plan", "migration-apply", "plan", "apply", "email-adoption", "quota-plan", "quota-create", "quota-refresh", "current-state-read", "runtime-metadata-read", "security-probe", "dev-history-observation-sync"):
            admitted = {job for job, expression in jobs.items() if evaluate_condition(expression, mode)}
            with self.subTest(mode=mode):
                if mode == "migration-plan":
                    self.assertEqual(admitted, {"migration_plan"})
                elif mode == "migration-apply":
                    self.assertEqual(admitted, {"migration_plan", "migration_apply"})
                else:
                    self.assertFalse(admitted & {"migration_plan", "migration_apply"})
        for job in ("migration_plan", "migration_apply"):
            block = re.search(r"^  " + job + r":\n(.*?)(?=^  [A-Za-z_]+:|\Z)", source, re.MULTILINE | re.DOTALL).group(1)
            stage = block.index("terraform_migration.py stage")
            self.assertLess(stage, block.index("terraform_migration.py oidc"))
            self.assertLess(stage, block.index("uses: aws-actions/configure-aws-credentials@"))
            self.assertLess(stage, block.index("uses: google-github-actions/auth@"))


class PublicDiagnosticTest(OfflineTest):
    def test_failure_readback_closes_http_error_responses_without_publishing_their_reason_or_url(self):
        streams = [io.BytesIO(CANARY.encode()), io.BytesIO(CANARY.encode())]
        errors = [urllib.error.HTTPError('https://fixture.invalid/' + CANARY, 403, CANARY, {}, stream)
                  for stream in streams]
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.object(migration, 'snapshot', side_effect=errors[0]), patch.object(migration, 'collect_metadata', side_effect=errors[1]), redirect_stdout(stdout), redirect_stderr(stderr):
            result = migration.failure_safety(environment(), self.directory, manifest(), {}, {})
        self.assertEqual(result, {'apply_attempted': False, 'state': 'UNOBSERVED', 'metadata': 'UNOBSERVED'})
        self.assertTrue(all(stream.closed for stream in streams))
        self.assertEqual(stdout.getvalue() + stderr.getvalue(), '')

    def test_diagnostics_include_only_fixed_names_and_counts_even_for_private_keys_values_and_addresses(self):
        plan = {'resource_changes': [{
            'address': CANARY, 'type': CANARY,
            'change': {'actions': ['update', CANARY], CANARY: CANARY,
                       'before_identity': {'id': CANARY}, 'after_identity': {'id': CANARY},
                       'before': {'schema': CANARY, CANARY: CANARY},
                       'after': {'schema': 'changed', CANARY: 'changed'}}}]}
        diagnostic = migration.plan_diagnostic(plan)
        self.assertNotIn(CANARY, json.dumps(diagnostic))
        self.assertEqual(diagnostic['actions']['update'], 1)
        self.assertEqual(diagnostic['actions']['other'], 1)
        self.assertEqual(diagnostic['changed_fields'], {'schema': 1})
        self.assertEqual(diagnostic['other_changed_fields'], 1)
        self.assertEqual(diagnostic['identities_present'], 1)
        self.assertEqual(diagnostic['identities_changed'], 0)
        self.assertEqual(diagnostic['unknown_change_keys'], len(set(plan['resource_changes'][0]['change']) - migration.contract.CHANGE_KEYS))
        for malformed in (None, [], {'resource_changes': CANARY}):
            self.assertEqual(migration.plan_diagnostic(malformed), {'shape_observed': False})


if __name__ == "__main__":
    unittest.main()
