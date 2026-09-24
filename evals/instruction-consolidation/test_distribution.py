"""Exercise installed specialist parity and safe retirement of consolidated docs."""
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("render_specialists", ROOT / "scripts/render_specialists.py")
renderer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(renderer)


class DistributionTests(unittest.TestCase):
    def command(self, *args):
        return subprocess.run(["sh", str(ROOT / "install.sh"), *args], capture_output=True, text=True)

    def test_native_projections_embed_same_contract_and_role(self):
        self.assertEqual(len(renderer.projections()), 6)
        for path, expected in renderer.projections().items():
            self.assertEqual(path.read_text(), expected, str(path))
            self.assertIn("Withhold the classification", expected)
            self.assertIn("omitted", expected)
        for name in renderer.DESCRIPTIONS:
            text = (ROOT / f"kernel/.codex/agents/{name}.toml").read_text()
            prompt = json.loads(text.split("developer_instructions = ", 1)[1])
            self.assertIn("# Shared evidence contract", prompt)
            self.assertIn((ROOT / f"kernel/90-Meta/specialists/{name}.md").read_text().strip(), prompt)

    def test_blind_review_bundle_contains_frozen_contract(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            frozen = root / "distribution/kernel"
            frozen.mkdir(parents=True)
            (frozen / "AGENTS.md").write_text((ROOT / "kernel/AGENTS.md").read_text())
            for name in ("90-Meta/Auditoria - Framework.md", "90-Meta/vault-resolution.md", ".agents/skills/manage-investigation/references/readiness-and-lifecycle.md"):
                target = frozen / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("Frozen common policy")
            (root / "results.json").write_text("[]")
            (root / "campaign.json").write_text(json.dumps({"cases": [], "distribution": str(frozen.parent)}))
            for legacy in (False, True):
                if legacy:
                    (frozen / "90-Meta/response-quality.md").write_text("Frozen legacy review contract")
                output = root / str(legacy)
                result = subprocess.run([sys.executable, "-B", str(ROOT / "evals/efficiency/prepare_review.py"), str(root), "--output", str(output)], capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                evidence = (output / "sources.md").read_text()
                self.assertIn("Frozen legacy review contract" if legacy else "### Review before delivery", evidence)
                self.assertNotIn("## Personal instructions", evidence)

    def test_install_update_retirement_and_consumer_preservation(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / "vault"
            result = self.command("init", "--dest", str(dest), "--cell-name", "Example", "--purpose", "Evidence", "--system", "example:Example", "--yes")
            self.assertEqual(result.returncode, 0, result.stderr)
            for path, expected in renderer.projections().items():
                installed = dest / path.relative_to(ROOT / "kernel")
                self.assertEqual(installed.read_text(), expected)
            own = dest / ".claude/agents/local-review.md"
            own.write_text("consumer-owned\n")
            lockpath = dest / ".knowledge-os.lock.yaml"
            lock = lockpath.read_text()
            for filename in ("SOUL.md", "response-quality.md"):
                rel = f"90-Meta/{filename}"
                original = f"# Previous managed {filename}\nRetired contract fixture.\n".encode()
                (dest / rel).write_bytes(original)
                digest = hashlib.sha256(original).hexdigest()
                lock = lock.replace("managed_hashes:\n", f'managed_hashes:\n  "{rel}": {digest}\n', 1)
            lockpath.write_text(lock)
            soul = dest / "90-Meta/SOUL.md"
            original_soul = soul.read_bytes()
            soul.write_bytes(original_soul + b"local change\n")
            blocked = self.command("update", "--dest", str(dest))
            self.assertEqual(blocked.returncode, 3, blocked.stdout + blocked.stderr)
            self.assertTrue((dest / "90-Meta/response-quality.md").exists())
            soul.write_bytes(original_soul)
            updated = self.command("update", "--dest", str(dest))
            self.assertEqual(updated.returncode, 0, updated.stdout + updated.stderr)
            for filename in ("SOUL.md", "response-quality.md"):
                self.assertFalse((dest / "90-Meta" / filename).exists())
                self.assertNotIn(f'"90-Meta/{filename}":', lockpath.read_text())
            self.assertEqual(own.read_text(), "consumer-owned\n")
            # Specialist customization is protected by the same ownership contract.
            native = dest / ".codex/agents/evidence-reviewer.toml"
            native.write_text(native.read_text() + "# local change\n")
            blocked = self.command("update", "--dest", str(dest))
            self.assertEqual(blocked.returncode, 3, blocked.stdout + blocked.stderr)
            self.assertTrue(native.read_text().endswith("# local change\n"))


if __name__ == "__main__":
    unittest.main()
