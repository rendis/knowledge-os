"""Locate and run the released `kos` binary; shared by the bootstrap evals.

Per ADR 0002 the checkout installer is retired: a cell is created, adopted and updated by the `kos`
binary alone. These evals drive `dist/kos-<os>-<arch>`, built by `make release` (see `tools/release`),
instead of a Python installer. Cells receive no Python and no binaries; `kos` is installed once per
machine (`scripts/install-kos.sh`).
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
    """dist/release.json, written by `make release`; refused when the source changed since, so the
    evals never drive a kos whose kernel is not the one in this tree."""
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
    """The release's kos for this host, verified against the checksum release.json records for it."""
    value = release(dist)
    selected = target()
    item = (value.get("artifacts") or {}).get(selected) or {}
    path = dist / "dist" / str(item.get("file", ""))
    if not item or path.is_symlink() or not path.is_file() or sha256(path) != item.get("sha256"):
        raise RuntimeError(f"kos release artifact for {selected} is missing or changed: run make release")
    return path


def run(dist: Path, *args: str, cwd: Path | None = None, input: str | None = None,
        env: dict[str, str] | None = None, timeout: float | None = 60) -> subprocess.CompletedProcess[str]:
    """Run the released kos, with the daily update check disabled so tests are deterministic."""
    merged = {**os.environ, "KOS_NO_UPDATE_CHECK": "1"}
    if env:
        merged.update(env)
    return subprocess.run([str(binary(dist)), *args], cwd=str(cwd) if cwd else None, input=input,
                          text=True, capture_output=True, env=merged, timeout=timeout, check=False)
