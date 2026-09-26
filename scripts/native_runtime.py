"""Development-side installer support for the native runtime release artifacts.

The portable lock owns the full release descriptor. Every consumer receives all
six executables and their manifest/licenses as versioned, portable artifacts.
"""
from __future__ import annotations

import hashlib
import json
import os
import platform
import re
import shutil
import tempfile
from pathlib import Path

TARGETS = {f"{system}/{arch}" for system in ("darwin", "linux", "windows") for arch in ("arm64", "amd64")}


def target() -> str:
    system = platform.system().lower()
    machine = platform.machine().lower()
    arch = {"aarch64": "arm64", "arm64": "arm64", "amd64": "amd64", "x86_64": "amd64"}.get(machine, machine)
    selected = f"{system}/{arch}"
    if selected not in TARGETS:
        raise RuntimeError(f"unsupported native runtime platform: {selected}")
    return selected


def filename(selected: str) -> str:
    return "kos-" + selected.replace("/", "-") + (".exe" if selected.startswith("windows/") else "")


def local_paths(selected: str) -> tuple[str, str]:
    return (".agents/bin/" + filename(selected), ".agents/bin/THIRD_PARTY_NOTICES.txt")


def manifest_bytes(current: dict) -> bytes:
    return (json.dumps(current, indent=2, sort_keys=True) + "\n").encode("utf-8")


def bundle_hashes(current: dict) -> dict[str, str]:
    return {
        **{".agents/bin/" + item["file"]: item["sha256"] for item in current["artifacts"].values()},
        ".agents/bin/THIRD_PARTY_NOTICES.txt": current["notices_sha256"],
        ".agents/bin/runtime-manifest.json": hashlib.sha256(manifest_bytes(current)).hexdigest(),
    }


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def source_fingerprint(dist: Path) -> str:
    """Hash local build inputs deterministically, independent of mtime and host."""
    files = [dist / "go.mod", dist / "go.sum"]
    for directory in ("cmd", "internal"):
        base = dist / directory
        if not base.is_dir() or base.is_symlink():
            raise RuntimeError("native source fingerprint requires cmd/ and internal/")
        for path in base.rglob("*"):
            if path.is_symlink():
                raise RuntimeError("native build input must not be a symlink")
            if path.is_file() and not path.name.endswith("_test.go"):
                files.append(path)
    digest = hashlib.sha256()
    for path in sorted(files, key=lambda p: p.relative_to(dist).as_posix()):
        if path.is_symlink() or not path.is_file():
            raise RuntimeError("native build input is missing or unsafe")
        digest.update(path.relative_to(dist).as_posix().encode("utf-8") + b"\0")
        digest.update(sha256(path).encode("ascii") + b"\n")
    return digest.hexdigest()


def descriptor(value: object) -> dict:
    if (not isinstance(value, dict) or set(value) != {"schema", "version", "source_revision", "source_dirty", "source_fingerprint", "artifacts", "notices_sha256"}
            or value.get("schema") != 1 or not isinstance(value["version"], str) or not value["version"]
            or not isinstance(value["source_revision"], str) or not value["source_revision"]
            or not isinstance(value["source_dirty"], bool) or not isinstance(value["artifacts"], dict)
            or set(value["artifacts"]) != TARGETS):
        raise RuntimeError("native release requires exactly all six supported platform artifacts")
    hashes = [value["notices_sha256"], value["source_fingerprint"]]
    for selected, item in value["artifacts"].items():
        if not isinstance(item, dict) or set(item) != {"file", "sha256"} or item["file"] != filename(selected):
            raise RuntimeError("invalid native runtime artifact name")
        hashes.append(item["sha256"])
    if any(not isinstance(d, str) or not re.fullmatch(r"[0-9a-f]{64}", d) for d in hashes):
        raise RuntimeError("invalid native runtime artifact checksum")
    return value


def release(dist: Path) -> dict:
    path = dist / "dist/runtime-manifest.json"
    if not path.is_file() or path.is_symlink():
        raise RuntimeError("native runtime release missing: run make release before installation")
    value = descriptor(json.loads(path.read_text(encoding="utf-8")))
    if value["version"] != (dist / "VERSION").read_text(encoding="utf-8").strip():
        raise RuntimeError("native runtime release version differs from distribution")
    if source_fingerprint(dist) != value["source_fingerprint"]:
        raise RuntimeError("native runtime release is stale for current source: rebuild with make release")
    for item in [*value["artifacts"].values(), {"file": "THIRD_PARTY_NOTICES.txt", "sha256": value["notices_sha256"]}]:
        artifact = dist / "dist" / item["file"]
        if artifact.is_symlink() or not artifact.is_file() or sha256(artifact) != item["sha256"]:
            raise RuntimeError(f"native runtime release artifact is missing or changed: {item['file']}")
    return value


def preflight(dest: Path, current: dict) -> None:
    for relative in bundle_hashes(current):
        path = dest / relative
        parent = dest
        for part in Path(relative).parts[:-1]:
            parent /= part
            if parent.is_symlink() or (parent.exists() and not parent.is_dir()):
                raise RuntimeError(f"unsafe native runtime local path: {relative}")
        if path.is_symlink() or (path.exists() and not path.is_file()):
            raise RuntimeError(f"unsafe native runtime local path: {relative}")


def conflicts(dest: Path, current: dict, previous: dict | None) -> list[str]:
    preflight(dest, current)
    if previous is not None:
        previous = descriptor(previous)
    expected = bundle_hashes(previous or current)
    return [relative for relative, digest in expected.items() if (dest / relative).exists() and sha256(dest / relative) != digest]


def install(dist: Path, dest: Path, current: dict) -> None:
    preflight(dest, current)
    for relative in bundle_hashes(current):
        output = dest / relative
        output.parent.mkdir(parents=True, exist_ok=True)
        fd, name = tempfile.mkstemp(prefix=".install-", dir=output.parent)
        try:
            with os.fdopen(fd, "wb") as writer:
                if output.name == "runtime-manifest.json":
                    writer.write(manifest_bytes(current))
                else:
                    with (dist / "dist" / output.name).open("rb") as reader:
                        shutil.copyfileobj(reader, writer)
                writer.flush()
                os.fsync(writer.fileno())
            os.chmod(name, 0o755 if output.name.startswith("kos-") else 0o644)
            os.replace(name, output)
        finally:
            if os.path.exists(name):
                os.unlink(name)


def status(dest: Path, current: dict, installed: dict | None) -> dict:
    if installed is None:
        return {"status": "missing-release-lock"}
    try:
        installed = descriptor(installed)
        preflight(dest, installed)
        selected = target()
        paths = local_paths(selected)
        drift = [relative for relative, digest in bundle_hashes(installed).items() if not (dest / relative).is_file() or sha256(dest / relative) != digest]
        if os.name != "nt":
            for artifact in installed["artifacts"].values():
                relative = ".agents/bin/" + artifact["file"]
                if (dest / relative).is_file() and not os.access(dest / relative, os.X_OK):
                    drift.append(relative + ":not-executable")
        return {"status": "drift" if drift else "ready", "platform": selected, "path": paths[0], "drift": drift, "release_matches_dist": current == installed}
    except (RuntimeError, ValueError, OSError) as error:
        return {"status": "invalid", "error": str(error)}
