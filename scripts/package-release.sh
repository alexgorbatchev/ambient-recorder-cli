#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
release_version=${RECORDER_VERSION:?Set RECORDER_VERSION to X.Y.Z}
case "$release_version" in ''|.*|*.|*..*|*[!0-9.]*) printf 'Release version must be X.Y.Z\n' >&2; exit 1 ;; esac
IFS=.
set -- $release_version
unset IFS
test "$#" -eq 3
for part do
    case "$part" in ''|0?*) printf 'Invalid release version component\n' >&2; exit 1 ;; esac
done
case "$(uname -m)" in
    arm64) release_arch=arm64 ;;
    x86_64) release_arch=amd64 ;;
    *) printf 'Unsupported release architecture\n' >&2; exit 1 ;;
esac
sh scripts/verify-binary.sh bin/ambient-recorder "$release_version"
mkdir -p .tmp dist
staging=$(mktemp -d "$PWD/.tmp/release-$release_version-$release_arch.XXXXXX")
mkdir -p "$staging/archive/docs" "$staging/extracted"
cp bin/ambient-recorder LICENSE THIRD_PARTY_NOTICES.md config.example.toml go.mod go.sum "$staging/archive/"
cp cmd/ambient-recorder/SKILL.md "$staging/archive/"
cp -R docs/licenses "$staging/archive/docs/"
archive="$PWD/dist/ambient-recorder_${release_version}_darwin_${release_arch}.tar.gz"
COPYFILE_DISABLE=1 tar -czf "$archive" -C "$staging/archive" ambient-recorder LICENSE THIRD_PARTY_NOTICES.md config.example.toml go.mod go.sum SKILL.md docs/licenses
tar -tzf "$archive" > "$staging/contents"
tar -xzf "$archive" -C "$staging/extracted"
cmp bin/ambient-recorder "$staging/extracted/ambient-recorder"
for file in LICENSE THIRD_PARTY_NOTICES.md config.example.toml go.mod go.sum; do
    cmp "$file" "$staging/extracted/$file"
done
cmp cmd/ambient-recorder/SKILL.md "$staging/extracted/SKILL.md"
for license in docs/licenses/*.txt; do
    cmp "$license" "$staging/extracted/$license"
done
sh scripts/verify-binary.sh "$staging/extracted/ambient-recorder" "$release_version"
shasum -a 256 "$archive"
printf 'Packaged %s; retained staging: %s\n' "$archive" "$staging"
