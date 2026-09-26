import importlib.util
from pathlib import Path
import tempfile
import subprocess
import contextlib
import io
from types import SimpleNamespace
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('installer_integrity', ROOT / 'scripts/knowledge_os.py')
installer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(installer)
import native_runtime  # noqa: E402  (scripts/ is on the path once the installer loaded)


def reproducible(dest: Path, dirty: bool = False) -> None:
    """Pretend the lock came from a clean distribution commit."""
    path = dest / installer.LOCK_NAME
    lines = []
    for line in path.read_text().splitlines():
        if line.startswith('distribution_revision:'):
            line = 'distribution_revision: "' + 'a' * 40 + '"'
        elif line.startswith('distribution_dirty:'):
            line = 'distribution_dirty: ' + ('true' if dirty else 'false')
        lines.append(line)
    path.write_text('\n'.join(lines) + '\n')


def instance(dest: Path) -> None:
    (dest / 'instance.yaml').write_text('version: 1\ncell:\n  name: T\n  purpose: T\nsystems:\n  - id: t\n    name: T\nadapters: []\n')

class IntegrityTests(unittest.TestCase):
    def test_strict_doctor_detects_adapter_configuration_pending_update(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            def cli(verb):
                return subprocess.run(['python3', '-B', str(ROOT / 'scripts/knowledge_os.py'),
                                       verb, '--dest', tmp], check=True, capture_output=True)
            subprocess.run(['python3', '-B', str(ROOT / 'scripts/knowledge_os.py'),
                            'init', '--dest', tmp, '--cell-name', 'Test', '--purpose', 'Test',
                            '--system', 'test:Test', '--yes'], check=True, capture_output=True)
            args = SimpleNamespace(dest=tmp, strict=True)
            def doctor():
                reproducible(dest)
                with patch.object(installer, 'distribution_provenance', return_value=('a' * 40, False)), contextlib.redirect_stdout(io.StringIO()):
                    return installer.cmd_doctor(args)
            self.assertEqual(doctor(), 0)
            path = dest / 'instance.yaml'
            path.write_text(path.read_text().replace('adapters: []', 'adapters: [reports]'))
            self.assertEqual(doctor(), 2)
            cli('update')
            self.assertTrue((dest / '.agents/skills/generate-reports/SKILL.md').exists())
            self.assertEqual(doctor(), 0)
            path.write_text(path.read_text().replace('adapters: [reports]', 'adapters: []'))
            self.assertEqual(doctor(), 2)
            cli('update')
            self.assertFalse((dest / '.agents/skills/generate-reports/SKILL.md').exists())
            self.assertEqual(doctor(), 0)

    def test_real_claude_skills_preserved_before_any_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            (dest / '.claude/skills').mkdir(parents=True)
            (dest / '.claude/skills/custom').write_text('mine')
            instance(dest)
            code, result = native_runtime.kernel(ROOT, dest)
            self.assertNotEqual(code, 0, result)
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
            code, result = native_runtime.kernel(ROOT, dest)
            self.assertNotEqual(code, 0, result)
            self.assertFalse((dest / 'AGENTS.md').exists())
            self.assertEqual(list(outside.iterdir()), [])

    def test_strict_doctor_requires_installed_reproducible_state(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = SimpleNamespace(dest=tmp, strict=False)
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(installer.cmd_doctor(args), 0)
                args.strict = True
                self.assertEqual(installer.cmd_doctor(args), 2)

    def test_strict_doctor_detects_managed_and_topology_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            subprocess.run(['python3', '-B', str(ROOT / 'scripts/knowledge_os.py'),
                            'init', '--dest', tmp, '--cell-name', 'Test', '--purpose', 'Test',
                            '--system', 'test:Test', '--yes'], check=True, capture_output=True)
            reproducible(dest)
            args = SimpleNamespace(dest=tmp, strict=True)
            with patch.object(installer, 'distribution_provenance', return_value=('a' * 40, False)), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(installer.cmd_doctor(args), 0)
                self.assertFalse((dest / 'CLAUDE.md').exists())
                agents = dest / 'AGENTS.md'
                original = agents.read_text()
                agents.write_text('changed')
                self.assertEqual(installer.cmd_doctor(args), 2)
                agents.write_text(original)
                (dest / '.claude/skills').unlink()
                self.assertEqual(installer.cmd_doctor(args), 2)
                (dest / '.claude/skills').symlink_to('../.agents/skills')
                reproducible(dest, dirty=True)
                self.assertEqual(installer.cmd_doctor(args), 2)

    def test_ignored_shipped_file_marks_distribution_dirty(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'kernel').mkdir()
            (root / 'adapters').mkdir()
            (root / 'MANAGED_PATHS').write_text('payload/\n')
            (root / 'kernel/payload').mkdir()
            (root / 'kernel/payload/tracked').write_text('tracked')
            (root / '.gitignore').write_text('ignored\n')
            def git(*args):
                subprocess.run(['git', '-C', str(root), *args], check=True, capture_output=True)
            git('init')
            git('add', '.')
            git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-m', 'base')
            with patch.object(installer, 'DIST', root):
                self.assertFalse(installer.distribution_provenance()[1])
                git('update-index', '--skip-worktree', 'kernel/payload/tracked')
                (root / 'kernel/payload/tracked').write_text('hidden modification')
                self.assertTrue(installer.distribution_provenance()[1])
                (root / 'kernel/payload/tracked').write_text('tracked')
                (root / 'kernel/payload/ignored').write_text('shipped')
                self.assertTrue(installer.distribution_provenance()[1])

if __name__ == '__main__':
    unittest.main()
