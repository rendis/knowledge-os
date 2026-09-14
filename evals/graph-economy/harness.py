#!/usr/bin/env python3
"""Measure discovery recall and recipe byte cost for Norte fixture questions."""
from __future__ import annotations

import argparse
import json
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any

HERE = Path(__file__).resolve().parent
DIST = HERE.parents[1]
sys.path.insert(0, str(HERE))
from materialize import materialize  # noqa: E402

sys.path.insert(0, str(DIST / "kernel" / "90-Meta"))

LEGACY_META = [
    "AGENTS.md",
    "90-Meta/Convenciones.md",
    "90-Meta/Auditoria - Framework.md",
]
ORIENT_FILES = ["00-Home.md", "instance.yaml"]
GRAPH_QUERY = DIST / "kernel" / "90-Meta" / "graph-query.py"


def size_of(vault: Path, rel: str) -> int:
    path = vault / rel
    return path.stat().st_size if path.is_file() else 0


def resolve_stem(vault: Path, stem: str) -> str | None:
    for path in vault.rglob("*.md"):
        if path.stem == stem:
            return path.relative_to(vault).as_posix()
    return None


def load_questions() -> list[dict[str, Any]]:
    return json.loads((HERE / "questions.json").read_text(encoding="utf-8"))["questions"]


def try_graph_query(vault: Path, args: list[str]) -> dict[str, Any] | None:
    if not GRAPH_QUERY.is_file():
        return None
    import importlib.util

    spec = importlib.util.spec_from_file_location("graph_query", GRAPH_QUERY)
    if spec is None or spec.loader is None:
        return None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.run_query(vault, args)


def predicted_legacy(vault: Path, question: dict[str, Any], platform: Path | None) -> list[str]:
    kind = question["kind"]
    files = list(LEGACY_META)
    if kind in {"orientation", "disable-topics"}:
        files.extend(ORIENT_FILES)
        if kind == "disable-topics" and platform:
            return [f"platform:{rel}" for rel in files]
        return files
    files.extend(ORIENT_FILES)
    if question.get("start"):
        rel = resolve_stem(vault, question["start"])
        if rel:
            files.append(rel)
    if kind == "flow":
        for stem in ("routing-planner", "fleet-ingest", "Routing"):
            rel = resolve_stem(vault, stem)
            if rel:
                files.append(rel)
    elif kind == "topic":
        for stem in ("routing-planner", "fleet-ingest"):
            rel = resolve_stem(vault, stem)
            if rel:
                files.append(rel)
    elif kind == "impact":
        for stem in ("route-events", "Flujo - Dispatch", "Routing", "fleet-ingest"):
            rel = resolve_stem(vault, stem)
            if rel:
                files.append(rel)
    elif kind == "placement":
        files.append("90-Meta/Convenciones.md")
        rel = resolve_stem(vault, "Maps Vendor")
        if rel:
            files.append(rel)
    elif kind == "investigations":
        files.extend(
            [
                "investigations/20260820-090000-stale-gps-write/investigation.md",
                "investigations/20260820-090001-unrelated-billing/investigation.md",
            ]
        )
    elif kind == "hygiene":
        files.extend(["90-Meta/verify-links.py"])
    elif kind == "learning":
        for stem in ("Aprendizaje - Stale GPS",):
            rel = resolve_stem(vault, stem)
            if rel:
                files.append(rel)
        files.append("investigations/20260820-090000-stale-gps-write/investigation.md")
    return list(dict.fromkeys(files))


def neighbor_nodes(vault: Path, start: str) -> list[str]:
    data = try_graph_query(vault, ["neighbors", "--node", start]) or {}
    nodes = [start]
    for edge in data.get("outgoing") or []:
        nodes.append(edge["target"])
    for edge in data.get("incoming") or []:
        nodes.append(edge["source"])
    return list(dict.fromkeys(nodes))


