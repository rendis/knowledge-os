#!/usr/bin/env python3
"""Portable capability bindings preserve cell knowledge and local conventions."""
from pathlib import Path
import importlib.util
import sys
import tempfile
import unittest

META = Path(__file__).resolve().parents[2] / 'kernel' / '90-Meta'
sys.path.insert(0, str(META))
from instance import validate_instance, dump_instance, load_instance, InstanceError
spec = importlib.util.spec_from_file_location('cell_config', META / 'cell-config.py')
config = importlib.util.module_from_spec(spec)
spec.loader.exec_module(config)


class Capabilities(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.base = validate_instance({'cell': {'name': 'Cedar', 'purpose': 'Research'}, 'systems': [{'id': 'cedar', 'name': 'Cedar'}], 'evidence': {'profile': 'documented-source'}})
        self.path = self.root / 'instance.yaml'
        self.path.write_text(dump_instance(self.base) + '# keep this cell comment\n')
        self.note = self.root / '60-Operacion' / 'Platform' / 'Cedar - Database reads.md'
        self.note.parent.mkdir(parents=True)
        self.note.write_text('---\ntipo: operacional\n---\nEnvironments: sandbox, canary, live. Executor: SSO tool.\n')

    def test_unconfigured_does_not_infer_a_company_procedure(self):
        before = self.path.read_bytes()
        self.assertEqual(config.resolve(self.root, self.base, 'database-inspection')['status'], 'unconfigured')
        self.assertEqual(self.path.read_bytes(), before)

    def test_binding_roundtrip_preserves_identity_comments_and_notes(self):
        before = self.path.read_text()
        note = self.note.read_bytes()
        config.bind(self.root, 'database-inspection', [self.note.stem])
        self.assertTrue(self.path.read_text().startswith(before))
        loaded = load_instance(self.path)
        self.assertEqual(load_instance(self.path)['evidence']['profile'], 'documented-source')
        self.assertEqual(config.resolve(self.root, loaded, 'database-inspection')['procedures'], ['60-Operacion/Platform/Cedar - Database reads.md'])
        self.assertEqual(load_instance(self.path)['capabilities'], {'database-inspection': [self.note.stem]})
        self.assertEqual(note, self.note.read_bytes())
        config.bind(self.root, 'runtime-inspection', [self.note.stem])
        self.assertEqual(len(load_instance(self.path)['capabilities']), 2)

    def test_rebinding_preserves_section_and_neighbor_comments(self):
        self.path.write_text(self.path.read_text() +
            'capabilities: # capability policy\n'
            '  # runner rationale\n'
            f'  database-inspection: ["{self.note.stem}"] # restricted reads\n'
            '# owner of the following field\nteam-owner: Cedar\n')
        config.bind(self.root, 'runtime-inspection', [self.note.stem])
        updated = self.path.read_text()
        for comment in ['# capability policy', '# runner rationale', '# restricted reads']:
            self.assertIn(comment, updated)
        self.assertTrue(updated.endswith('# owner of the following field\nteam-owner: Cedar\n'))
        self.assertEqual(len(load_instance(self.path)['capabilities']), 2)

    def test_unsupported_capability_syntax_rejects_without_writes(self):
        original = self.path.read_text()
        for section in ['"capabilities": {}\n', 'capabilities: {}\n']:
            self.path.write_text(original + section)
            before = self.path.read_bytes()
            with self.assertRaises(InstanceError):
                config.bind(self.root, 'runtime-inspection', [self.note.stem])
            self.assertEqual(self.path.read_bytes(), before)

    def test_missing_duplicate_and_traversal_reject_without_writes(self):
        before = self.path.read_bytes()
        for name in ['Missing', '../escape']:
            with self.assertRaises(InstanceError):
                config.bind(self.root, 'database-inspection', [name])
        other = self.note.parent.parent / 'Other' / self.note.name
        other.parent.mkdir(); other.write_bytes(self.note.read_bytes())
        with self.assertRaises(InstanceError):
            config.bind(self.root, 'database-inspection', [self.note.stem])
        self.assertEqual(self.path.read_bytes(), before)

    def test_symlinked_procedure_outside_vault_rejected(self):
        with tempfile.TemporaryDirectory() as external:
            target = Path(external) / 'note.md'; target.write_bytes(self.note.read_bytes())
            self.note.unlink(); self.note.symlink_to(target)
            with self.assertRaises(InstanceError):
                config.bind(self.root, 'database-inspection', [self.note.stem])


if __name__ == '__main__':
    unittest.main()
