#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
fsl_root="$(cd -- "$script_dir/.." && pwd)"
version="$(tr -d '[:space:]' <"$fsl_root/VERSION")"
checksum_file="$fsl_root/SHA256SUMS"
tool_root="${FSL_TOOL_DIR:-$fsl_root/.tools}"

case "$(uname -s):$(uname -m)" in
	Linux:x86_64)
		asset="fslc-linux-x64"
		;;
	Linux:aarch64|Linux:arm64)
		asset="fslc-linux-arm64"
		;;
	Darwin:arm64)
		asset="fslc-macos-arm64"
		;;
	*)
		echo "Unsupported platform for pinned fslc: $(uname -s) $(uname -m)" >&2
		exit 2
		;;
esac

expected_sha="$(awk -v asset="$asset" '$2 == asset { print $1 }' "$checksum_file")"
if [[ -z "$expected_sha" ]]; then
	echo "Missing checksum for $asset" >&2
	exit 2
fi

sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{ print $1 }'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{ print $1 }'
	else
		echo "sha256sum or shasum is required" >&2
		exit 2
	fi
}

install_dir="$tool_root/$version"
binary="$install_dir/$asset"
mkdir -p "$install_dir"

if [[ -f "$binary" ]]; then
	actual_sha="$(sha256_file "$binary")"
	if [[ "$actual_sha" == "$expected_sha" ]]; then
		echo "$binary"
		exit 0
	fi
	echo "Cached fslc checksum mismatch; reinstalling $asset" >&2
	rm -f "$binary"
fi

tmp="$(mktemp "$install_dir/.${asset}.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
url="https://github.com/ymm-oss/fsl/releases/download/${version}/${asset}"

echo "Downloading pinned FSL ${version} (${asset})" >&2
curl -fsSL --retry 3 --retry-delay 1 "$url" -o "$tmp"
actual_sha="$(sha256_file "$tmp")"
if [[ "$actual_sha" != "$expected_sha" ]]; then
	echo "fslc checksum mismatch: expected=$expected_sha actual=$actual_sha" >&2
	exit 1
fi

chmod 0755 "$tmp"
mv "$tmp" "$binary"
trap - EXIT

echo "$binary"