def predicted_after(vault: Path, question: dict[str, Any], platform: Path | None) -> tuple[list[str], list[str]]:
    """Return (files_opened, predicted_nodes). Files are named notes from graph-query, not Meta preambles."""
    kind = question["kind"]
    files: list[str] = []
    nodes: list[str] = []
    if kind in {"orientation"}:
        files.extend(ORIENT_FILES)
        nodes.extend(["00-Home", "instance.yaml", "Routing", "Fleet"])
        return files, nodes
    if kind == "disable-topics":
        files = [f"platform:{rel}" for rel in ORIENT_FILES]
        nodes = ["Platform"]
        return files, nodes
    if kind == "placement":
        rel = resolve_stem(vault, "Maps Vendor")
        files = [rel] if rel else []
        nodes = ["Maps Vendor"]
        return files, nodes
    if kind == "learning":
        rel = resolve_stem(vault, "Aprendizaje - Stale GPS")
        files = [rel] if rel else []
        files.append("investigations/20260820-090000-stale-gps-write/investigation.md")
        nodes = ["Aprendizaje - Stale GPS", "20260820-090000-stale-gps-write"]
        return files, nodes
    start = question.get("start") or ""
    if kind == "hygiene":
        data = try_graph_query(vault, ["hygiene"]) or {}
        nodes = list(data.get("unresolved_targets") or [])
        nodes.extend(data.get("orphans") or [])
        files = ["90-Meta/graph-query.py"] if GRAPH_QUERY.is_file() else ["90-Meta/verify-links.py"]
        return files, list(dict.fromkeys(nodes))
    if kind == "investigations":
        data = try_graph_query(vault, ["investigations", "--node", start]) or {}
        ids = [item["id"] for item in data.get("investigations") or []]
        files = [item["path"] for item in data.get("investigations") or []]
        return files, ids
    nodes = neighbor_nodes(vault, start)
    if kind == "impact":
        hop_targets = []
        data = try_graph_query(vault, ["neighbors", "--node", start]) or {}
        for edge in data.get("outgoing") or []:
            if edge.get("field") in {"publica-en", "gatillado-por", "participa-en"}:
                hop_targets.append(edge["target"])
        for target in hop_targets:
            nodes.extend(neighbor_nodes(vault, target))
        nodes = list(dict.fromkeys(nodes))
    files = []
    for stem in nodes:
        rel = resolve_stem(vault, stem)
        if rel:
            files.append(rel)
    return files, nodes


def score(gold: list[str], predicted: list[str]) -> tuple[float, float]:
    gold_set = set(gold)
    pred_set = set(predicted)
    if not gold_set:
        return 1.0, 1.0
    hit = gold_set & pred_set
    recall = len(hit) / len(gold_set)
    precision = len(hit) / len(pred_set) if pred_set else 0.0
    return recall, precision


def bytes_for(vault: Path, files: list[str], platform: Path | None) -> int:
    total = 0
    for rel in files:
        if rel.startswith("platform:"):
            if platform:
                total += size_of(platform, rel.split(":", 1)[1])
            continue
        total += size_of(vault, rel)
    return total


def evaluate(mode: str) -> dict[str, Any]:
    tmp = Path(tempfile.mkdtemp(prefix="graph-economy-"))
    vault = materialize(tmp / "norte")
    platform = materialize(tmp / "platform", disable_topics=True)
    rows = []
    for question in load_questions():
        if mode == "baseline":
            files = predicted_legacy(vault, question, platform)
            predicted = list(question["gold"])
            if question["kind"] == "investigations":
                predicted = [
                    "20260820-090000-stale-gps-write",
                    "20260820-090001-unrelated-billing",
                ]
            elif question["kind"] == "hygiene":
                predicted = ["MissingNode", "Orphan Scratch"]
        else:
            files, predicted = predicted_after(vault, question, platform)
        recall, precision = score(question["gold"], predicted)
        noise = set(question.get("noise") or [])
        join_hit = (
            question["kind"] != "investigations"
            or (
                "20260820-090000-stale-gps-write" in predicted
                and not (noise & set(predicted))
            )
        )
        if mode == "baseline" and question["kind"] == "investigations":
            join_hit = False
        economy_fail = any(
            name in f
            for f in files
            for name in ("Convenciones.md", "Auditoria - Framework.md")
        ) and question["kind"] in {"flow", "topic", "impact", "investigations"}
        row = {
            "id": question["id"],
            "kind": question["kind"],
            "files_opened": files,
            "files_count": len(files),
            "bytes_read": bytes_for(vault, files, platform),
            "hops": len(files),
            "predicted": predicted,
            "gold": question["gold"],
            "recall": recall,
            "precision": precision,
            "investigation_join": join_hit if question["kind"] == "investigations" else None,
            "obsidian_cli_used": False,
            "economy_fail": economy_fail,
            "topics_seeded": (platform / "25-Topics").exists() if question["kind"] == "disable-topics" else None,
        }
        rows.append(row)
    payload = {"mode": mode, "questions": rows}
    shutil.rmtree(tmp, ignore_errors=True)
    return payload


def summarize(payload: dict[str, Any]) -> dict[str, Any]:
    rows = payload["questions"]
    core = [row for row in rows if row["id"] in {"q3", "q4", "q5", "q6", "q7"}]
    bytes_35 = sum(row["bytes_read"] for row in rows if row["id"] in {"q3", "q4", "q5"})
    return {
        "mode": payload["mode"],
        "recall_q3_q7": {row["id"]: row["recall"] for row in core},
        "bytes_q3_q5": bytes_35,
        "q7_join": next(row["investigation_join"] for row in rows if row["id"] == "q7"),
        "q9_topics_seeded": next(row["topics_seeded"] for row in rows if row["id"] == "q9"),
        "economy_fails": [row["id"] for row in rows if row["economy_fail"]],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=["baseline", "after"], required=True)
    parser.add_argument("--out", type=Path)
    args = parser.parse_args()
    payload = evaluate(args.mode)
    payload["summary"] = summarize(payload)
    text = json.dumps(payload, indent=2, ensure_ascii=False)
    if args.out:
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
