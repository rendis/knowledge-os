#!/usr/bin/env sh
# Record the README GIFs into docs/assets/, each from a fresh demo cell.
# Needs vhs and jq, and a current `make release`. Usage: record.sh [TAPE ...]
set -eu
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export DEMO="${DEMO:-${TMPDIR:-/tmp}/kos-demo}" BASH_SILENCE_DEPRECATION_WARNING=1
cd "$HERE"
[ $# -gt 0 ] || set -- onboard discover claims investigation handoff sync
for tape in "$@"; do
  sh ./setup.sh "$DEMO"
  vhs -q "$tape.tape"
  echo "recorded docs/assets/demo-$tape.gif"
done
