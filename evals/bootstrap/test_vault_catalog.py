"""Consumer-owned catalog survives installer lifecycle and is diagnosed."""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(DIST / 'kernel/90-Meta'))
from vault_catalog import CATALOG_PATH  # noqa: E402


class CatalogLifecycleTests(unittest.TestCase):
    def test_catalog_ownership_and_doctor(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'vault'

            def install(command, *args):
                return subprocess.run(['sh', str(DIST / 'install.sh'), command,
                                       '--dest', str(dest), *args],
                                      capture_output=True, text=True)

            init_args = ('--cell-name', 'Orders', '--purpose', 'Order processing',
                         '--system', 'orders:Orders', '--yes')
            result = install('init', *init_args)
            self.assertEqual(result.returncode, 0, result.stderr)
            path = dest / CATALOG_PATH
            self.assertFalse(path.exists())
            self.assertEqual(json.loads(install('doctor').stdout)['vault_catalog']['status'], 'absent')
            guide = (DIST / 'kernel/.agents/skills/map-ecosystem/references/vault-catalog.md').read_text()
            content = '# Team-owned catalog\n' + guide.split('```yaml\n')[1].split('```')[0]
            path.write_text(content)
            before = path.read_bytes()
            for command in ('init', 'update', 'adopt'):
                if command == 'adopt':
                    (dest / '.knowledge-os.lock.yaml').unlink()
                result = install(command, *(init_args if command == 'init' else ()))
                # init on an installed destination may refuse; it must still preserve content.
                if command != 'init':
                    self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(path.read_bytes(), before)
                if (dest / '.knowledge-os.lock.yaml').exists():
                    lock = (dest / '.knowledge-os.lock.yaml').read_text()
                    self.assertNotIn(CATALOG_PATH.as_posix(), lock)
            doctor = install('doctor')
            self.assertEqual(doctor.returncode, 0, doctor.stderr)
            self.assertEqual(json.loads(doctor.stdout)['vault_catalog'], {'status': 'valid', 'count': 1})
            path.write_text('version: 999\nvaults: []\n')
            invalid = path.read_bytes()
            for args in ((), ('--strict',)):
                doctor = install('doctor', *args)
                self.assertEqual(doctor.returncode, 2)
                self.assertEqual(json.loads(doctor.stdout)['vault_catalog']['status'], 'invalid')
                self.assertEqual(path.read_bytes(), invalid)
            self.assertEqual(install('update').returncode, 0)
            self.assertEqual(path.read_bytes(), invalid)


if __name__ == '__main__':
    unittest.main()
