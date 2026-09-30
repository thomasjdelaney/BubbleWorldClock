#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
installer=$root/scripts/install.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/bubbleworldclock-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

check_asset() {
    actual=$(sh "$installer" --asset-name "$1" "$2" "$3")
    [ "$actual" = "$4" ] || { echo "Unexpected asset: $actual" >&2; exit 1; }
}
check_asset linux x86_64 v1.2.3 BubbleWorldClock_1.2.3_Linux_x86_64.tar.gz
check_asset darwin arm64 v1.2.3 BubbleWorldClock_1.2.3_Darwin_arm64.tar.gz
check_asset linux aarch64 v1.2.3 BubbleWorldClock_1.2.3_Linux_arm64.tar.gz
if sh "$installer" --asset-name windows amd64 v1.2.3 >/dev/null 2>&1; then
    echo 'Unsupported OS was accepted' >&2
    exit 1
fi
if sh "$installer" --asset-name linux riscv64 v1.2.3 >/dev/null 2>&1; then
    echo 'Unsupported architecture was accepted' >&2
    exit 1
fi

printf 'installer checksum fixture\n' > "$tmp/archive.tar.gz"
if command -v sha256sum >/dev/null 2>&1; then
    hash=$(sha256sum "$tmp/archive.tar.gz" | awk '{print $1}')
else
    hash=$(shasum -a 256 "$tmp/archive.tar.gz" | awk '{print $1}')
fi
printf '%s  archive.tar.gz\n' "$hash" > "$tmp/checksums.txt"
sh "$installer" --verify-checksum "$tmp/checksums.txt" "$tmp/archive.tar.gz"
printf '%064d  archive.tar.gz\n' 0 > "$tmp/checksums.txt"
if sh "$installer" --verify-checksum "$tmp/checksums.txt" "$tmp/archive.tar.gz" >/dev/null 2>&1; then
    echo 'Invalid checksum was accepted' >&2
    exit 1
fi
printf '%s  another-file.tar.gz\n' "$hash" > "$tmp/checksums.txt"
if sh "$installer" --verify-checksum "$tmp/checksums.txt" "$tmp/archive.tar.gz" >/dev/null 2>&1; then
    echo 'Missing checksum entry was accepted' >&2
    exit 1
fi
echo 'Installer selection and checksum checks passed.'
