"""Private I/O and plan-shape helpers for normal Terraform Plan/Apply.

The legacy input schema identifiers are wire compatibility with existing
Environment secrets, not executable migration modes. No import/initialization
controller or cloud credentials are acquired by this module.
"""
from __future__ import annotations

import base64
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.error
import urllib.request
import zlib

ROOT = Path(__file__).resolve().parents[3]

MAX_BYTES = 4 * 1024 * 1024
INPUT_WIRE_BYTES = 48 * 1024
INPUT_SCHEMA = 'gcp-migration-inputs-v1'
PACKED_INPUT_SCHEMA = 'gcp-migration-inputs-packed-v1'
INPUT_FIELDS = frozenset({'schema_version', 'tfvars', 'manifest', 'plan_integrity_key'})
VERSION = (ROOT / 'infra/gcp/.terraform-version').read_text().strip()


def need(condition, reason=None):
    if not condition:
        if reason is not None:
            raise ContractError(reason)
        raise ValueError('Terraform runtime contract rejected')


def close_http_error(error):
    # urllib responses otherwise emit ResourceWarning with private URL/reason
    # on newer Python runtimes even when exception text is never printed.
    if isinstance(error, urllib.error.HTTPError):
        try:
            error.close()
        except Exception:
            pass


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def pairs(items):
    result = {}
    for key, value in items:
        need(key not in result)
        result[key] = value
    return result


def decode(raw):
    need(0 < len(raw) <= 16 * MAX_BYTES)
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: need(False))


def run(args, *, cwd=None, env=None, codes=(0,), timeout=90):
    result = subprocess.run(args, cwd=cwd, env=env, capture_output=True, timeout=timeout)
    need(len(result.stdout) <= 16 * MAX_BYTES and len(result.stderr) <= MAX_BYTES
         and result.returncode in codes)
    return result


