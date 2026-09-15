"""Fixture installation and blindness checks; these do not test model behavior."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from datetime import datetime


SPEC = importlib.util.spec_from_file_location("lifecycle_fixture", Path(__file__).with_name("prepare.py"))
FIXTURE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(FIXTURE)


class FixtureTests(unittest.TestCase):
    def test_absorption_review_contract(self):
        self.assertIn("independent reviewer invocation", FIXTURE.CRITERIA["absorption"][0])
        negative = " ".join(FIXTURE.CRITERIA["absorption-review-unavailable"])
        self.assertIn("authorized attributed case update", negative)
        self.assertNotIn("investigation byte-for-byte unchanged", negative)

    def test_scenarios_install_and_separate_inputs(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            for scenario in FIXTURE.SCENARIOS:
                with self.subTest(scenario=scenario):
                    root, evaluation = base / scenario, base / (scenario + "-judge")
                    task = FIXTURE.prepare(FIXTURE.DIST, root, evaluation, scenario)
                    self.assertTrue((root / "vault/.knowledge-os.lock.yaml").is_file())
                    self.assertTrue((root / "vault/.agents/skills/manage-investigation/SKILL.md").is_file())
                    self.assertNotIn("criteria", task.read_text())
                    if scenario == "absorption":
                        self.assertIn("required independent final-note-review reviewer", task.read_text())
                        self.assertNotIn("Do not delegate.", task.read_text())
                    else:
                        self.assertIn("Do not delegate.", task.read_text())
                    if scenario == "absorption-review-unavailable":
                        self.assertTrue((root / "vault/.investigations-private" / FIXTURE.CASE_ID / "private.md").is_file())
                        self.assertEqual(FIXTURE.PROMPTS[scenario], FIXTURE.PROMPTS["absorption"])
                        self.assertIn("Leaves canonical notes and private context byte-for-byte unchanged", " ".join(FIXTURE.CRITERIA[scenario]))
                    self.assertFalse(list(root.rglob("criteria.json")))
                    self.assertFalse(list(root.rglob("provenance.json")))
                    now = datetime.fromisoformat(json.loads((root / "context.json").read_text())["now"].replace("Z", "+00:00"))
                    for case in (root / "vault/investigations").glob("*/investigation.md"):
                        updated = next(line.split(": ", 1)[1] for line in case.read_text().splitlines() if line.startswith("updated-at: "))
                        self.assertLessEqual(datetime.fromisoformat(updated.strip('"').replace("Z", "+00:00")), now)
                    hashes = json.loads((evaluation / "baseline-hashes.json").read_text())
                    for name, digest in hashes.items():
                        self.assertEqual(hashlib.sha256((root / name).read_bytes()).hexdigest(), digest)
                    self.assertTrue(json.loads((evaluation / "criteria.json").read_text())["criteria"])
                    if scenario == "observation-missing":
                        self.assertFalse((root / "sources/runtime-snapshot.json").exists())
                    if scenario == "observation-scheduling":
                        capabilities = json.loads((root / "context.json").read_text())["capabilities"]
                        self.assertFalse(capabilities["native_scheduler"])
                        self.assertFalse(capabilities["separate_session_dispatch"])
                        self.assertTrue((root / "sources/runtime-snapshot.json").is_file())
                        self.assertFalse((root / "vault/.operations" / FIXTURE.RUN_ID).exists())
                    if scenario.startswith("observation-") and scenario not in {"observation-plan", "observation-scheduling"}:
                        run = root / "vault/.operations" / FIXTURE.RUN_ID / "run.md"
                        expected = "cancelled" if scenario == "observation-cancelled" else "in-progress"
                        self.assertIn(f"estado: {expected}", run.read_text())
                        self.assertIn(f"investigation {FIXTURE.CASE_ID}", run.read_text())
                        self.assertIn("Rama y límite de efectos: Audit", run.read_text())
                        self.assertFalse(list((root / "vault/investigations").rglob("observation-plan.md")))
                    if scenario in {"retirement", "historical-lookup"}:
                        vault = root / "vault"
                        case_id = FIXTURE.RETIREMENT_CASE_ID
                        public = f"investigations/{case_id}"
                        private = vault / ".investigations-private" / case_id / "private.md"
                        self.assertTrue(private.is_file())
                        self.assertEqual(private.stat().st_mode & 0o777, 0o600)
                        self.assertEqual(FIXTURE.run(["git", "ls-files", "--", ".investigations-private"], vault), "")
                        self.assertEqual(FIXTURE.run(["git", "status", "--porcelain"], vault), "")
                        self.assertNotIn("ACK-073", task.read_text())
                        if scenario == "retirement":
                            self.assertIn("status: closed", (vault / public / "investigation.md").read_text())
                            self.assertEqual(FIXTURE.run(["git", "show", f"HEAD:{public}/artifacts/receipt.txt"], vault), (vault / public / "artifacts/receipt.txt").read_text())
                        else:
                            helper = vault / ".agents/skills/manage-investigation/scripts/investigation-case.py"
                            loaded = json.loads(FIXTURE.run(["python3", "-B", str(helper), "--root", str(vault / "investigations"), "load", "--id", case_id]))
                            self.assertEqual(loaded["status"], "retired")
                            self.assertEqual(loaded["commit_state"], "committed")
                            self.assertEqual(loaded["retirement_commit"], FIXTURE.run(["git", "rev-parse", "HEAD"], vault).strip())
                            self.assertIn("ACK-073", FIXTURE.run(["git", "show", f"{loaded['snapshot-commit']}:{public}/artifacts/receipt.txt"], vault))
                            self.assertFalse((vault / public).exists())
                            changed = FIXTURE.run(["git", "diff-tree", "--no-commit-id", "--name-status", "-r", "HEAD"], vault)
                            self.assertIn("A\tinvestigations/retired.md", changed)
                            self.assertIn(f"D\t{public}/artifacts/receipt.txt", changed)

    def test_rejects_overlapping_and_existing_roots_before_writing(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            for output, evaluation in ((base / "a", base / "a/judge"), (base / "b/run", base / "b"), (base / "c", base / "c")):
                with self.assertRaises(ValueError):
                    FIXTURE.prepare(FIXTURE.DIST, output, evaluation, "components")
                self.assertFalse(output.exists())
            with self.assertRaises(ValueError):
                FIXTURE.prepare(FIXTURE.DIST, base, base / "other", "components")

    def test_prompt_reproducible_and_private_input_stays_private(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            for label in ("a", "b"):
                FIXTURE.prepare(FIXTURE.DIST, base / label, base / (label + "-judge"), "absorption")
            self.assertEqual((base / "a/task.txt").read_bytes(), (base / "b/task.txt").read_bytes())
            for path in (base / "a/vault").rglob("*.md"):
                if ".investigations-private" not in path.parts:
                    self.assertNotIn("PRIVATE-CANARY-742", path.read_text())


if __name__ == "__main__":
    unittest.main()
