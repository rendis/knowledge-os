#!/usr/bin/env python3
"""Tests for graph-query against the Norte fixture."""
from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(DIST / "evals" / "graph-economy"))

from materialize import materialize  # noqa: E402


def load_graph_query():
    path = DIST / "kernel" / "90-Meta" / "graph-query.py"
    spec = importlib.util.spec_from_file_location("graph_query", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class GraphQueryTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.tmp = Path(tempfile.mkdtemp(prefix="graph-query-test-"))
        cls.vault = materialize(cls.tmp / "norte")
        cls.gq = load_graph_query()

    def test_neighbors_planner(self) -> None:
        data = self.gq.neighbors(self.vault, "routing-planner")
        targets = {edge["target"] for edge in data["outgoing"]}
        self.assertIn("route-events", targets)
        self.assertIn("Flujo - Dispatch", targets)
        sources = {edge["source"] for edge in data["incoming"]}
        self.assertIn("Routing", sources)

    def test_hygiene(self) -> None:
        data = self.gq.hygiene(self.vault)
        self.assertIn("MissingNode", data["unresolved_targets"])
        self.assertIn("Orphan Scratch", data["orphans"])

    def test_personal_instructions_do_not_change_graph_or_hygiene(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "Public.md").write_text("# Public\n", encoding="utf-8")
            graph_before = self.gq.build_graph(root)
            hygiene_before = self.gq.hygiene(root)
            (root / "AGENTS.personal.md").write_text(
                "# Personal\nUse [[Private mail profile]] and [[Public]].\n",
                encoding="utf-8",
            )
            self.assertEqual(self.gq.build_graph(root), graph_before)
            self.assertEqual(self.gq.hygiene(root), hygiene_before)

    def test_investigations_join(self) -> None:
        data = self.gq.investigations(self.vault, "routing-planner")
        ids = [item["id"] for item in data["investigations"]]
        self.assertEqual(ids, ["20260820-090000-stale-gps-write"])


if __name__ == "__main__":
    unittest.main()
