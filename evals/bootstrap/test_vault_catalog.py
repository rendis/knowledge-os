"""Consumer-owned catalog survives the kos lifecycle (init/update/adopt) and is diagnosed by `kos doctor`."""
from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
CATALOG_PATH = Path("90-Meta/vault-catalog.yaml")

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _kos  # noqa: E402


class CatalogLifecycleTests(unittest.TestCase):
    def test_catalog_ownership_and_doctor(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp) / 'vault'

            init_args = ('--cell-name', 'Orders', '--purpose', 'Order processing',
                         '--system', 'orders:Orders', '--yes')
            result = _kos.run(DIST, 'init', '--vault', str(dest), *init_args)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            path = dest / CATALOG_PATH
            self.assertFalse(path.exists())
            doctor = _kos.run(DIST, 'doctor', '--vault', str(dest))
            self.assertEqual(doctor.returncode, 0, doctor.stdout + doctor.stderr)
            self.assertEqual(json.loads(doctor.stdout)['vault_catalog']['status'], 'absent')

            guide = (DIST / 'kernel/.agents/skills/map-ecosystem/references/vault-catalog.md').read_text()
            content = '# Team-owned catalog\n' + guide.split('```yaml\n')[1].split('```')[0]
            path.write_text(content)
            before = path.read_bytes()

            # `init` on an already-installed destination is refused; it must still preserve the catalog.
            refused = _kos.run(DIST, 'init', '--vault', str(dest), *init_args)
            self.assertNotEqual(refused.returncode, 0)
            self.assertEqual(path.read_bytes(), before)

            updated = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertEqual(updated.returncode, 0, updated.stdout + updated.stderr)
            self.assertEqual(path.read_bytes(), before)
            self.assertNotIn(CATALOG_PATH.as_posix(), (dest / '.knowledge-os.lock.yaml').read_text())

            (dest / '.knowledge-os.lock.yaml').unlink()
            adopted = _kos.run(DIST, 'adopt', '--vault', str(dest))
            self.assertEqual(adopted.returncode, 0, adopted.stdout + adopted.stderr)
            self.assertEqual(path.read_bytes(), before)
            self.assertNotIn(CATALOG_PATH.as_posix(), (dest / '.knowledge-os.lock.yaml').read_text())

            doctor = _kos.run(DIST, 'doctor', '--vault', str(dest))
            self.assertEqual(doctor.returncode, 0, doctor.stdout + doctor.stderr)
            self.assertEqual(json.loads(doctor.stdout)['vault_catalog'], {'status': 'valid', 'count': 1})

            path.write_text('version: 999\nvaults: []\n')
            invalid = path.read_bytes()
            for args in ((), ('--strict',)):
                doctor = _kos.run(DIST, 'doctor', '--vault', str(dest), *args)
                self.assertNotEqual(doctor.returncode, 0, doctor.stdout + doctor.stderr)
                self.assertEqual(json.loads(doctor.stdout)['vault_catalog']['status'], 'invalid')
                self.assertEqual(path.read_bytes(), invalid)
            update = _kos.run(DIST, 'kernel', 'update', '--vault', str(dest))
            self.assertEqual(update.returncode, 0, update.stdout + update.stderr)
            self.assertEqual(path.read_bytes(), invalid)


if __name__ == '__main__':
    unittest.main()
