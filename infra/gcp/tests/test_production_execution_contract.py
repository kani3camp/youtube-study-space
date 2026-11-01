#!/usr/bin/env python3
"""Production boundaries with synthetic complete private snapshots only."""
from __future__ import annotations

import contextlib
from copy import deepcopy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import production_execution_contract as gate
from test_production_readiness import credentialless_environment

SHA = "a" * 40
NOW = datetime(2026, 10, 14, 10, tzinfo=timezone.utc)


def topic(number):
    name = f"fixture-{number}"
    identifier = f"projects/{gate.PROJECT}/topics/{name}"
    return {"address": f'module.topics.google_pubsub_topic.topic["{name}"]', "type": "google_pubsub_topic",
            "schema_version": 0, "import_id": identifier,
            "values": {"id": identifier, "project": gate.PROJECT, "name": name, "labels": {}}}


def api():
    service = "fixture.googleapis.com"
    identifier = gate.PROJECT + "/" + service
    return {"address": f'module.apis.google_project_service.owned["{service}"]', "type": "google_project_service",
            "schema_version": 0, "import_id": identifier,
            "values": {"id": identifier, "project": gate.PROJECT, "service": service,
                       "disable_on_destroy": False, "disable_dependent_services": False}}


def state(resources, serial=3):
    containers = {}
    for descriptor in resources:
        match = gate.ADDRESS.fullmatch(descriptor["address"])
        module = match["modules"].removesuffix(".")
        prefix = (module + "." if module else "") + match["type"] + "." + match["name"]
        container = containers.setdefault(prefix, {**({"module": module} if module else {}), "mode": "managed", "type": match["type"],
            "name": match["name"], "provider": f'provider["{gate.GOOGLE}"]', "instances": []})
        instance = {"schema_version": descriptor["schema_version"], "attributes": deepcopy(descriptor["values"]),
                    "sensitive_attributes": [], "private": "fixture-private-state", "dependencies": []}
        if match["index"]:
            instance["index_key"] = json.loads(match["index"][1:-1])
        container["instances"].append(instance)
    return {"version": 4, "terraform_version": gate.TERRAFORM_VERSION, "serial": serial,
            "lineage": "12345678-1234-1234-1234-123456789abc",
            "outputs": {name: {"value": value, "type": "string"} for name, value in gate.OUTPUTS.items()},
            "resources": list(containers.values()), "check_results": None}


def listing(prefix, contents=()):
    return {"Name": "fixture-private-state-bucket", "Prefix": prefix, "MaxKeys": 2,
            "KeyCount": len(contents), "IsTruncated": False, "Contents": list(contents)}


def snapshot(value, version="fixture-before"):
    if value is None:
        return {"state_body": None, "head_before": None, "get_metadata": None, "head_after": None,
                "state_listing": listing(gate.STATE_KEY), "lock_listing": listing(gate.STATE_KEY + ".tflock"),
                "workspace_listing": listing(gate.WORKSPACE_PREFIX + "/")}
    body = gate.canonical(value)
    head = {"ContentLength": len(body.encode()), "VersionId": version,
            "ETag": '"' + hashlib.md5(body.encode()).hexdigest() + '"', "ServerSideEncryption": "AES256"}
    return {"state_body": body, "head_before": head, "get_metadata": deepcopy(head), "head_after": deepcopy(head),
            "state_listing": listing(gate.STATE_KEY, [{"Key": gate.STATE_KEY, "Size": head["ContentLength"], "ETag": head["ETag"]}]),
            "lock_listing": listing(gate.STATE_KEY + ".tflock"), "workspace_listing": listing(gate.WORKSPACE_PREFIX + "/")}


def plan(resources, *, importing=(), initialize=False):
    changes, planned = [], []
    for descriptor in resources:
        match = gate.ADDRESS.fullmatch(descriptor["address"])
        header = {"address": descriptor["address"], **({"module_address": match["modules"].removesuffix(".")} if match["modules"] else {}),
                  "mode": "managed", "type": descriptor["type"], "name": match["name"], "provider_name": gate.GOOGLE}
        if match["index"]:
            header["index"] = json.loads(match["index"][1:-1])
        change = {"actions": ["no-op"], "before": deepcopy(descriptor["values"]), "after": deepcopy(descriptor["values"]),
                  "after_unknown": {}, "before_sensitive": {}, "after_sensitive": {}}
        if descriptor["address"] in importing:
            change["importing"] = {"id": descriptor["import_id"]}
        changes.append(dict(header, change=change))
        planned.append(dict(header, schema_version=descriptor["schema_version"], values=deepcopy(descriptor["values"]), sensitive_values={}))
    return {"format_version": "1.2", "terraform_version": gate.TERRAFORM_VERSION, "complete": True, "errored": False,
            "resource_changes": changes, "resource_drift": [], "checks": [],
            "output_changes": {name: {"actions": ["create"] if initialize else ["no-op"],
                "before": None if initialize else value, "after": value, "after_unknown": False,
                "before_sensitive": False, "after_sensitive": False} for name, value in gate.OUTPUTS.items()},
            "planned_values": {"outputs": {name: {"value": value, "type": "string", "sensitive": False}
                for name, value in gate.OUTPUTS.items()}, "root_module": {"resources": planned}}}


