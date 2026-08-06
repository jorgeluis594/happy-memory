#!/bin/sh

set -eu

dist=${1:-dist}
[ -d "$dist" ] || { echo "missing release directory: $dist" >&2; exit 1; }
[ -f "$dist/checksums.txt" ] || { echo 'missing checksums.txt' >&2; exit 1; }

archives=$(find "$dist" -maxdepth 1 -type f \( -name 'happy-memory_*_linux_*.tar.gz' -o -name 'happy-memory_*_darwin_*.tar.gz' -o -name 'happy-memory_*_windows_*.zip' \) | sort)
count=$(printf '%s\n' "$archives" | grep -c .)
[ "$count" -eq 6 ] || { echo "expected 6 archives, found $count" >&2; exit 1; }

for os in linux darwin windows; do
  for arch in amd64 arm64; do
    case $os in
      windows) pattern="happy-memory_*_${os}_${arch}.zip"; expected=happy-memory.exe ;;
      *) pattern="happy-memory_*_${os}_${arch}.tar.gz"; expected=happy-memory ;;
    esac
    # The intentional expansion verifies that exactly one archive matches.
    # shellcheck disable=SC2086
    set -- "$dist"/$pattern
    [ "$#" -eq 1 ] && [ -f "$1" ] || { echo "missing or duplicate artifact for $os/$arch" >&2; exit 1; }
    archive=$1
    name=${archive##*/}
    entries=$(awk -v artifact="$name" '{ file=$2; sub(/^\*/, "", file); if (file == artifact) count++ } END { print count+0 }' "$dist/checksums.txt")
    [ "$entries" -eq 1 ] || { echo "expected one checksum for $name" >&2; exit 1; }
    case $archive in
      *.zip) contents=$(unzip -Z1 "$archive") ;;
      *) contents=$(tar -tzf "$archive") ;;
    esac
    [ "$contents" = "$expected" ] || { echo "$name contains unexpected files: $contents" >&2; exit 1; }
  done
done

while IFS= read -r archive; do
  name=${archive##*/}
  expected=$(awk -v artifact="$name" '{ file=$2; sub(/^\*/, "", file); if (file == artifact) print $1 }' "$dist/checksums.txt")
  actual=$(shasum -a 256 "$archive" | awk '{print $1}')
  [ "$actual" = "$expected" ] || { echo "checksum mismatch: $name" >&2; exit 1; }
done <<EOF
$archives
EOF

echo 'release snapshot contains six verified single-binary archives'
