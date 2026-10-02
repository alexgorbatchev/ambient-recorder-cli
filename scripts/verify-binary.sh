#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
binary=$1
expected_version=$2
case "$binary" in /*) ;; *) binary="$PWD/$binary" ;; esac
case "$(uname -m)" in
    arm64) expected_arch=arm64 ;;
    x86_64) expected_arch=x86_64 ;;
    *) printf 'Unsupported verification architecture\n' >&2; exit 1 ;;
esac
mkdir -p .tmp
verification=$(mktemp -d "$PWD/.tmp/binary-verification.XXXXXX")
test -x "$binary"
test "$(lipo -archs "$binary")" = "$expected_arch"
codesign --verify --verbose "$binary"
printf '%s\n' "$expected_version" > "$verification/expected-version"
for mode in 0 1; do
    AGENT=$mode "$binary" --version > "$verification/version-$mode"
    cmp "$verification/expected-version" "$verification/version-$mode"
    # Run the extracted executable away from source and with module access disabled.
    (cd "$verification"; AGENT=$mode GOPROXY=off "$binary" skill) > "$verification/skill-$mode"
    cmp cmd/ambient-recorder/SKILL.md "$verification/skill-$mode"
done
AGENT=1 "$binary" recording start --help > "$verification/agent-help"
IFS= read -r first_line < "$verification/agent-help"
test "$first_line" = 'ALERT: Agents must read `AGENT=1 ambient-recorder skill` before using this tool.'
otool -L "$binary" > "$verification/libraries"
awk 'NR > 1 && $1 !~ /^\/System\/Library\/Frameworks\// && $1 !~ /^\/usr\/lib\// { bad=1; print "Unexpected dynamic dependency: " $1 > "/dev/stderr" } END { exit bad }' "$verification/libraries"
otool -P "$binary" > "$verification/plist-section"
sed -n '/^<?xml/,$p' "$verification/plist-section" > "$verification/Info.plist"
plutil -lint "$verification/Info.plist"
test "$(plutil -extract CFBundleIdentifier raw "$verification/Info.plist")" = com.alexgorbatchev.ambient-recorder
test -n "$(plutil -extract NSMicrophoneUsageDescription raw -expect string "$verification/Info.plist")"
test -n "$(plutil -extract NSAudioCaptureUsageDescription raw -expect string "$verification/Info.plist")"
printf 'Verified %s version %s (%s); evidence: %s\n' "$binary" "$expected_version" "$expected_arch" "$verification"
