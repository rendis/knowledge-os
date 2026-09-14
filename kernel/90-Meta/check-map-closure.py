#!/usr/bin/env python3
"""Read-only check of current checkpoint and visible pending counts."""
import argparse
import json
import re
from pathlib import Path


COUNT = re.compile(
    r"(?:permanecen|remaining\s*:?)\s+(\d+)\s+"
    r"(?:verificaciones|verification items)\s+(?:en|in)\s+(\d+)\s+(?:notas|notes)",
    re.IGNORECASE,
)


def check(vault, checkpoint):
    data = json.loads(checkpoint.read_text(encoding="utf-8"))
    counts = [
        sum(bool(re.match(r"^\s*- \[ \]", line)) for line in note.read_text(encoding="utf-8").splitlines())
        for note in (vault / "20-Repos").rglob("*.md")
    ]
    expected = {
        "remaining_verification_items": sum(counts),
        "notes_with_verifications": sum(count > 0 for count in counts),
    }
    errors = []
    current = data.get("visible_coverage", {})
    for key, value in expected.items():
        if type(current.get(key)) is not int or current[key] != value:
            errors.append(f"visible_coverage.{key}: expected {value}")
    paths = current.get("paths", [])
    if not isinstance(paths, list) or len(paths) < 2 or "00-Home.md" not in paths:
        errors.append("visible_coverage.paths: Home and linked coverage note required")
        paths = []
    for name in paths:
        path = (vault / name).resolve()
        if not path.is_relative_to(vault.resolve()) or not path.is_file():
            errors.append(f"invalid summary path: {name}")
            continue
        matches = COUNT.findall(path.read_text(encoding="utf-8").replace("*", ""))
        target = (str(expected["remaining_verification_items"]), str(expected["notes_with_verifications"]))
        if matches != [target]:
            errors.append(f"summary count missing, ambiguous or stale: {name}")
    return {"status": "blocked" if errors else "pass", "observed": expected, "errors": errors}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault", required=True, type=Path)
    parser.add_argument("--checkpoint", required=True, type=Path)
    args = parser.parse_args()
    try:
        result = check(args.vault, args.checkpoint)
    except (OSError, ValueError, TypeError) as error:
        result = {"status": "blocked", "errors": [str(error)]}
    print(json.dumps(result, ensure_ascii=False))
    return 0 if result["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
