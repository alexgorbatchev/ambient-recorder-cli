#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
case "$(uname -m)" in
    arm64)
        target=aarch64-apple-darwin
        checksum=50ae3e996c974a0bf32ea7d10f495070df33f1b43e0616b2769e3d4821ed8f48
        ;;
    x86_64)
        target=x86_64-apple-darwin
        checksum=9a09cfef66aaa79da58203970103a0684307716caaabd3e9844cacc4dc0f4023
        ;;
    *) printf 'Unsupported CI architecture\n' >&2; exit 1 ;;
esac
mkdir -p .tmp/ci-just
archive=".tmp/ci-just/just-1.58.0-$target.tar.gz"
curl --fail --location --retry 3 --output "$archive" "https://github.com/casey/just/releases/download/1.58.0/just-1.58.0-$target.tar.gz"
actual=$(shasum -a 256 "$archive")
test "${actual%% *}" = "$checksum"
tar -xzf "$archive" -C .tmp/ci-just just
test "$(.tmp/ci-just/just --version)" = 'just 1.58.0'
printf '%s/.tmp/ci-just\n' "$PWD" >> "${GITHUB_PATH:?GITHUB_PATH is required on CI}"
