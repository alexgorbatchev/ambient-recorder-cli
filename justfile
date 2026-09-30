set dotenv-load := false

default:
    @just --list

native:
    sh scripts/build-native.sh

build: native
    mkdir -p bin
    go build -ldflags='-extldflags=-Wl,-sectcreate,__TEXT,__info_plist,{{justfile_directory()}}/internal/capture/Info.plist' -o bin/ambient-recorder ./cmd/ambient-recorder

run *args: build
    bin/ambient-recorder {{args}}

run-ai *args: build
    AGENT=1 bin/ambient-recorder {{args}}

test: native
    mkdir -p .tmp
    TMPDIR="{{justfile_directory()}}/.tmp" go test -race ./...

vet: native
    go vet ./...

lint: vet
    go mod tidy -diff
    test -z "$(gofmt -l cmd internal)"

check: lint test build
