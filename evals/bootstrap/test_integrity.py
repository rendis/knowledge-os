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
                lock = installer.load_lock(dest / installer.LOCK_NAME)
                lock.update(distribution_revision='a' * 40, distribution_dirty=False)
                (dest / installer.LOCK_NAME).write_text(installer.dump_lock(lock))
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

    def test_retired_provider_pack_update_preserves_cell_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            command = ['python3', '-B', str(ROOT / 'scripts/knowledge_os.py')]
            subprocess.run(command + ['init', '--dest', tmp, '--cell-name', 'Test',
                           '--purpose', 'Test', '--system', 'test:Test', '--yes'],
                           check=True, capture_output=True)
            retired = dest / '.agents/skills/gcloud/SKILL.md'
            retired.parent.mkdir()
            retired.write_text('old distributed helper')
            custom = retired.parent / 'local.md'
            custom.write_text('developer owned')
            lock = installer.load_lock(dest / installer.LOCK_NAME)
            lock['adapters'] = ['gcp', 'postgres']
            lock['managed_hashes']['.agents/skills/gcloud/SKILL.md'] = installer.sha256_file(retired)
            (dest / installer.LOCK_NAME).write_text(installer.dump_lock(lock))
            # Configuration still naming removed packs fails without touching the vault.
            instance = dest / 'instance.yaml'
            original = instance.read_text()
            instance.write_text(original.replace('adapters: []', 'adapters: [gcp, postgres]'))
            refused = subprocess.run(command + ['update', '--dest', tmp], capture_output=True)
            self.assertNotEqual(refused.returncode, 0)
            self.assertTrue(retired.exists())
            doctor = subprocess.run(command + ['doctor', '--dest', tmp, '--strict'],
                                    capture_output=True, text=True)
            self.assertNotEqual(doctor.returncode, 0)
            self.assertNotIn('Traceback', doctor.stderr)
            instance.write_text(original)
            # Existing drift protection still applies to a retired managed file.
            retired.write_text('locally modified helper')
            refused = subprocess.run(command + ['update', '--dest', tmp], capture_output=True)
            self.assertNotEqual(refused.returncode, 0)
            self.assertEqual(retired.read_text(), 'locally modified helper')
            retired.write_text('old distributed helper')
            subprocess.run(command + ['update', '--dest', tmp], check=True, capture_output=True)
            self.assertFalse(retired.exists())
            self.assertEqual(custom.read_text(), 'developer owned')
            self.assertTrue((dest / '.agents/skills/inspect-database/SKILL.md').is_file())
            self.assertEqual(instance.read_text(), original)
            self.assertEqual(installer.load_lock(dest / installer.LOCK_NAME)['adapters'], [])

    def test_real_claude_skills_preserved_before_any_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)
            (dest / '.claude/skills').mkdir(parents=True)
            (dest / '.claude/skills/custom').write_text('mine')
            with self.assertRaises(RuntimeError):
                installer.copy_kernel(dest, [])
            self.assertFalse((dest / 'AGENTS.md').exists())
            self.assertEqual((dest / '.claude/skills/custom').read_text(), 'mine')

    def test_late_symlink_parent_rejected_before_any_copy(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'vault'
            dest.mkdir()
            outside = Path(tmp) / 'outside'
            outside.mkdir()
            (dest / '90-Meta').symlink_to(outside, target_is_directory=True)
            with self.assertRaises(RuntimeError):
                installer.copy_kernel(dest, [])
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
            lock = installer.load_lock(dest / installer.LOCK_NAME)
            lock.update(distribution_revision='a' * 40, distribution_dirty=False)
            (dest / installer.LOCK_NAME).write_text(installer.dump_lock(lock))
            args = SimpleNamespace(dest=tmp, strict=True)
            with patch.object(installer, 'distribution_provenance', return_value=('a' * 40, False)), contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(installer.cmd_doctor(args), 0)
                agents = dest / 'AGENTS.md'
                original = agents.read_text()
                agents.write_text('changed')
                self.assertEqual(installer.cmd_doctor(args), 2)
                agents.write_text(original)
                (dest / 'CLAUDE.md').unlink()
                self.assertEqual(installer.cmd_doctor(args), 2)
                (dest / 'CLAUDE.md').symlink_to('AGENTS.md')
                lock['distribution_dirty'] = True
                (dest / installer.LOCK_NAME).write_text(installer.dump_lock(lock))
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
