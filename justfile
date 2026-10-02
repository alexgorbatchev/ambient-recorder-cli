set dotenv-load := false

# Apply the deployment target to every CGO object, including runtime/cgo.
export MACOSX_DEPLOYMENT_TARGET := "14.2"
export CGO_CFLAGS := env("CGO_CFLAGS", "-O2 -g") + " -mmacosx-version-min=14.2"
export CGO_LDFLAGS := env("CGO_LDFLAGS", "-O2 -g") + " -mmacosx-version-min=14.2"
export TMPDIR := justfile_directory() + "/.tmp"

version := env("RECORDER_VERSION", "dev")
link_flags := "-X main.version=" + version + " -extldflags=-Wl,-sectcreate,__TEXT,__info_plist," + justfile_directory() + "/internal/capture/Info.plist"

default:
    @just --list

native:
    mkdir -p .tmp
    sh scripts/build-native.sh

build: native
    mkdir -p bin
    go build -work -trimpath -ldflags={{quote(link_flags)}} -o bin/ambient-recorder ./cmd/ambient-recorder

run *args: build
    bin/ambient-recorder {{args}}

run-ai *args: build
    AGENT=1 bin/ambient-recorder {{args}}

test: native
    mkdir -p .tmp
    go test -work -race ./...

vet: native
    go vet -work ./...

lint: vet
    go mod tidy -diff
    test -z "$(gofmt -l cmd internal)"

check: lint test build

# Build, verify, and package the native architecture. Set RECORDER_VERSION=X.Y.Z.
release: check
    codesign --force --sign - bin/ambient-recorder
    sh scripts/package-release.sh
