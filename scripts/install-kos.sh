#!/bin/sh
# Install or update kos, the knowledge OS CLI.
#
#   curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh
#   sh scripts/install-kos.sh --local     # build from a distribution checkout (requires Go)
#
# The release is downloaded with curl; when that fails (a private fork in KOS_REPO), the GitHub CLI
# and your login read it instead.
#
# Environment:
#   KOS_REPO          owner/repo that publishes releases (default rendis/knowledge-os)
#   KOS_VERSION       version without "v" (default: latest)
#   KOS_INSTALL_DIR   destination directory (default: ~/.local/bin)
#   KOS_DOWNLOAD_URL  base URL holding the release assets (overrides REPO and VERSION)

set -eu

REPO="${KOS_REPO:-rendis/knowledge-os}"
VERSION="${KOS_VERSION:-latest}"
INSTALL_DIR="${KOS_INSTALL_DIR:-$HOME/.local/bin}"
WORK_DIR=""

say() { printf 'kos: %s\n' "$*"; }
fail() { printf 'kos: %s\n' "$*" >&2; exit 1; }
cleanup() { [ -z "$WORK_DIR" ] || rm -rf "$WORK_DIR"; }
trap cleanup EXIT

platform() {
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) fail "unsupported system $(uname -s): use macOS, Linux or WSL (Windows: install-kos.ps1)" ;;
  esac
  case "$(uname -m)" in
    arm64|aarch64) arch=arm64 ;;
    x86_64|amd64) arch=amd64 ;;
    *) fail "unsupported architecture $(uname -m)" ;;
  esac
  printf 'kos-%s-%s' "$os" "$arch"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | awk '{print $1}'
  else fail "sha256sum or shasum is required to verify the download"
  fi
}

install_binary() {
  mkdir -p "$INSTALL_DIR"
  cp "$1" "$INSTALL_DIR/.kos.tmp.$$"
  chmod 0755 "$INSTALL_DIR/.kos.tmp.$$"
  mv -f "$INSTALL_DIR/.kos.tmp.$$" "$INSTALL_DIR/kos"
  say "installed kos $("$INSTALL_DIR/kos" version | awk -F'"' '/"kos"/ {print $4}') at $INSTALL_DIR/kos"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) say "add $INSTALL_DIR to your PATH, for example: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
  esac
}

install_release() {
  asset="$(platform)"
  WORK_DIR="$(mktemp -d)"
  if [ -n "${KOS_DOWNLOAD_URL:-}" ]; then
    base="${KOS_DOWNLOAD_URL%/}"
    say "downloading $asset from $base"
    curl -fsSL --retry 3 -o "$WORK_DIR/$asset" "$base/$asset" || fail "download failed: $base/$asset"
    curl -fsSL --retry 3 -o "$WORK_DIR/SHA256SUMS" "$base/SHA256SUMS" || fail "download failed: $base/SHA256SUMS"
  else
    if [ "$VERSION" = "latest" ]; then base="https://github.com/$REPO/releases/latest/download"
    else base="https://github.com/$REPO/releases/download/v$VERSION"
    fi
    say "downloading $asset from $base"
    if ! { command -v curl >/dev/null 2>&1 \
        && curl -fsSL --retry 3 -o "$WORK_DIR/$asset" "$base/$asset" \
        && curl -fsSL --retry 3 -o "$WORK_DIR/SHA256SUMS" "$base/SHA256SUMS"; }; then
      command -v gh >/dev/null 2>&1 || fail "download failed: $base/$asset (a private repository needs the GitHub CLI)"
      tag=""
      [ "$VERSION" = "latest" ] || tag="v$VERSION"
      say "retrying with the GitHub CLI"
      # shellcheck disable=SC2086
      gh release download $tag --repo "$REPO" --pattern "$asset" --pattern SHA256SUMS --dir "$WORK_DIR" --clobber \
        || fail "gh could not read $REPO: log in with an account that can read it (gh auth login / gh auth switch)"
    fi
  fi
  expected="$(awk -v name="$asset" '$2 == name {print $1}' "$WORK_DIR/SHA256SUMS")"
  [ -n "$expected" ] || fail "SHA256SUMS has no entry for $asset"
  actual="$(sha256_of "$WORK_DIR/$asset")"
  [ "$expected" = "$actual" ] || fail "checksum mismatch for $asset: refused"
  install_binary "$WORK_DIR/$asset"
}

install_local() {
  root="$(cd "$(dirname "$0")/.." && pwd -P)"
  [ -f "$root/payload.go" ] || fail "--local runs from a knowledge-os checkout"
  command -v go >/dev/null 2>&1 || fail "Go is required for --local"
  WORK_DIR="$(mktemp -d)"
  (cd "$root" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(cat kernel/VERSION)" -o "$WORK_DIR/kos" ./cmd/kos)
  install_binary "$WORK_DIR/kos"
}

case "${1:-}" in
  "") install_release ;;
  --local) install_local ;;
  *) fail "usage: install-kos.sh [--local]" ;;
esac
