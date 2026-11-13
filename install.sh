#!/usr/bin/env bash
set -euo pipefail

repo=bettertomorrow-dev/twikko
case "$(uname -s)" in
  Darwin) platform=darwin ;;
  Linux) platform=linux ;;
  *) echo 'Unsupported operating system.' >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo 'Unsupported architecture.' >&2; exit 1 ;;
esac

version=${TWIKKO_VERSION:-}
if [[ -z "$version" ]]; then
  # Follow GitHub's latest-release redirect without requiring a JSON parser.
  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version=${url##*/}
fi
[[ "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo 'Expected version vX.Y.Z.' >&2; exit 1; }
asset="twikko_${version#v}_${platform}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v name="$asset" '$2 == name {print $1; exit}' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
else
  actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
fi
[[ -n "$expected" && "$expected" == "$actual" ]] || { echo 'SHA-256 verification failed.' >&2; exit 1; }
tar -xzf "$tmp/$asset" -C "$tmp" twikko
install_dir=${TWIKKO_INSTALL_DIR:-$HOME/.local/bin}
mkdir -p "$install_dir"
install -m 755 "$tmp/twikko" "$install_dir/twikko"
printf 'Installed twikko %s to %s\n' "$version" "$install_dir/twikko"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) printf 'Add this directory to PATH: %s\n' "$install_dir" ;;
esac
