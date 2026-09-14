import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


spec = importlib.util.spec_from_file_location(
    "closure", Path(__file__).resolve().parents[2] / "kernel/90-Meta/check-map-closure.py"
)
tool = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tool)


class CurrentCounts(unittest.TestCase):
    def test_stale_checkpoint_and_summary_block(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / "20-Repos").mkdir()
            (root / "20-Repos/service.md").write_text("- [ ] pending\n- [x] closed\n")
            for name in ("00-Home.md", "coverage.md"):
                (root / name).write_text("Permanecen **1 verificaciones en 1 notas**.")
            state = {"visible_coverage": {
                "remaining_verification_items": 2, "notes_with_verifications": 1,
                "paths": ["00-Home.md", "coverage.md"],
            }, "historical": {"remaining_verification_items": 99}}
            checkpoint = root / "checkpoint.json"
            checkpoint.write_text(json.dumps(state))
            self.assertEqual(tool.check(root, checkpoint)["status"], "blocked")
            state["visible_coverage"]["remaining_verification_items"] = 1
            checkpoint.write_text(json.dumps(state))
            self.assertEqual(tool.check(root, checkpoint)["status"], "pass")
            (root / "coverage.md").write_text("Remaining: 2 verification items in 1 notes.")
            self.assertEqual(tool.check(root, checkpoint)["status"], "blocked")
            (root / "coverage.md").write_text("Remaining: 1 verification items in 1 notes.")
            self.assertEqual(tool.check(root, checkpoint)["status"], "pass")


if __name__ == "__main__":
    unittest.main()
