#!/usr/bin/env sh
# Knowledge OS installer. Safe with paths that contain spaces.
# Local v1: run from a clone of this repository.
# Future: curl -fsSL <raw-tag>/install.sh | sh -s -- init --dest <vault>
set -eu

SCRIPT_PATH=$0
if [ -n "${BASH_VERSION:-}" ]; then
  SCRIPT_PATH=$BASH_SOURCE
fi
# Resolve this script even when executed via sh install.sh
if command -v python3 >/dev/null 2>&1; then
  :
else
  echo "python3 is required" >&2
  exit 127
fi

ROOT=$(CDPATH= python3 -c 'import os, sys; print(os.path.dirname(os.path.realpath(sys.argv[1])))' "$SCRIPT_PATH")

# If this file is a curl-fetched stub without kernel/, clone a tagged cache here
# in a later release. v1 requires the full distribution next to this script.
if [ ! -d "$ROOT/kernel" ] || [ ! -f "$ROOT/VERSION" ]; then
  echo "This installer must run from a documentation-vault checkout (kernel/ missing)." >&2
  echo "Clone the distribution, then run: ./install.sh init --dest <vault>" >&2
  exit 2
fi

exec python3 -B "$ROOT/scripts/knowledge_os.py" "$@"
