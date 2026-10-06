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
}
assert set(api["paths"]) == set(expected_paths)
for path, method in expected_paths.items():
    assert set(api["paths"][path]) == {method}
    op = api["paths"][path][method]
    if path in ("/api/mypage", "/api/auth/session/complete"):
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
print(f"MyPage six-endpoint schema and {len(fixtures)} synthetic fixtures passed")
