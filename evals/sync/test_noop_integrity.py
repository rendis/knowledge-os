import unittest
import tempfile
from pathlib import Path
import test_sync_state as base
from test_sync_state import make_repository_pair, gate_v2, digest, bytes_digest, write_json

class NoopIntegrity(base.SyncRunStateEval):
    def test_unchanged_file_does_not_imply_partial_apply(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault = root / "vault"
            first_path, second_path = "10-Sistemas/target-service.md", "20-Repos/target-service.md"
            first = vault / first_path
            first.parent.mkdir(parents=True)
            first.write_text("before-one\n", encoding="utf-8")
            patch = (
                f"--- a/{first_path}\n+++ b/{first_path}\n@@ -1 +1 @@\n-before-one\n+before-one\n"
                f"--- /dev/null\n+++ b/{second_path}\n@@ -0,0 +1 @@\n+after-two\n"
            )
            projection = {
                "version": 1, "run_id": run_id, "gate_digest": digest(gate),
                "unit_id": "group-001", "unit_type": "write-group", "patch_digest": bytes_digest(patch),
                "base_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("")},
                "result_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("after-two\n")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "crash.json", root / "crash.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            retried = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault),
            )
            self.assertEqual(retried.returncode, 0, retried.stdout + retried.stderr)
            self.assertEqual(first.read_text(), "before-one\n")
            self.assertEqual((vault / second_path).read_text(), "after-two\n")


    def test_unchanged_file_does_not_imply_partial_resume(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            _, _, oids = make_repository_pair(root)
            gate = gate_v2(oids)
            run_id = self.begin(root, oids)
            self.seal(root, run_id, gate)
            vault = root / "vault"
            first_path, second_path = "10-Sistemas/target-service.md", "20-Repos/target-service.md"
            first = vault / first_path
            first.parent.mkdir(parents=True)
            first.write_text("before-one\n", encoding="utf-8")
            patch = (
                f"--- a/{first_path}\n+++ b/{first_path}\n@@ -1 +1 @@\n-before-one\n+before-one\n"
                f"--- /dev/null\n+++ b/{second_path}\n@@ -0,0 +1 @@\n+after-two\n"
            )
            projection = {
                "version": 1, "run_id": run_id, "gate_digest": digest(gate),
                "unit_id": "group-001", "unit_type": "write-group", "patch_digest": bytes_digest(patch),
                "base_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("")},
                "result_files": {first_path: bytes_digest("before-one\n"), second_path: bytes_digest("after-two\n")},
                "grants": gate["write_groups"][0]["grants"],
            }
            projection_path, patch_path = root / "crash.json", root / "crash.patch"
            write_json(projection_path, projection)
            patch_path.write_text(patch, encoding="utf-8")
            self.assertEqual(self.validate_unit(root, run_id, "group-001", projection_path, patch_path).returncode, 0)
            retried = self.run_sync(
                "apply-unit", "--state-root", str(root / "state"), "--run-id", run_id,
                "--unit-id", "group-001", "--vault", str(vault), "--fail-after-writes", "1",
            )
            self.assertEqual(retried.returncode, 1, retried.stdout + retried.stderr)
            resumed = self.run_sync("resume", "--state-root", str(root / "state"), "--run-id", run_id)
            self.assertEqual(resumed.returncode, 0, resumed.stdout + resumed.stderr)
            self.assertEqual(first.read_text(), "before-one\n")
            self.assertFalse((vault / second_path).exists())


if __name__ == '__main__':
    unittest.main(defaultTest=['NoopIntegrity.test_unchanged_file_does_not_imply_partial_apply', 'NoopIntegrity.test_unchanged_file_does_not_imply_partial_resume'])
