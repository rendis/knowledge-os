"""Safety properties of `kos kernel update` and `kos doctor` against the released binary.

Per ADR 0002 there is no Python installer any more: `kos` alone writes and inspects a cell. Doctor's
"unreproducible build" flag now comes from the binary's own build-time ldflags (`make release`), not
from live inspection of this checkout's Git state, so these tests avoid `--strict` where a dirty
development checkout would make it fail for reasons unrelated to what is being tested; the drift and
topology fields it would otherwise gate are asserted directly instead.
"""
import json
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _kos  # noqa: E402


def instance(dest: Path) -> None:
    (dest / 'instance.yaml').write_text(
        'version: 1\ncell:\n  name: T\n  purpose: T\nsystems:\n  - id: t\n    name: T\nadapters: []\n'
    )


class IntegrityTests(unittest.TestCase):
    def test_doctor_detects_adapter_configuration_pending_update(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'cell'
            init = _kos.run(DIST, 'init', '--vault', str(dest), '--cell-name', 'Test', '--purpose', 'Test',
                            '--system', 'test:Test', '--yes')
            self.assertEqual(init.returncode, 0, init.stdout + init.stderr)

            def doctor():
                return _kos.run(DIST, 'doctor', '--vault', str(dest))

            d = doctor()
            self.assertEqual(d.returncode, 0, d.stdout + d.stderr)
            self.assertEqual(json.loads(d.stdout)['adapter_configuration_drift'], [])

            path = dest / 'instance.yaml'
            path.write_text(path.read_text().replace('adapters: []', 'adapters: [reports]'))
            d = doctor()
            self.assertEqual(d.returncode, 0, d.stdout + d.stderr)
            self.assertEqual(json.loads(d.stdout)['adapter_configuration_drift'], ['reports'])

            update = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertEqual(update.returncode, 0, update.stdout + update.stderr)
            self.assertTrue((dest / '.agents/skills/generate-reports/SKILL.md').exists())
            self.assertEqual(json.loads(doctor().stdout)['adapter_configuration_drift'], [])

            path.write_text(path.read_text().replace('adapters: [reports]', 'adapters: []'))
            self.assertEqual(json.loads(doctor().stdout)['adapter_configuration_drift'], ['reports'])

            update = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertEqual(update.returncode, 0, update.stdout + update.stderr)
            self.assertFalse((dest / '.agents/skills/generate-reports/SKILL.md').exists())
            self.assertEqual(json.loads(doctor().stdout)['adapter_configuration_drift'], [])

    def test_real_claude_skills_preserved_before_any_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            (dest / '.claude/skills').mkdir(parents=True)
            (dest / '.claude/skills/custom').write_text('mine')
            instance(dest)
            result = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertFalse((dest / 'AGENTS.md').exists())
            self.assertEqual((dest / '.claude/skills/custom').read_text(), 'mine')

    def test_late_symlink_parent_rejected_before_any_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'vault'
            dest.mkdir()
            outside = Path(tmp) / 'outside'
            outside.mkdir()
            (dest / '90-Meta').symlink_to(outside, target_is_directory=True)
            instance(dest)
            result = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertFalse((dest / 'AGENTS.md').exists())
            self.assertEqual(list(outside.iterdir()), [])

    def test_doctor_requires_an_installed_kernel_only_when_strict(self):
        with tempfile.TemporaryDirectory() as tmp:
            plain = _kos.run(DIST, 'doctor', '--vault', tmp)
            self.assertEqual(plain.returncode, 0, plain.stdout + plain.stderr)
            strict = _kos.run(DIST, 'doctor', '--vault', tmp, '--strict')
            self.assertNotEqual(strict.returncode, 0, strict.stdout + strict.stderr)

    def test_doctor_detects_managed_and_topology_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'cell'
            init = _kos.run(DIST, 'init', '--vault', str(dest), '--cell-name', 'Test', '--purpose', 'Test',
                            '--system', 'test:Test', '--yes')
            self.assertEqual(init.returncode, 0, init.stdout + init.stderr)

            def doctor():
                return _kos.run(DIST, 'doctor', '--vault', str(dest))

            d = doctor()
            self.assertEqual(d.returncode, 0, d.stdout + d.stderr)
            info = json.loads(d.stdout)
            self.assertEqual(info['drift'], [])
            self.assertEqual(info['topology_drift'], [])
            self.assertFalse((dest / 'CLAUDE.md').exists())

            agents = dest / 'AGENTS.md'
            original = agents.read_text()
            agents.write_text('changed')
            d = doctor()
            self.assertEqual(d.returncode, 0, d.stdout + d.stderr)
            self.assertIn('AGENTS.md', json.loads(d.stdout)['drift'])
            agents.write_text(original)
            self.assertEqual(json.loads(doctor().stdout)['drift'], [])

            (dest / '.claude/skills').unlink()
            d = doctor()
            self.assertEqual(d.returncode, 0, d.stdout + d.stderr)
            self.assertEqual(json.loads(d.stdout)['topology_drift'], ['.claude/skills'])

            (dest / '.claude/skills').symlink_to('../.agents/skills')
            d = doctor()
            self.assertEqual(json.loads(d.stdout)['drift'], [])
            self.assertEqual(json.loads(d.stdout)['topology_drift'], [])


if __name__ == '__main__':
    unittest.main()
