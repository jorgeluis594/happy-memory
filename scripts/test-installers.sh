#!/bin/sh

set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/happy-memory-installer-test.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
release=$work/releases/download/v1.2.3
mkdir -p "$release" "$work/mock-bin"

go build -trimpath -ldflags '-X github.com/jorgeluis594/happy-memory/internal/buildinfo.version=1.2.3 -X github.com/jorgeluis594/happy-memory/internal/buildinfo.commit=testcommit -X github.com/jorgeluis594/happy-memory/internal/buildinfo.buildDate=2026-08-06T00:00:00Z' -o "$work/happy-memory" ./cmd/happy-memory
tar -czf "$release/happy-memory_1.2.3_darwin_arm64.tar.gz" -C "$work" happy-memory
cp "$release/happy-memory_1.2.3_darwin_arm64.tar.gz" "$release/happy-memory_1.2.3_linux_amd64.tar.gz"
(
  cd "$release"
  shasum -a 256 happy-memory_1.2.3_darwin_arm64.tar.gz happy-memory_1.2.3_linux_amd64.tar.gz > checksums.txt
)
mkdir -p "$work/api/releases"
printf '%s\n' '{"tag_name":"v1.2.3"}' > "$work/api/releases/latest"

cat > "$work/mock-bin/curl" <<'EOF'
#!/bin/sh
destination=
url=
while [ "$#" -gt 0 ]; do
  case $1 in
    --output) destination=$2; shift 2 ;;
    -*) shift ;;
    *) url=$1; shift ;;
  esac
done
case $url in
  mock://*) source=${url#mock://} ;;
  *) exit 22 ;;
esac
cp "$source" "$destination"
EOF
chmod +x "$work/mock-bin/curl"

run_install() {
  PATH="$work/mock-bin:/usr/bin:/bin" \
    HOME="$work/home" \
    HAPPY_MEMORY_API_BASE="mock://$work/api" \
    HAPPY_MEMORY_DOWNLOAD_BASE="mock://$work/releases/download" \
    HAPPY_MEMORY_UNAME_S=Darwin \
    HAPPY_MEMORY_UNAME_M=arm64 \
    sh "$root/install.sh" "$@"
}

bin_dir="$work/path with spaces/bin"
run_install --bin-dir "$bin_dir"
result=$("$bin_dir/happy-memory" version)
[ "$result" = '{"ok":true,"data":{"version":"1.2.3","commit":"testcommit","build_date":"2026-08-06T00:00:00Z"}}' ]
run_install --version v1.2.3 --bin-dir "$bin_dir"
[ "$(find "$bin_dir" -type f | wc -l | tr -d ' ')" -eq 1 ]

if HAPPY_MEMORY_UNAME_S=Plan9 HAPPY_MEMORY_UNAME_M=amd64 sh "$root/install.sh" --version v1.2.3 --bin-dir "$work/nope" >/dev/null 2>&1; then
  echo 'unsupported platform unexpectedly succeeded' >&2
  exit 1
fi
if PATH="$work/mock-bin:/usr/bin:/bin" HOME="$work/home" HAPPY_MEMORY_DOWNLOAD_BASE='bad://missing' HAPPY_MEMORY_UNAME_S=Darwin HAPPY_MEMORY_UNAME_M=arm64 sh "$root/install.sh" --version v1.2.3 --bin-dir "$work/nope" >/dev/null 2>&1; then
  echo 'failed download unexpectedly succeeded' >&2
  exit 1
fi

cp "$release/checksums.txt" "$work/checksums.good"
printf '%064d  %s\n' 0 happy-memory_1.2.3_darwin_arm64.tar.gz > "$release/checksums.txt"
if run_install --version v1.2.3 --bin-dir "$bin_dir" >/dev/null 2>&1; then
  echo 'bad checksum unexpectedly succeeded' >&2
  exit 1
fi
[ "$("$bin_dir/happy-memory" version)" = "$result" ]
cp "$work/checksums.good" "$release/checksums.txt"

mkdir -p "$work/malicious"
cp "$work/happy-memory" "$work/malicious/happy-memory"
printf evil > "$work/malicious/extra"
tar -czf "$release/happy-memory_1.2.3_darwin_arm64.tar.gz" -C "$work/malicious" happy-memory extra
hash=$(shasum -a 256 "$release/happy-memory_1.2.3_darwin_arm64.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$hash" happy-memory_1.2.3_darwin_arm64.tar.gz > "$release/checksums.txt"
if run_install --version v1.2.3 --bin-dir "$bin_dir" >/dev/null 2>&1; then
  echo 'malicious archive unexpectedly succeeded' >&2
  exit 1
fi
[ "$("$bin_dir/happy-memory" version)" = "$result" ]

echo 'Unix installer fixtures passed'
