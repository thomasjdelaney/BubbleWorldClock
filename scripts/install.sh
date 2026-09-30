#!/bin/sh
set -eu

repo="thomasjdelaney/BubbleWorldClock"
api="https://api.github.com/repos/$repo"

asset_name() {
    os=$1 arch=$2 version=$3
    version=${version#v}
    case "$os" in linux) os_name=Linux ;; darwin) os_name=Darwin ;; *) return 1 ;; esac
    case "$arch" in
        x86_64|amd64) arch_name=x86_64 ;;
        arm64|aarch64) arch_name=arm64 ;;
        *) return 1 ;;
    esac
    printf 'BubbleWorldClock_%s_%s_%s.tar.gz\n' "$version" "$os_name" "$arch_name"
}

verify_checksum() {
    sums=$1 archive=$2 name=${2##*/}
    expected=$(awk -v name="$name" '$2 == name || $2 == "*" name {print $1; exit}' "$sums")
    [ -n "$expected" ] || { echo "No checksum entry for $name" >&2; return 1; }
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        echo "Need sha256sum or shasum to verify the release" >&2; return 1
    fi
    [ "$actual" = "$expected" ] || { echo "Checksum mismatch for $name" >&2; return 1; }
}

if [ "${1-}" = --asset-name ]; then
    [ "$#" -eq 4 ] || exit 2
    asset_name "$2" "$3" "$4" || { echo "Unsupported OS/architecture: $2/$3" >&2; exit 1; }
    exit
fi
if [ "${1-}" = --verify-checksum ]; then
    [ "$#" -eq 3 ] || exit 2
    verify_checksum "$2" "$3"
    exit
fi

version=
install_dir="${HOME:?HOME must be set}/.local/bin"
while [ "$#" -gt 0 ]; do
    case "$1" in
        --version) [ "$#" -ge 2 ] || { echo "--version needs a value" >&2; exit 2; }; version=$2; shift 2 ;;
        --install-dir) [ "$#" -ge 2 ] || { echo "--install-dir needs a value" >&2; exit 2; }; install_dir=$2; shift 2 ;;
        -h|--help) echo "Usage: install.sh [--version VERSION] [--install-dir DIR]"; exit 0 ;;
        *) echo "Unknown option: $1" >&2; exit 2 ;;
    esac
done

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in linux|darwin) ;; *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;; esac
arch=$(uname -m)
case "$arch" in x86_64|amd64|aarch64|arm64) ;; *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; esac
if [ "$os" = linux ] && [ "$arch" = arm64 ]; then arch=aarch64; fi
if [ "$os" = darwin ] && [ "$arch" = aarch64 ]; then arch=arm64; fi

if [ -z "$version" ]; then
    release_url="$api/releases/latest"
else
    case "$version" in v*) ;; *) version="v$version" ;; esac
    release_url="$api/releases/tags/$version"
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/bubbleworldclock.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
if ! command -v curl >/dev/null 2>&1; then echo "curl is required" >&2; exit 1; fi
curl -fsSL "$release_url" -o "$tmp/release.json" || { echo "Could not find release: ${version:-latest stable}" >&2; exit 1; }
version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp/release.json" | head -n 1)
[ -n "$version" ] || { echo "Could not read release version" >&2; exit 1; }
asset=$(asset_name "$os" "$arch" "$version") || { echo "Unsupported OS/architecture: $os/$arch" >&2; exit 1; }
base="https://github.com/$repo/releases/download/$version"
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
verify_checksum "$tmp/checksums.txt" "$tmp/$asset"

mkdir -p "$tmp/extract" "$install_dir"
tar -xzf "$tmp/$asset" -C "$tmp/extract"
[ -f "$tmp/extract/bubble-world-clock" ] || { echo "Archive does not contain bubble-world-clock" >&2; exit 1; }
install -m 0755 "$tmp/extract/bubble-world-clock" "$install_dir/bubble-world-clock"

profile=${HOME:?}/.profile
if [ "$os" = darwin ]; then profile="$HOME/.zprofile"; fi
path_line='case ":$PATH:" in *:"$HOME/.local/bin":*) ;; *) PATH="$HOME/.local/bin:$PATH" ;; esac'
if [ "$install_dir" = "$HOME/.local/bin" ] && [ -f "$profile" ] && ! grep -Fq '# BubbleWorldClock PATH' "$profile"; then
    { printf '\n# BubbleWorldClock PATH\n%s\nexport PATH\n' "$path_line"; } >> "$profile"
elif [ "$install_dir" = "$HOME/.local/bin" ] && [ ! -f "$profile" ]; then
    { printf '# BubbleWorldClock PATH\n%s\nexport PATH\n' "$path_line"; } > "$profile"
fi
echo "Installed bubble-world-clock to $install_dir"
if [ "$install_dir" = "$HOME/.local/bin" ]; then echo "Open a new shell (or source ~/.profile) to update PATH."; fi
