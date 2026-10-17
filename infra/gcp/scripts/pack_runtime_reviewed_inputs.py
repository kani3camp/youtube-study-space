#!/usr/bin/env python3
"""Offline, private, lossless transport for an already reviewed initial Pool fragment.

Default mode validates only. --pack writes one exclusive private wrapper; this
tool has no credential, cloud, GitHub, Secret, binding, or nonce operation.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import os
from pathlib import Path
import re
import stat
import sys

import runtime_ownership_packet as packet

ROOT = Path(__file__).resolve().parents[3]
CATALOG = ROOT / '.github/terraform/current-state-attestation-sources.json'


class PrivateParser(argparse.ArgumentParser):
    def error(self, message):
        raise ValueError('private pack arguments STOP')


def now():
    return datetime.now(timezone.utc)


def private_parent(path):
    packet.require(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    info = path.parent.lstat()
    packet.require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                   and stat.S_IMODE(info.st_mode) == 0o700)
    for directory in (path.parent, *path.parent.parents):
        marker = directory / '.git'
        if marker.exists() or marker.is_symlink():
            info = marker.lstat()
            # These empty sandbox guards are not repository checkouts.
            empty_guard = (marker in (Path('/tmp/.git'), Path('/workspace/.git'))
                           and stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o555
                           and not any(marker.iterdir()))
            packet.require(empty_guard)


def private_bytes(path):
    private_parent(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        packet.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                       and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
                       and 0 < info.st_size <= packet.MAX_PACKET_BYTES)
        raw = handle.read(packet.MAX_PACKET_BYTES + 1)
    packet.require(0 < len(raw) <= packet.MAX_PACKET_BYTES)
    return raw


def validate_fragment(raw, *, fragment_sha256, git_sha, issuer_sha, current_time=None):
    packet.require(type(fragment_sha256) is str and re.fullmatch(r'[0-9a-f]{64}', fragment_sha256)
                   and hashlib.sha256(raw).hexdigest() == fragment_sha256)
    value = packet.reviewed_fragment(packet.input_json(raw.decode('utf-8', errors='strict')))
    packet.require(type(git_sha) is str and re.fullmatch(r'[0-9a-f]{40}', git_sha)
                   and value['git_sha'] == git_sha and value['wave'] == 'pool')
    reviewed, expires = packet.timestamp(value['reviewed_utc']), packet.timestamp(value['expires_utc'])
    packet.require(reviewed <= (current_time or now()) < expires <= reviewed + timedelta(hours=24))
    # This initial-wave helper cannot create a cumulative chain or a binding.
    packet.require(value['adoption_receipts'] == []
                   and value['adopted'] == dict(pool=False, provider=False, grants=[], apis=[])
                   and value['selected'] == dict(pool=True, provider=False, grants=[], apis=[]))
    candidate = packet.prepare(value['inventory'])
    for key in ('adopted', 'selected'):
        packet.selection(value[key], candidate)
    sys.path.insert(0, str(ROOT / '.github/scripts'))
    import terraform_current_state_attestation as current_state
    current_state.reference(value['current_state_attestation'])
    catalog = current_state.load_catalog(packet.input_json(CATALOG.read_text(encoding='utf-8')))
    packet.require(type(issuer_sha) is str and re.fullmatch(r'[0-9a-f]{40}', issuer_sha)
                   and value['current_state_attestation']['source_sha'] == issuer_sha
                   and issuer_sha in catalog['issuers'])
    # Encoding uses these exact bytes. Compacting, if desired, must happen
    # explicitly before the fragment's independent review and hash are fixed.
    return packet.pack_reviewed_input(raw)


def write_private(path, raw):
    private_parent(path)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, 'wb') as handle:
            info = os.fstat(handle.fileno())
            packet.require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                           and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1)
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        packet.require(private_bytes(path) == raw)
    except Exception:
        path.unlink(missing_ok=True)
        raise


def main(argv=None):
    try:
        parser = PrivateParser(description=__doc__)
        parser.add_argument('--fragment', type=Path, required=True)
        parser.add_argument('--fragment-sha256', required=True)
        parser.add_argument('--git-sha', required=True)
        parser.add_argument('--issuer-sha', required=True)
        parser.add_argument('--pack', action='store_true')
        parser.add_argument('--output', type=Path)
        args = parser.parse_args(argv)
        packet.require(args.pack == (args.output is not None))
        raw = private_bytes(args.fragment)
        encoded = validate_fragment(raw, fragment_sha256=args.fragment_sha256,
                                    git_sha=args.git_sha, issuer_sha=args.issuer_sha)
        if args.pack:
            write_private(args.output, encoded)
    except Exception:
        print('STOP: private reviewed-input transport rejected; diagnostic suppressed.', file=sys.stderr)
        return 3
    print('Offline reviewed-input transport checked; no authentication or approval performed.')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
