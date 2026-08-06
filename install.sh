#!/bin/sh

set -eu

repository=${HAPPY_MEMORY_REPOSITORY:-jorgeluis594/happy-memory}
api_base=${HAPPY_MEMORY_API_BASE:-https://api.github.com/repos/$repository}
download_base=${HAPPY_MEMORY_DOWNLOAD_BASE:-https://github.com/$repository/releases/download}
version=
bin_dir=${HOME:?HOME must be set}/.local/bin
temp_dir=
backup=
target=
installed=0
stage=

usage() {
  printf '%s\n' 'Usage: install.sh [--version vMAJOR.MINOR.PATCH] [--bin-dir DIRECTORY] [--help]'
}

fail() {
  printf 'happy-memory installer: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  status=$?
  if [ "$status" -ne 0 ] && [ "$installed" -eq 1 ] && [ -n "$target" ]; then
    rm -f "$target"
    if [ -n "$backup" ] && [ -f "$backup" ]; then
      mv "$backup" "$target" || :
    fi
  fi
  if [ -n "$backup" ] && [ -f "$backup" ]; then
    rm -f "$backup"
  fi
  if [ -n "$stage" ] && [ -f "$stage" ]; then
    rm -f "$stage"
  fi
  if [ -n "$temp_dir" ] && [ -d "$temp_dir" ]; then
    rm -rf "$temp_dir"
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

while [ "$#" -gt 0 ]; do
  case $1 in
    --version)
      [ "$#" -ge 2 ] || fail '--version requires a value'
      version=$2
      shift 2
      ;;
    --bin-dir)
      [ "$#" -ge 2 ] || fail '--bin-dir requires a value'
      bin_dir=$2
      shift 2
      ;;
    --help)
      usage
      exit 0
      ;;
    *) fail "unknown argument: $1" ;;
  esac
done

case ${HAPPY_MEMORY_UNAME_S:-$(uname -s)} in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail 'unsupported operating system; supported systems are Linux and macOS' ;;
esac
case ${HAPPY_MEMORY_UNAME_M:-$(uname -m)} in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail 'unsupported architecture; supported architectures are x86_64 and arm64' ;;
esac

download() {
  url=$1
  destination=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fL --silent --show-error --output "$destination" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q --https-only --output-document="$destination" "$url"
  else
    fail 'curl or wget is required to download a release'
  fi
}

if [ -z "$version" ]; then
  printf '%s\n' 'Resolving latest stable release...'
  temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/happy-memory-install.XXXXXX")
  download "$api_base/releases/latest" "$temp_dir/latest.json" || fail 'could not resolve the latest stable release'
  version=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$temp_dir/latest.json" | sed -n '1p')
fi
case $version in
  v0.0.0) ;;
  v[0-9]*.[0-9]*.[0-9]*)
    printf '%s' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' || fail 'version must match vMAJOR.MINOR.PATCH'
    ;;
  *) fail 'version must match vMAJOR.MINOR.PATCH' ;;
esac

[ -n "$temp_dir" ] || temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/happy-memory-install.XXXXXX")
plain_version=${version#v}
archive="happy-memory_${plain_version}_${os}_${arch}.tar.gz"
release_url="$download_base/$version"

printf 'Downloading %s...\n' "$archive"
download "$release_url/$archive" "$temp_dir/$archive" || fail "could not download $archive"
download "$release_url/checksums.txt" "$temp_dir/checksums.txt" || fail 'could not download checksums.txt'

expected=$(awk -v name="$archive" '{ file=$2; sub(/^\*/, "", file); if (file == name) { print $1; count++ } } END { if (count != 1) exit 1 }' "$temp_dir/checksums.txt") || fail "checksums.txt must contain exactly one entry for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$temp_dir/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$temp_dir/$archive" | awk '{print $1}')
else
  fail 'sha256sum or shasum is required to verify the release'
fi
[ "$actual" = "$expected" ] || fail "checksum mismatch for $archive"

entries=$(tar -tzf "$temp_dir/$archive") || fail 'release archive is malformed'
[ "$entries" = 'happy-memory' ] || fail 'release archive must contain only happy-memory'
tar -xzf "$temp_dir/$archive" -C "$temp_dir" happy-memory || fail 'could not extract happy-memory'
chmod 755 "$temp_dir/happy-memory"
"$temp_dir/happy-memory" version >/dev/null 2>&1 || fail 'downloaded executable failed validation'

mkdir -p "$bin_dir" || fail "could not create $bin_dir"
[ -d "$bin_dir" ] && [ -w "$bin_dir" ] || fail "installation directory is not writable: $bin_dir"
target=$bin_dir/happy-memory
stage=$bin_dir/.happy-memory.new.$$
mv "$temp_dir/happy-memory" "$stage" || fail 'could not stage the new executable'
"$stage" version >/dev/null 2>&1 || fail 'staged executable failed validation'
if [ -e "$target" ]; then
  backup=$bin_dir/.happy-memory.backup.$$
  mv "$target" "$backup" || fail 'could not back up the existing executable'
fi
installed=1
mv "$stage" "$target" || fail 'could not install the new executable'
"$target" version >/dev/null 2>&1 || fail 'installed executable failed validation'
stage=
[ -z "$backup" ] || rm -f "$backup"
backup=
installed=0

printf 'Installed happy-memory %s at %s\n' "$version" "$target"
case :${PATH:-}: in
  *:"$bin_dir":*) ;;
  *) printf 'Add %s to PATH, for example: export PATH="%s:%s"\n' "$bin_dir" "$bin_dir" "\$PATH" ;;
esac