def write(path, raw):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def input_value(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    value = decode(raw.decode('utf-8'))
    canonical(value)  # Reject exponent overflow as well as JSON NaN/Infinity.
    need(type(value) is dict and set(value) == INPUT_FIELDS
         and value['schema_version'] == INPUT_SCHEMA)
    return value


def decode_input_transport(raw):
    """Decode one bounded transport; all admission/HMAC uses the original value."""
    try:
        need(type(raw) is bytes and 0 < len(raw) <= INPUT_WIRE_BYTES)
        value = decode(raw.decode('utf-8'))
        if type(value) is dict and value.get('schema_version') == PACKED_INPUT_SCHEMA:
            need(set(value) == {'schema_version', 'codec', 'payload'}
                 and value['codec'] == 'zlib+base64' and type(value['payload']) is str
                 and b'\n' not in raw and b'\r' not in raw)
            payload = value['payload'].encode('ascii')
            compressed = base64.b64decode(payload, validate=True)
            need(base64.b64encode(compressed) == payload)
            stream = zlib.decompressobj()
            raw = stream.decompress(compressed, MAX_BYTES + 1)
            need(0 < len(raw) <= MAX_BYTES and stream.eof
                 and not stream.unused_data and not stream.unconsumed_tail)
        # A second wrapper (or any unknown schema/field) is rejected here.
        return input_value(raw)
    except (ValueError, TypeError, UnicodeError, RecursionError, zlib.error):
        raise ValueError('Terraform runtime contract rejected') from None


def encode_input_transport(raw):
    """Offline lossless transport encoder; never modifies the reviewed input file."""
    try:
        expected = input_value(raw)
        packed = canonical({'schema_version': PACKED_INPUT_SCHEMA, 'codec': 'zlib+base64',
                            'payload': base64.b64encode(zlib.compress(raw, 9)).decode('ascii')})
        need(len(packed) <= INPUT_WIRE_BYTES and decode_input_transport(packed) == expected)
        return packed
    except (ValueError, TypeError, UnicodeError, RecursionError, zlib.error):
        raise ValueError('Terraform runtime contract rejected') from None


def aws(env, *args):
    return decode(run(['aws', *args, '--output', 'json'], env=env).stdout)


def google(env, host, path, body=None):
    request = urllib.request.Request('https://' + host + '/' + path,
        headers={'Authorization': 'Bearer ' + env['GCP_ACCESS_TOKEN'], 'Content-Type': 'application/json'},
        data=None if body is None else canonical(body))
    with urllib.request.urlopen(request, timeout=30) as response:
        raw = response.read(MAX_BYTES + 1)
        need(len(raw) <= MAX_BYTES)
    return decode(raw)


def snapshot(env, root, name):
    target = env['NORMAL_TARGET']
    bucket, key = env['STATE_BUCKET'], f'youtube-study-space/{target}/terraform.tfstate'
    need(re.fullmatch('[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]', bucket) is not None)
    common = ['--bucket', bucket, '--expected-bucket-owner', env['STATE_ACCOUNT_ID']]
    for prefix in (key + '.tflock', key.rsplit('/', 1)[0] + '/workspaces/'):
        listing = aws(env, 's3api', 'list-objects-v2', *common, '--prefix', prefix, '--max-keys', '2')
        need(listing.get('Name') == bucket and listing.get('Prefix') == prefix
             and type(listing.get('KeyCount')) is int and listing['KeyCount'] == 0
             and listing.get('IsTruncated') is False and listing.get('Contents', []) == []
             and listing.get('CommonPrefixes', []) == [] and 'NextContinuationToken' not in listing)
    listing = aws(env, 's3api', 'list-objects-v2', *common, '--prefix', key, '--max-keys', '2')
    need(listing.get('Name') == bucket and listing.get('Prefix') == key
         and listing.get('IsTruncated') is False and type(listing.get('KeyCount')) is int
         and listing.get('CommonPrefixes', []) == [] and 'NextContinuationToken' not in listing)
    need(listing.get('KeyCount') == 1 and [item['Key'] for item in listing.get('Contents', [])] == [key])
    head = aws(env, 's3api', 'head-object', *common, '--key', key)
    need(type(head.get('ContentLength')) is int and 0 < head['ContentLength'] <= MAX_BYTES
         and head.get('ServerSideEncryption') == 'AES256'
         and type(head.get('VersionId')) is str and head['VersionId'] not in {'', 'null'})
    need(listing['Contents'][0].get('Size') == head['ContentLength']
         and listing['Contents'][0].get('ETag') == head.get('ETag'))
    path = root / (name + '.tfstate')
    result = aws(env, 's3api', 'get-object', *common, '--key', key,
                 '--range', f'bytes=0-{MAX_BYTES-1}', str(path))
    raw = path.read_bytes()
    os.chmod(path, 0o600)
    need(len(raw) == head['ContentLength'] == result.get('ContentLength')
         and result.get('VersionId') == head['VersionId'] and result.get('ETag') == head.get('ETag')
         and aws(env, 's3api', 'head-object', *common, '--key', key) == head)
    state = decode(raw)
    need(state.get('version') == 4 and state.get('terraform_version') == VERSION
         and type(state.get('serial')) is int and state['serial'] >= 0
         and type(state.get('lineage')) is str)
    return {'head': head, 'raw': raw.decode(), 'state': state}


def terraform(env, root, tfroot, name, args, *, codes=(0,)):
    clean = {key: value for key, value in env.items()
             if not key.startswith(('TF_VAR_', 'TF_CLI_ARGS', 'TF_LOG'))}
    clean.update(TF_IN_AUTOMATION='true', TF_INPUT='false', TF_WORKSPACE='default',
                 TF_CLI_CONFIG_FILE=str(root / 'terraform.rc'), CHECKPOINT_DISABLE='1')
    result = run(['terraform', *args], cwd=tfroot, env=clean, codes=codes, timeout=600)
    write(root / (name + '.stdout'), result.stdout)
    write(root / (name + '.stderr'), result.stderr)
    return result


GOOGLE = "registry.terraform.io/hashicorp/google"


RESOURCE_KEYS = frozenset({
    "address", "module_address", "mode", "type", "name", "index", "provider_name",
})


CHANGE_KEYS = frozenset({
    "actions", "before", "after", "before_sensitive", "after_sensitive", "after_unknown",
    "importing", "replace_paths", "before_identity", "after_identity",
})


class ContractError(ValueError):
    """Fixed failure text; private input data must never enter an exception."""

    def __init__(self, reason="SHAPE"):
        self.reason = reason if reason in {
            "SHAPE", "SCHEMA", "MASK",
        } else "SHAPE"
        super().__init__("GCP plan contract STOP: " + self.reason)


def _mask(value, *, unknown=False):
    if type(value) is dict:
        need(all(type(key) is str for key in value), "MASK")
        for child in value.values():
            _mask(child, unknown=unknown)
    elif type(value) is list:
        for child in value:
            _mask(child, unknown=unknown)
    else:
        need(type(value) is bool and (not unknown or value is False), "MASK")


def _schema(provider_schema, resource_type):
    try:
        need(type(provider_schema) is dict and provider_schema.get("format_version") == "1.0", "SCHEMA")
        schema = provider_schema["provider_schemas"][GOOGLE]["resource_schemas"][resource_type]
        need(type(schema) is dict and type(schema.get("version")) is int and schema["version"] >= 0, "SCHEMA")
        block = schema["block"]
        need(type(block) is dict and type(block.get("attributes", {})) is dict
             and type(block.get("block_types", {})) is dict, "SCHEMA")
        return schema
    except (KeyError, TypeError):
        raise ContractError("SCHEMA") from None


def observation_keys(provider_schema, resource_type, fields):
    """Provider-verified read-only metadata for normal Plan validation."""
    if resource_type != "google_bigquery_table":
        return frozenset()
    attrs = _schema(provider_schema, resource_type)["block"].get("attributes", {})
    for name in fields:
        item = attrs.get(name)
        need(type(item) is dict and item.get("computed") is True
             and item.get("optional", False) is False and item.get("required", False) is False, "SCHEMA")
    return frozenset(fields)
