"""Regressions reproduced by a completed consumer mapping campaign."""
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

META = Path(__file__).resolve().parents[2] / "kernel/90-Meta"
sys.path.insert(0, str(META))


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, META / filename)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


manifest = load("tracking_manifest", "git-change-manifest.py")
notes = load("tracking_notes", "review-note-candidate.py")
sync = load("tracking_sync", "sync-run.py")


class TrackingRegressions(unittest.TestCase):
    def test_tool_identity_is_computed_and_drift_cannot_be_hidden(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            tool_dir = root / "tools"
            tool_dir.mkdir()
            for path in META.glob("*.py"):
                shutil.copy2(path, tool_dir / path.name)

            def run(*args):
                result = subprocess.run([sys.executable, "-B", str(tool_dir / "sync-run.py"), *args], capture_output=True, text=True)
                return result.returncode, json.loads(result.stdout)

            code, identity = run("tool-digest")
            self.assertEqual(code, 0)
            self.assertEqual(identity["tool_digest"], sync.installed_tool_digest())
            begin = ("begin", "--state-root", str(root / "state"), "--inventory-digest", "b" * 64, "--package", "service", "1" * 40)
            code, started = run(*begin)
            self.assertEqual(code, 0, started)
            stored = json.loads(sync.run_path(root / "state", started["run_id"]).read_text())
            self.assertEqual(stored["tool_digest"], identity["tool_digest"])
            with (tool_dir / "git-change-manifest.py").open("a") as handle:
                handle.write("\n# changed tool bytes\n")
            code, rejected = run(*begin, "--tool-digest", identity["tool_digest"])
            self.assertEqual(code, 2)
            self.assertEqual(rejected["code"], "run-version-mismatch")
            for supplied in ((), ("--tool-digest", identity["tool_digest"])):
                code, rejected = run("resume", "--state-root", str(root / "state"), "--run-id", started["run_id"], *supplied)
                self.assertEqual(code, 2)
                self.assertEqual(rejected["code"], "run-version-mismatch")
            self.assertEqual(run("status", "--state-root", str(root / "state"), "--run-id", started["run_id"])[0], 0)

    def test_unicode_note_review_uses_same_digest_as_sync_gate(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            vault, candidate, evidence = (root / name for name in ("vault", "candidate", "evidence"))
            for directory in (vault, candidate, evidence):
                directory.mkdir()
            relative = "50-Glosario/Nota de crédito.md"
            target = candidate / relative
            target.parent.mkdir()
            target.write_text("# Nota de crédito\n\nTransacción revisada.\n", encoding="utf-8")
            (evidence / "evidencia.json").write_text('{"razón":"verificación"}', encoding="utf-8")
            frozen = notes.freeze_candidate(vault, candidate, evidence, ["evidencia.json"])
            self.assertEqual(notes.canonical_digest(frozen), manifest.canonical_digest(frozen))
            self.assertEqual(notes.canonical_digest(frozen), sync.canonical_digest(frozen))
            manifest_path, review_path = root / "manifest.json", root / "review.json"
            manifest_path.write_text(json.dumps(frozen, ensure_ascii=False, indent=2), encoding="utf-8")
            review = {"version": 1, "manifest_digest": sync.canonical_digest(frozen), "verdict": "accept", "findings": [], "connection_decisions": {}}
            review_path.write_text(json.dumps(review), encoding="utf-8")
            self.assertEqual(notes.check_candidate(vault, candidate, evidence, manifest_path, review_path)["status"], "pass")
            self.assertNotEqual(sync.bytes_digest(manifest_path.read_bytes()), sync.canonical_digest(frozen))

    def test_empty_claims_do_not_invent_credential_redaction(self):
        for old_oid in ("1" * 40, next(iter(manifest.EMPTY_TREE_OIDS))):
            with self.subTest(old_oid=old_oid):
                analysis = {"repository": "service", "claims": [], "nodes": [{"basename": "service", "action": "create", "claim_ids": []}], "paths": [], "blockers": []}
                source = {"credential_suspects": [], "environment_configs": [], "old_oid": old_oid}
                finalized, _ = manifest.finalize_analysis_payload(analysis, source, set())
                self.assertEqual(finalized["nodes"][0]["reason"], manifest.FALLBACK_REASON)
                self.assertNotIn(manifest.REDACTION_REASON, json.dumps(finalized))

    def test_real_credential_suspect_retains_redaction_reason(self):
        analysis = {"repository": "service", "claims": [], "nodes": [], "paths": [{"path": ".env", "disposition": "document", "claim_ids": []}], "blockers": []}
        source = {"credential_suspects": [{"path": ".env"}], "environment_configs": [], "old_oid": "1" * 40}
        finalized, _ = manifest.finalize_analysis_payload(analysis, source, set())
        self.assertEqual(finalized["paths"][0]["reason"], manifest.REDACTION_REASON)
        self.assertEqual(finalized["paths"][0]["disposition"], "not-documentable")


if __name__ == "__main__":
    unittest.main()
