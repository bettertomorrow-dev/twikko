#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/release"
printf 'fixture binary\n' > "$tmp/release/twikko"
tar -czf "$tmp/release/archive.tar.gz" -C "$tmp/release" twikko
cat > "$tmp/bin/uname" <<'MOCK'
#!/usr/bin/env bash
case "$1" in -s) echo "$TEST_OS";; -m) echo "$TEST_ARCH";; esac
MOCK
cat > "$tmp/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *releases/latest* ]]; then
  printf 'https://github.com/bettertomorrow-dev/twikko/releases/tag/v1.2.3'
  exit
fi
url=$2
out=$4
if [[ "$url" == *checksums.txt ]]; then
  digest=$(sha256sum "$FIXTURE/archive.tar.gz" | awk '{print $1}')
  [[ "${CORRUPT:-}" != yes ]] || digest=bad
  printf '%s  %s\n' "$digest" "$ASSET" > "$out"
else
  cp "$FIXTURE/archive.tar.gz" "$out"
fi
MOCK
chmod +x "$tmp/bin/"*
export PATH="$tmp/bin:$PATH" FIXTURE="$tmp/release" TWIKKO_INSTALL_DIR="$tmp/installed"
for TEST_OS in Linux Darwin; do
  for TEST_ARCH in x86_64 aarch64; do
    platform=linux; [[ "$TEST_OS" != Darwin ]] || platform=darwin
    arch=amd64; [[ "$TEST_ARCH" != aarch64 ]] || arch=arm64
    export TEST_OS TEST_ARCH ASSET="twikko_1.2.3_${platform}_${arch}.tar.gz"
    bash "$root/install.sh"
    cmp "$tmp/release/twikko" "$TWIKKO_INSTALL_DIR/twikko"
    test -x "$TWIKKO_INSTALL_DIR/twikko"
  done
done
printf 'existing binary\n' > "$TWIKKO_INSTALL_DIR/twikko"
if CORRUPT=yes bash "$root/install.sh"; then echo 'Accepted corrupt archive' >&2; exit 1; fi
[[ "$(cat "$TWIKKO_INSTALL_DIR/twikko")" == 'existing binary' ]]
if TWIKKO_VERSION=invalid bash "$root/install.sh"; then echo 'Accepted invalid version' >&2; exit 1; fi
if TEST_ARCH=unsupported bash "$root/install.sh"; then echo 'Accepted unsupported architecture' >&2; exit 1; fi
echo 'Bash installer checks passed.'
