#!/usr/bin/env python3
"""Validate the public wire contract and synthetic fixtures, including failure cases."""
import copy
import json
from pathlib import Path

import jsonschema
import yaml

ROOT = Path(__file__).resolve().parents[2]
api = yaml.safe_load((ROOT / "docs/mypage/openapi.yaml").read_text())
schema = {"$ref": "#/components/schemas/MyPage", "components": api["components"]}
jsonschema.Draft202012Validator.check_schema(schema)
validator = jsonschema.Draft202012Validator(schema, format_checker=jsonschema.FormatChecker())
expected_paths = {
    "/api/auth/youtube/start": "post",
    "/api/auth/youtube/callback": "get",
    "/api/auth/youtube/channel": "get",
    "/api/auth/youtube/confirm": "post",
    "/api/auth/session/complete": "post",
    "/api/mypage": "get",
    "/api/privacy/requests": "post",
    "/api/privacy/requests/status": "post",
}
assert set(api["paths"]) == set(expected_paths)
for path, method in expected_paths.items():
    assert set(api["paths"][path]) == {method}
    op = api["paths"][path][method]
    if path in ("/api/mypage", "/api/auth/session/complete", "/api/privacy/requests", "/api/privacy/requests/status"):
        assert op["security"] == [{"FirebaseAuth": [], "AppCheck": []}]
    elif path.endswith("callback"):
        assert op["security"] == []
    else:
        assert "AppCheck" in op["security"][0]
assert api["components"]["securitySchemes"]["OAuthTransaction"]["name"] == "__session"

fixtures = []
for path in sorted((ROOT / "docs/mypage/fixtures").glob("*.json")):
    data = json.loads(path.read_text())
    validator.validate(data)
    fixtures.append(data)
assert len(fixtures) >= 2


def rejects(mutator):
    data = copy.deepcopy(fixtures[0])
    mutator(data)
    assert not validator.is_valid(data), "invalid public response was accepted"


rejects(lambda x: x.update(uid="synthetic-internal-id"))
rejects(lambda x: x.update(timezone="UTC"))
rejects(lambda x: x["recent7Days"]["data"].pop())
rejects(lambda x: x["summary"]["data"]["today"].update(workSec=-1))
rejects(lambda x: x["summary"]["data"]["today"].update(workSec=1.5))
rejects(lambda x: x["summary"]["data"]["today"].update(availability="unavailable", workSec=0))
rejects(lambda x: x["current"].update(availability="unavailable", reasonCode="SOURCE_UNAVAILABLE"))
rejects(lambda x: x["current"]["data"].update(breakWorkName="obsolete"))
print(f"MyPage eight-endpoint schema and {len(fixtures)} synthetic fixtures passed")


# Purpose is authoritative server-side. A support response cannot carry a login
# token, and the normal response cannot masquerade as a support proof.
def contract_validator(name):
    return jsonschema.Draft202012Validator({"$ref": f"#/components/schemas/{name}", "components": api["components"]}, format_checker=jsonschema.FormatChecker())
confirm = contract_validator("ConfirmResponse")
confirm.validate({"purpose": "login", "customToken": "synthetic-custom"})
confirm.validate({"purpose": "support", "requestRef": "a" * 64})
for invalid in [{"customToken": "synthetic-custom"}, {"purpose": "support", "customToken": "synthetic-custom", "requestRef": "a" * 64}, {"purpose": "login", "requestRef": "a" * 64}, {"purpose": "support", "requestRef": "not-opaque"}]:
    assert not confirm.is_valid(invalid)
channel = contract_validator("ChannelResponse")
common = {"displayName": "Synthetic channel", "handle": None, "avatarUrl": None, "confirmationRef": "b" * 64}
channel.validate({**common, "purpose": "login"})
channel.validate({**common, "purpose": "support", "supportPurpose": "delete"})
assert not channel.is_valid({**common, "purpose": "support"})
assert not channel.is_valid({**common, "purpose": "support", "supportPurpose": "login"})
assert not channel.is_valid({**common, "purpose": "login", "supportPurpose": "delete"})
start = contract_validator("StartRequest")
normal = {"privacyPolicyVersion": "synthetic-p1", "privacyAccepted": True, "termsVersion": "synthetic-t1", "termsAccepted": True}
start.validate(normal)
start.validate({**normal, "supportChallenge": "c" * 64})
for invalid in [None, "", "client-channel"]:
    assert not start.is_valid({**normal, "supportChallenge": invalid})
assert not start.is_valid({**normal, "targetChannel": "UCsynthetic"})
print("Login/support discriminated contract and purpose isolation passed")

intake = contract_validator("PrivacyIntakeRequest")
intake.validate({"submissionKey": "synthetic-key-0001", "purpose": "delete", "body": "Synthetic request"})
for invalid in [
    {"submissionKey": "synthetic-key-0001", "purpose": "login", "body": "Synthetic request"},
    {"submissionKey": "synthetic-key-0001", "purpose": "delete", "body": "Synthetic request", "targetChannel": "UCvictim"},
]:
    assert not intake.is_valid(invalid)
receipt = contract_validator("PrivacyIntakeReceipt")
receipt.validate({"requestRef": "a" * 64, "supportChallenge": "b" * 64, "purpose": "delete", "status": "awaiting_proof", "acceptedAt": "2026-10-09T00:00:00Z", "deleteBy": "2026-10-16T00:00:00Z"})
status_schema = contract_validator("PrivacyRequestStatus")
status_schema.validate({"requestRef": "a" * 64, "purpose": "delete", "status": "verified", "acceptedAt": "2026-10-09T00:00:00Z", "deleteBy": "2026-10-16T00:00:00Z", "verifiedAt": "2026-10-09T01:00:00Z", "reply": "Synthetic reply"})
assert not status_schema.is_valid({"requestRef": "a" * 64, "purpose": "delete", "status": "awaiting_proof", "acceptedAt": "2026-10-09T00:00:00Z", "deleteBy": "2026-10-16T00:00:00Z", "verifiedAt": "2026-10-09T01:00:00Z", "reply": "Synthetic reply"})

# Restriction wire codes are stable and cannot carry internal reason/target data.
error = contract_validator("Error")
for code in ["SERVICE_ACCESS_RESTRICTED", "DATA_DELETION_IN_PROGRESS", "TEMPORARY_UNAVAILABLE"]:
    error.validate({"error": {"code": code, "message": code, "requestId": "synthetic-request"}})
