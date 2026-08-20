#!/usr/bin/env python3
"""Prepare direction-safe Jira Blocks payloads and read-back expectations."""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections.abc import Sequence


ISSUE_KEY = re.compile(r"^[A-Z][A-Z0-9_]*-[1-9][0-9]*$")
EXPECTED_TYPE_NAME = "Blocks"
EXPECTED_OUTWARD_DESCRIPTION = "blocks"
EXPECTED_INWARD_DESCRIPTION = "is blocked by"


class PlanError(ValueError):
    """Raised when a Blocks link plan is unsafe or ambiguous."""


def _validate_issue_key(value: str, role: str) -> str:
    key = value.strip()
    if not ISSUE_KEY.fullmatch(key):
        raise PlanError(f"{role} must be an exact Jira key, got {value!r}")
    return key


def build_plan(
    edges: Sequence[Sequence[str]],
    *,
    type_name: str,
    outward_description: str,
    inward_description: str,
) -> dict[str, object]:
    """Return transport payloads and two-sided assertions for directed edges."""

    if type_name != EXPECTED_TYPE_NAME:
        raise PlanError(
            f"expected link type {EXPECTED_TYPE_NAME!r}, got {type_name!r}"
        )
    if outward_description != EXPECTED_OUTWARD_DESCRIPTION:
        raise PlanError(
            "live outward description does not match the canonical Blocks contract: "
            f"expected {EXPECTED_OUTWARD_DESCRIPTION!r}, got {outward_description!r}"
        )
    if inward_description != EXPECTED_INWARD_DESCRIPTION:
        raise PlanError(
            "live inward description does not match the canonical Blocks contract: "
            f"expected {EXPECTED_INWARD_DESCRIPTION!r}, got {inward_description!r}"
        )
    if not edges:
        raise PlanError("at least one directed blocker/blocked edge is required")

    seen: set[tuple[str, str]] = set()
    links: list[dict[str, object]] = []

    for raw_edge in edges:
        if len(raw_edge) != 2:
            raise PlanError("each edge must contain exactly BLOCKER and BLOCKED")
        blocker = _validate_issue_key(raw_edge[0], "blocker")
        blocked = _validate_issue_key(raw_edge[1], "blocked")
        edge = (blocker, blocked)

        if blocker == blocked:
            raise PlanError(f"self-blocking edge is not allowed: {blocker}")
        if edge in seen:
            raise PlanError(f"duplicate directed edge: {blocker} blocks {blocked}")
        if (blocked, blocker) in seen:
            raise PlanError(
                "contradictory directed edges in one plan: "
                f"{blocker} blocks {blocked} and {blocked} blocks {blocker}"
            )

        seen.add(edge)
        links.append(
            {
                "blocker": blocker,
                "blocked": blocked,
                "transport": {
                    "type": type_name,
                    "outwardIssue": blocker,
                    "inwardIssue": blocked,
                },
                "standaloneLinkReadBack": {
                    "outwardIssue": blocker,
                    "inwardIssue": blocked,
                },
                "readBackAssertions": [
                    {
                        "issue": blocker,
                        "relationship": outward_description,
                        "otherIssue": blocked,
                        "issueScopedCounterpartField": "inwardIssue",
                        "sentence": f"{blocker} {outward_description} {blocked}",
                    },
                    {
                        "issue": blocked,
                        "relationship": inward_description,
                        "otherIssue": blocker,
                        "issueScopedCounterpartField": "outwardIssue",
                        "sentence": f"{blocked} {inward_description} {blocker}",
                    },
                ],
            }
        )

    return {
        "status": "valid",
        "linkType": {
            "name": type_name,
            "outward": outward_description,
            "inward": inward_description,
        },
        "issueScopedReadBackRule": (
            "In an issue-scoped issuelinks response, the field names the other "
            "endpoint: the blocker entry contains inwardIssue=BLOCKED and the "
            "blocked entry contains outwardIssue=BLOCKER."
        ),
        "edgeCount": len(links),
        "links": links,
    }


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Translate semantic Jira dependencies BLOCKER -> BLOCKED into "
            "direction-safe Blocks payloads and two-sided read-back assertions."
        )
    )
    parser.add_argument(
        "--type-name",
        required=True,
        help="Current Jira link type name obtained from a live metadata read.",
    )
    parser.add_argument(
        "--outward-description",
        required=True,
        help="Current outward description obtained from a live metadata read.",
    )
    parser.add_argument(
        "--inward-description",
        required=True,
        help="Current inward description obtained from a live metadata read.",
    )
    parser.add_argument(
        "--edge",
        action="append",
        nargs=2,
        metavar=("BLOCKER", "BLOCKED"),
        required=True,
        help="Directed dependency; repeat once per required Blocks link.",
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        plan = build_plan(
            args.edge,
            type_name=args.type_name,
            outward_description=args.outward_description,
            inward_description=args.inward_description,
        )
    except PlanError as exc:
        print(
            json.dumps(
                {"status": "blocked", "reason": str(exc)},
                ensure_ascii=False,
                sort_keys=True,
            ),
            file=sys.stderr,
        )
        return 2

    print(json.dumps(plan, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
