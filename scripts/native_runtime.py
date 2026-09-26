"""Development-side access to the kos binary that `make release` builds.

The installer delegates every kernel installation to `kos kernel update`, so a vault is written by one
implementation. The release must match the current source: the kernel it embeds is the one installed.
"""
from __future__ import annotations

import hashlib
import json
import os
import platform
import subprocess
from pathlib import Path

TARGETS = {f"{system}/{arch}" for system in ("darwin", "linux", "windows") for arch in ("arm64", "amd64")}


def target() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()
    arch = {"aarch64": "arm64", "arm64": "arm64", "amd64": "amd64", "x86_64": "amd64"}.get(machine, machine)
    selected = f"{system}/{arch}"
    if selected not in TARGETS:
        raise RuntimeError(f"unsupported platform: {selected}")
    return selected


def filename(selected: str) -> str:
    return "kos-" + selected.replace("/", "-") + (".exe" if selected.startswith("windows/") else "")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def source_fingerprint(dist: Path) -> str:
    """The same inputs tools/release hashes: the Go sources and the payload they embed."""
    files = [dist / name for name in ("go.mod", "go.sum", "payload.go", "MANAGED_PATHS")]
    for directory in ("cmd", "internal", "kernel", "adapters"):
        base = dist / directory
        if not base.is_dir() or base.is_symlink():
            raise RuntimeError(f"release fingerprint requires {directory}/")
        for path in base.rglob("*"):
            if path.is_symlink():
                raise RuntimeError(f"build input must not be a symlink: {path}")
            if path.is_file() and not path.name.endswith("_test.go"):
                files.append(path)
    digest = hashlib.sha256()
    for path in sorted(files, key=lambda p: p.relative_to(dist).as_posix()):
        digest.update(path.relative_to(dist).as_posix().encode("utf-8") + b"\0")
        digest.update(sha256(path).encode("ascii") + b"\n")
    return digest.hexdigest()


def release(dist: Path) -> dict:
    path = dist / "dist" / "release.json"
    if not path.is_file() or path.is_symlink():
        raise RuntimeError("kos release missing: run make release")
    value = json.loads(path.read_text(encoding="utf-8"))
    if value.get("version") != (dist / "kernel" / "VERSION").read_text(encoding="utf-8").strip():
        raise RuntimeError("kos release version differs from the kernel: run make release")
    if value.get("source_fingerprint") != source_fingerprint(dist):
        raise RuntimeError("kos release is stale for the current source: run make release")
    return value


def binary(dist: Path) -> Path:
    """The release's kos for this host, verified against its checksum."""
    value = release(dist)
    selected = target()
    item = (value.get("artifacts") or {}).get(selected) or {}
    path = dist / "dist" / str(item.get("file", ""))
    if not item or path.is_symlink() or not path.is_file() or sha256(path) != item.get("sha256"):
        raise RuntimeError(f"kos release artifact for {selected} is missing or changed: run make release")
    return path


def kernel(dist: Path, dest: Path, *flags: str) -> tuple[int, dict]:
    """Run `kos kernel update` on dest and return its exit code and JSON result."""
    env = {**os.environ, "KOS_NO_UPDATE_CHECK": "1"}
    result = subprocess.run([str(binary(dist)), "kernel", "update", "--vault", str(dest), *flags],
                            capture_output=True, text=True, env=env, check=False)
    try:
        payload = json.loads(result.stdout) if result.stdout.strip() else {}
    except json.JSONDecodeError:
        payload = {}
    if result.returncode != 0 and not payload:
        payload = {"status": "error", "error": (result.stderr or result.stdout).strip()}
    return result.returncode, payload
