#!/bin/sh
set -eu
repo="vika2603/ccs"
version="${CCS_VERSION:-latest}"
prefix="${CCS_PREFIX:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin|linux) ;;
  *) echo "unsupported os: $os" >&2; exit 1 ;;
esac

archive="ccs_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $archive" >&2
curl -fSL --progress-bar "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"

hash_line=$(grep " $archive\$" "$tmp/SHA256SUMS" || true)
if [ -z "$hash_line" ]; then
  echo "no checksum entry for $archive in SHA256SUMS" >&2
  exit 1
fi
(cd "$tmp" && printf '%s\n' "$hash_line" | shasum -a 256 -c -) || {
  echo "checksum mismatch for $archive" >&2
  exit 1
}

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$prefix"
install -m 0755 "$tmp/ccs" "$prefix/ccs"
echo "installed $prefix/ccs"
echo "add this to your shell rc:"
echo '    eval "$(ccs shell-init)"'
