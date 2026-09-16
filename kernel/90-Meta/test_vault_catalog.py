"""Catalog validation and read-only behavior."""
from __future__ import annotations

import copy
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from vault_catalog import CATALOG_PATH, CatalogError, load_catalog, validate_catalog

META = Path(__file__).resolve().parent


def entry():
    return dict(id='payments', repository='https://example.org/team/payments.git',
                domain='Payments', scope='Authorization', summary='Payment services',
                tags=['payments'], relationship='Orders request authorization',
                consult_when=['Authorization failures'])


class CatalogTests(unittest.TestCase):
    def test_absent_is_empty_without_writes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.assertEqual(load_catalog(root), {'version': 1, 'vaults': []})
            self.assertEqual(list(root.iterdir()), [])

    def test_example_and_cli_are_valid_and_read_only(self):
        guide = (META.parent / '.agents/skills/map-ecosystem/references/vault-catalog.md').read_text()
        example = guide.split('```yaml\n')[1].split('```')[0]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / CATALOG_PATH
            path.parent.mkdir()
            path.write_text(example)
            before = path.read_bytes()
            self.assertEqual(load_catalog(root)['vaults'][0]['id'], 'payments')
            for command in ('list', 'validate'):
                result = subprocess.run([sys.executable, '-B', str(META / 'vault-catalog.py'),
                                         command, '--root', tmp], capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(json.loads(result.stdout)['count'], 1)
            self.assertEqual(path.read_bytes(), before)

    def test_duplicates_across_remote_transports(self):
        first = entry()
        for remote in ('git@example.org:team/payments.git',
                       'ssh://git@example.org/team/payments',
                       'https://EXAMPLE.org:443/team/payments/'):
            second = dict(first, id='other', repository=remote)
            with self.subTest(remote=remote), self.assertRaises(CatalogError):
                validate_catalog({'version': 1, 'vaults': [first, second]})
        with self.assertRaises(CatalogError):
            validate_catalog({'version': 1, 'vaults': [first, dict(first, repository='https://example.org/other')]})

    def test_duplicate_repository_case_variants(self):
        first = dict(entry(), repository='https://github.com/Example/Payments.git')
        for remote in ('git@github.com:example/payments.git',
                       'ssh://git@GITHUB.com/EXAMPLE/PAYMENTS.GIT',
                       'https://github.com/example/payments.GiT/'):
            with self.subTest(remote=remote), self.assertRaises(CatalogError):
                validate_catalog({'version': 1, 'vaults': [
                    first, dict(first, id='payments-alias', repository=remote)]})

    def test_local_paths_and_credential_urls_rejected(self):
        for remote in ('/tmp/vault', 'file:///tmp/vault', 'https://token@example.org/repo',
                       'ssh://git:secret@example.org/repo', 'https://example.org/repo?token=secret'):
            with self.subTest(remote=remote), self.assertRaises(CatalogError):
                validate_catalog({'version': 1, 'vaults': [dict(entry(), repository=remote)]})

    def test_closed_fields_and_types(self):
        data = {'version': 1, 'vaults': [entry()]}
        for key, value in [('tags', 'payments'), ('consult_when', []), ('summary', ''),
                           ('local_path', '/tmp/vault'), ('access', True), ('id', 'Bad ID')]:
            changed = copy.deepcopy(data)
            changed['vaults'][0][key] = value
            with self.subTest(key=key), self.assertRaises(CatalogError):
                validate_catalog(changed)
        for version in (True, 2, '1'):
            with self.assertRaises(CatalogError):
                validate_catalog(dict(data, version=version))

    def test_invalid_yaml_is_not_repaired(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / CATALOG_PATH
            path.parent.mkdir()
            path.write_text('vaults: [\n')
            before = path.read_bytes()
            with self.assertRaises(CatalogError):
                load_catalog(root)
            self.assertEqual(path.read_bytes(), before)

    def test_symlink_is_not_a_versioned_catalog(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / CATALOG_PATH
            path.parent.mkdir()
            path.symlink_to(root / 'missing.yaml')
            with self.assertRaises(CatalogError):
                load_catalog(root)


if __name__ == '__main__':
    unittest.main()
