#!/bin/sh
set -eu

cd "$(dirname "$0")/.."
project_dir=$(pwd)
native_dir="$project_dir/.tmp/native"
build_id="opus-1.6.1_libopusenc-0.3_$(uname -m)_macos14.2"
if test -f "$native_dir/build-id" && test "$(cat "$native_dir/build-id")" = "$build_id" && test -f "$native_dir/lib/libopus.a" && test -f "$native_dir/lib/libopusenc.a"; then
    exit 0
fi

mkdir -p .tmp/downloads "$native_dir"
fetch() {
    archive=$1
    checksum=$2
    if ! test -f ".tmp/downloads/$archive"; then
        curl --fail --location --output ".tmp/downloads/$archive.part" "https://downloads.xiph.org/releases/opus/$archive"
        mv ".tmp/downloads/$archive.part" ".tmp/downloads/$archive"
    fi
    actual=$(shasum -a 256 ".tmp/downloads/$archive")
    if test "${actual%% *}" != "$checksum"; then
        printf 'Checksum mismatch: %s\n' "$archive" >&2
        exit 1
    fi
    tar -xzf ".tmp/downloads/$archive" -C .tmp
}

fetch opus-1.6.1.tar.gz 6ffcb593207be92584df15b32466ed64bbec99109f007c82205f0194572411a1
fetch libopusenc-0.3.tar.gz f616d3aff9b2034547894ccb8ab56c36cf1a4acb0d922c5d7119f97bbe58642c

(
    cd .tmp/opus-1.6.1
    CFLAGS='-O2 -mmacosx-version-min=14.2' ./configure --prefix="$native_dir" --disable-shared --enable-static --disable-doc --disable-extra-programs
    make -j4
    make install
)
(
    cd .tmp/libopusenc-0.3
    CFLAGS='-O2 -mmacosx-version-min=14.2' DEPS_CFLAGS="-I$native_dir/include/opus" DEPS_LIBS="-L$native_dir/lib -lopus -lm" ./configure --prefix="$native_dir" --disable-shared --enable-static --disable-doc --disable-examples
    make clean
    make -j4
    make install
)
clang scripts/native-check.c -I"$native_dir/include/opus" "$native_dir/lib/libopusenc.a" "$native_dir/lib/libopus.a" -lm -o .tmp/native-check
.tmp/native-check
printf '%s\n' "$build_id" > "$native_dir/build-id"
