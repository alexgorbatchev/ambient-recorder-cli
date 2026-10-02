#!/bin/sh
set -eu

evidence=$1
shift
otool -l "$@" > "$evidence"
awk '$1 == "minos" { found=1; if ($2 != "14.2") { bad=1; print "Unexpected minimum macOS: " $2 > "/dev/stderr" } } END { exit (bad || !found) }' "$evidence"