def bundle(wave="import", adopted_count=2, same_container=False, now=NOW):
    adopted = [topic(number) for number in range(adopted_count)] if wave != "initialize" else []
    candidate = (topic(adopted_count) if same_container else api()) if wave == "import" else None
    resources = adopted + ([candidate] if candidate else [])
    metadata = {descriptor["address"]: {"project_id": gate.PROJECT, "resource_id": descriptor["values"]["id"],
        "payload": {"name": descriptor["values"]["id"], "opaque": {"marker": "private-sentinel"}}} for descriptor in resources}
    before = snapshot(None) if wave == "initialize" else snapshot(state(adopted))
    after = (deepcopy(before) if wave == "noop" else snapshot(state(resources, serial=8), "fixture-after"))
    return {"schema_version": "production-private-validation-bundle-v1", "contract": {
        "schema_version": "production-execution-contract-v1", "git_sha": SHA,
        "issued_utc": (now - timedelta(minutes=1)).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "expires_utc": (now + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "project_id": gate.PROJECT, "project_number": "111111111111", "state_bucket": "fixture-private-state-bucket",
        "state_key": gate.STATE_KEY, "wave": wave, "adopted": adopted, "candidate": candidate},
        "before": {"snapshot": before, "metadata": deepcopy(metadata)}, "after": {"snapshot": after, "metadata": deepcopy(metadata)},
        "plan_before": plan(resources, importing={candidate["address"]} if candidate else set(), initialize=wave == "initialize"),
        "plan_after": plan(resources)}


class ProductionExecutionContractTest(unittest.TestCase):
    def rejected(self, change, original=None):
        value = deepcopy(bundle() if original is None else original)
        change(value)
        with self.assertRaises((ValueError, TypeError, KeyError)):
            gate.validate(value, git_sha=SHA, now=NOW)

    def edit_state(self, value, phase, edit):
        prior = value[phase]["snapshot"]
        decoded = json.loads(prior["state_body"])
        edit(decoded)
        value[phase]["snapshot"] = snapshot(decoded, prior["head_before"]["VersionId"])

    def test_initialization_is_a_separate_output_only_wave_and_not_infrastructure_readiness(self):
        result = gate.validate(bundle("initialize"), git_sha=SHA, now=NOW)
        self.assertEqual((result["imports"], result["managed_before"], result["managed_after"]), (0, 0, 0))
        self.assertFalse(result["live_readiness_attested"])
        self.assertEqual(result["resource_change_actions"], 0)
        self.rejected(lambda value: value["contract"].update(candidate=api()), bundle("initialize"))
        self.rejected(lambda value: value["plan_before"]["output_changes"]["environment"].update(after="development"), bundle("initialize"))
        self.rejected(lambda value: value["plan_before"].update(resource_changes=plan([api()])["resource_changes"]), bundle("initialize"))

    def test_exact_import_preserves_arbitrary_adopted_graph_and_same_container_instances(self):
        for count in (0, 1, 5):
            for same_container in (False, True):
                with self.subTest(count=count, same_container=same_container):
                    result = gate.validate(bundle(adopted_count=count, same_container=same_container), git_sha=SHA, now=NOW)
                    self.assertEqual((result["imports"], result["managed_before"], result["managed_after"]), (1, count, count + 1))
        value = bundle(same_container=True)
        value["contract"]["adopted"].reverse()
        self.edit_state(value, "after", lambda state: state["resources"][0]["instances"].reverse())
        gate.validate(value, git_sha=SHA, now=NOW)

    def test_noop_requires_identical_state_object_metadata_and_graph(self):
        gate.validate(bundle("noop"), git_sha=SHA, now=NOW)
        self.rejected(lambda value: value["after"]["snapshot"]["head_before"].update(VersionId="different"), bundle("noop"))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state.update(serial=state["serial"] + 1)), bundle("noop"))
        self.rejected(lambda value: value["plan_before"]["resource_changes"][0]["change"].update(importing={"id": "unexpected"}), bundle("noop"))

    def test_unknown_graph_actions_outputs_and_deferred_execution_stop(self):
        for phase in ("plan_before", "plan_after"):
            for actions in (["create"], ["update"], ["delete"], ["delete", "create"], ["read"], ["forget"], ["unknown"]):
                with self.subTest(phase=phase, actions=actions):
                    self.rejected(lambda value: value[phase]["resource_changes"][0]["change"].update(actions=actions))
            for field in ("resource_drift", "deferred_changes", "action_invocations", "deferred_action_invocations"):
                with self.subTest(phase=phase, field=field):
                    self.rejected(lambda value: value[phase].update({field: [{}]}))
            for field in ("previous_address", "deposed"):
                self.rejected(lambda value: value[phase]["resource_changes"][0].update({field: "private-sentinel"}))
            self.rejected(lambda value: value[phase].update(complete=False))
            self.rejected(lambda value: value[phase].update(unknown_future_execution=[]))
            self.rejected(lambda value: value[phase]["resource_changes"][0]["change"].update(after_unknown={"nested": [True]}))
            self.rejected(lambda value: value[phase]["output_changes"]["project_id"].update(actions=["update"]))
            self.rejected(lambda value: value[phase]["output_changes"].update(extra={"actions": ["no-op"]}))
            self.rejected(lambda value: value[phase]["planned_values"]["root_module"]["resources"].pop())
            self.rejected(lambda value: value[phase]["resource_changes"].append(deepcopy(value[phase]["resource_changes"][0])))
        self.rejected(lambda value: value["plan_before"]["resource_changes"][-1]["change"].update(importing={"id": "wrong-private-id"}))
        self.rejected(lambda value: value["plan_after"]["resource_changes"][-1]["change"].update(importing={"id": value["contract"]["candidate"]["import_id"]}))

    def test_failed_missing_truncated_or_wrong_target_listing_never_proves_absence(self):
        for field in ("state_listing", "lock_listing", "workspace_listing"):
            for bad in ({"Error": {"Code": "AccessDenied"}}, {}, None):
                self.rejected(lambda value: value["before"]["snapshot"].update({field: bad}), bundle("initialize"))
            self.rejected(lambda value: value["before"]["snapshot"][field].update(IsTruncated=True), bundle("initialize"))
            self.rejected(lambda value: value["before"]["snapshot"][field].update(Prefix="youtube-study-space/dev/"), bundle("initialize"))
            self.rejected(lambda value: value["before"]["snapshot"][field].update(KeyCount=1, Contents=[{"Key": "unexpected-private-key"}]), bundle("initialize"))
        self.rejected(lambda value: value["before"]["snapshot"].update(state_body="private-sentinel"), bundle("initialize"))
        self.rejected(lambda value: value["before"]["snapshot"]["get_metadata"].update(VersionId="raced-version"))
        self.rejected(lambda value: value["before"]["snapshot"]["head_before"].update(ContentLength=gate.MAX_STATE_BYTES + 1))

    def test_state_lineage_serial_old_private_identity_and_container_changes_stop(self):
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state.update(lineage="87654321-1234-1234-1234-123456789abc")))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state.update(serial=3)))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state["resources"][0]["instances"][0].update(private="changed")))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state["resources"][0].update(provider='provider["other"]')))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state["resources"][0]["instances"].append(deepcopy(state["resources"][0]["instances"][0]))))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state.update(check_results=[{"status": "unknown"}])))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state.update(unknown_metadata={})))
        self.rejected(lambda value: self.edit_state(value, "after", lambda state: state["resources"][-1]["instances"][0].update(identity_schema_version=99, identity={})))
        self.rejected(lambda value: value["after"]["snapshot"]["get_metadata"].update(ServerSideEncryption="aws:kms"))

    def test_any_private_cloud_metadata_change_or_identity_mismatch_stops(self):
        address = bundle()["contract"]["candidate"]["address"]
        self.rejected(lambda value: value["after"]["metadata"][address]["payload"]["opaque"].update(marker="changed"))
        self.rejected(lambda value: value["after"]["metadata"][address]["payload"].pop("opaque"))
        self.rejected(lambda value: value["before"]["metadata"][address].update(project_id="test-youtube-study-space"))
        self.rejected(lambda value: value["before"]["metadata"][address].update(resource_id="another-private-id"))
        self.rejected(lambda value: value["before"]["metadata"].pop(address))

    def test_whole_iam_unknown_types_wrong_project_schema_and_api_mutation_are_rejected(self):
        for resource_type in ("google_project_iam_policy", "google_service_account_iam_policy", "google_project_iam_binding", "google_compute_instance"):
            self.rejected(lambda value: value["contract"]["candidate"].update(type=resource_type))
        self.rejected(lambda value: value["contract"]["candidate"].update(schema_version=99))
        self.rejected(lambda value: value["contract"].update(project_id="test-youtube-study-space"))
        self.rejected(lambda value: value["contract"].update(state_key="youtube-study-space/dev/terraform.tfstate"))
        self.rejected(lambda value: value["contract"]["adopted"][0]["values"].update(id="projects/test-youtube-study-space/topics/fixture-0"))
        self.rejected(lambda value: value["contract"]["candidate"]["values"].update(disable_on_destroy=True))
        self.rejected(lambda value: value["contract"]["candidate"]["values"].update(disable_dependent_services=True))
        self.rejected(lambda value: value["contract"]["adopted"].append(deepcopy(value["contract"]["adopted"][0])))
        self.rejected(lambda value: value["contract"].update(expires_utc="2026-10-14T10:00:00Z"))
        self.rejected(lambda value: value["contract"].update(expires_utc="2026-10-16T10:00:00Z"))
        with self.assertRaises(ValueError):
            gate.validate(bundle(), git_sha="b" * 40, now=NOW)

    def test_import_id_cannot_be_detached_from_same_project_or_foreign_resource_identity(self):
        for wrong_id in ("arbitrary-private-id", "youtube-study-space/unrelated.googleapis.com",
                         "test-youtube-study-space/fixture.googleapis.com"):
            def change(value):
                candidate = value["contract"]["candidate"]
                candidate["import_id"] = wrong_id
                value["plan_before"]["resource_changes"][-1]["change"]["importing"] = {"id": wrong_id}
            with self.subTest(wrong_id=wrong_id):
                self.rejected(change)

    def test_retained_bucket_and_native_backup_keep_their_provider_project_and_id_shapes(self):
        descriptors = [
            {"address": "google_storage_bucket.retained", "type": "google_storage_bucket", "schema_version": 4,
             "import_id": "fixture-retained-bucket", "values": {"id": "fixture-retained-bucket", "project": "111111111111"}},
            {"address": "google_firestore_backup_schedule.retained", "type": "google_firestore_backup_schedule", "schema_version": 0,
             "import_id": f"projects/{gate.PROJECT}/databases/(default)/backupSchedules/fixture",
             "values": {"id": f"projects/{gate.PROJECT}/databases/(default)/backupSchedules/fixture", "project": gate.PROJECT, "database": "(default)"}},
        ]
        for descriptor in descriptors:
            value = bundle(adopted_count=0)
            value["contract"]["candidate"] = descriptor
            metadata = {descriptor["address"]: {"project_id": gate.PROJECT, "resource_id": descriptor["values"]["id"], "payload": {"fixture": "unchanged"}}}
            value["before"]["metadata"] = deepcopy(metadata)
            value["after"]["metadata"] = deepcopy(metadata)
            value["after"]["snapshot"] = snapshot(state([descriptor], serial=8), "fixture-after")
            value["plan_before"] = plan([descriptor], importing={descriptor["address"]})
            value["plan_after"] = plan([descriptor])
            gate.validate(value, git_sha=SHA, now=NOW)

    def test_service_account_member_binds_pinned_space_import_and_slash_state_id(self):
        for conditional in (False, True):
            sa = f"projects/{gate.PROJECT}/serviceAccounts/fixture@{gate.PROJECT}.iam.gserviceaccount.com"
            role, member = "roles/iam.workloadIdentityUser", "principalSet://iam.googleapis.com/projects/111111111111/locations/global/workloadIdentityPools/fixture/attribute.aws_role/fixture"
            condition = {"title": "fixture-title", "description": "fixture-description", "expression": "true"}
            candidate = {"address": "module.runtime.google_service_account_iam_member.runtime[\"fixture\"]",
                "type": "google_service_account_iam_member", "schema_version": 0,
                "import_id": " ".join([sa, role, member] + ([condition["title"]] if conditional else [])),
                "values": {"id": "/".join([sa, role, member] + ([condition[field] for field in ("title", "description", "expression")] if conditional else [])),
                    "service_account_id": sa, "role": role, "member": member, "condition": [condition] if conditional else [], "etag": "fixture-etag"}}
            value = bundle(adopted_count=0)
            value["contract"]["candidate"] = candidate
            metadata = {candidate["address"]: {"project_id": gate.PROJECT, "resource_id": candidate["values"]["id"], "payload": {"bindings": [role]}}}
            value["before"]["metadata"] = deepcopy(metadata)
            value["after"]["metadata"] = deepcopy(metadata)
            value["after"]["snapshot"] = snapshot(state([candidate], serial=8), "fixture-after")
            value["plan_before"] = plan([candidate], importing={candidate["address"]})
            value["plan_after"] = plan([candidate])
            gate.validate(value, git_sha=SHA, now=NOW)
            self.rejected(lambda value: value["contract"]["candidate"].update(import_id=" ".join([sa, role, "another-member"])), value)
            self.rejected(lambda value: value["contract"]["candidate"]["values"].update(id="unrelated-state-id"), value)
            for unsupported in ("user:Owner@EXAMPLE.invalid", "serviceAccount:fixture@example.invalid", member + " whitespace"):
                self.rejected(lambda value: value["contract"]["candidate"]["values"].update(member=unsupported), value)
            if conditional:
                self.rejected(lambda value: value["contract"]["candidate"]["values"]["condition"][0].update(title="ambiguous title"), value)

    def test_cli_has_owner_only_bounded_inputs_and_fixed_private_errors(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "bundle.json"
            source.write_text(gate.canonical(bundle(now=datetime.now(timezone.utc))))
            source.chmod(0o600)
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                self.assertEqual(gate.main(["--bundle", str(source), "--git-sha", SHA]), 0)
            public = json.loads(out.getvalue())
            self.assertFalse(public["live_readiness_attested"])
            self.assertNotIn("private-sentinel", out.getvalue() + err.getvalue())
            self.assertNotIn("fixture", out.getvalue() + err.getvalue())
            link = Path(directory) / "link"
            link.symlink_to(source)
            fifo = Path(directory) / "fifo"
            os.mkfifo(fifo)
            for arguments in (["--bundle", str(link), "--git-sha", SHA], ["--bundle", str(fifo), "--git-sha", SHA],
                              ["--bundle", str(source), "--unknown", "private-sentinel"]):
                out, err = io.StringIO(), io.StringIO()
                with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                    self.assertEqual(gate.main(arguments), 1)
                self.assertEqual(out.getvalue(), "")
                self.assertEqual(err.getvalue(), "Production offline validation STOP; private diagnostic suppressed.\n")
            for body in ('{"private":"first","private":"private-sentinel"}', '{"private":NaN}', '{"private":Infinity}', 'private-sentinel'):
                source.write_text(body)
                with contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(gate.main(["--bundle", str(source), "--git-sha", SHA]), 1)
            source.write_bytes(b"x" * (gate.MAX_BUNDLE_BYTES + 1))
            with contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(gate.main(["--bundle", str(source), "--git-sha", SHA]), 1)
            source.chmod(0o644)
            with contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(gate.main(["--bundle", str(source), "--git-sha", SHA]), 1)


class TerraformInitialStateContractTest(unittest.TestCase):
    def test_real_pinned_output_only_plan_state_and_post_noop_satisfy_initial_contract(self):
        # This local root has no backend, provider, resource or credential. It
        # verifies actual pinned Terraform JSON/state shapes, not cloud readiness.
        with tempfile.TemporaryDirectory(prefix="production-empty-local-") as directory:
            root = Path(directory)
            (root / "main.tf").write_text('terraform { required_version = "=' + gate.TERRAFORM_VERSION + '" }\n'
                'output "environment" { value = "production" }\n'
                'output "project_id" { value = "youtube-study-space" }\n')
            def terraform(*arguments):
                result = subprocess.run(["terraform", f"-chdir={root}", *arguments], capture_output=True,
                    env=credentialless_environment(), check=False, timeout=30)
                if result.returncode:
                    raise RuntimeError("Local output-only contract verification failed; output suppressed.")
                return result.stdout
            terraform("init", "-backend=false", "-input=false")
            terraform("plan", "-input=false", "-out=before.tfplan")
            before_plan = json.loads(terraform("show", "-json", "before.tfplan"))
            terraform("apply", "-input=false", "before.tfplan")
            local_state = json.loads((root / "terraform.tfstate").read_text())
            terraform("plan", "-input=false", "-out=after.tfplan")
            after_plan = json.loads(terraform("show", "-json", "after.tfplan"))
            value = bundle("initialize")
            value["plan_before"], value["plan_after"] = before_plan, after_plan
            value["after"]["snapshot"] = snapshot(local_state, "fixture-after")
            self.assertEqual(gate.validate(value, git_sha=SHA, now=NOW)["managed_after"], 0)


if __name__ == "__main__":
    unittest.main()
