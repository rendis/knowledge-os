"""Regressions for read-only graph, evidence scope and closure helpers."""
import contextlib
import importlib.util
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

META = Path(__file__).resolve().parents[2] / 'kernel/90-Meta'
sys.path.insert(0, str(META))


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), META / f'{name}.py')
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


GRAPH = load('graph-query')
SCAN = load('static-evidence-scan')
CLOSURE = load('check-map-closure')


class HelperIntegrityTests(unittest.TestCase):
    def test_hygiene_exposes_hidden_and_duplicate_diagnostics(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'a').mkdir()
            (root / 'b').mkdir()
            (root / 'a/Node.md').write_text('[private](.agents/private.md)\n')
            (root / 'b/Node.md').write_text('# Duplicate\n')
            result = GRAPH.hygiene(root)
            self.assertEqual(result['hidden'], ['a/Node.md -> .agents/private.md'])
            self.assertEqual(len(result['duplicates']), 1)
            with self.assertRaisesRegex(ValueError, 'duplicate note basename.*a/Node.md.*b/Node.md'):
                GRAPH.neighbors(root, 'Node')

    def test_shared_frontmatter_multiline_aliases_and_comments(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'reader.md'
            path.write_text('---\naliases:\n  - APP123-reader # ignored-token\n  - "Readable alias"\ncobertura-api: por-confirmar # review\n---\n# Body\n')
            note = SCAN.read_note(path)
            self.assertEqual(note.aliases, ['APP123-reader', 'Readable alias'])
            self.assertEqual(note.source_repo_name, 'APP123-reader')
            self.assertEqual(note.frontmatter['cobertura-api'], 'por-confirmar')
            self.assertNotIn('ignored-token', SCAN.extract_tokens(note))
            self.assertIn('# Body', note.body)

    def test_tracked_scope_includes_dirty_bytes_excludes_local_artifacts(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            subprocess.run(['git', 'init', '-q', str(repo)], check=True)
            (repo / 'tracked.py').write_text('baseline-token\n')
            (repo / ' tracked.py').write_text('leading-space-evidence-token\n')
            (repo / '.gitignore').write_text('ignored.txt\n')
            subprocess.run(['git', '-C', str(repo), 'add', '.'], check=True)
            subprocess.run(['git', '-C', str(repo), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture'], check=True)
            (repo / 'tracked.py').write_text('dirty-evidence-token\n')
            (repo / 'untracked.py').write_text('local-untracked-token\n')
            (repo / 'ignored.txt').write_text('local-ignored-token\n')
            (repo / 'Dockerfile').write_text('untracked deployment\n')
            (repo / 'link.py').symlink_to(repo / 'tracked.py')
            subprocess.run(['git', '-C', str(repo), 'add', 'link.py'], check=True)
            names = {path.name for path in SCAN.iter_source_files(repo)}
            self.assertEqual(names, {'tracked.py', ' tracked.py', '.gitignore'})
            self.assertEqual(SCAN.deploy_inventory(repo), [])
            source = SCAN.SourceRepo(repo, 'example.invalid/fixture', 'fixture', 'main', 'a' * 40)
            self.assertTrue(SCAN.find_cross_refs(['dirty-evidence-token'], None, 12, [source]))
            leading_space_hits = SCAN.find_cross_refs(['leading-space-evidence-token'], None, 12, [source])
            self.assertEqual(leading_space_hits, [f'{repo.name}/ tracked.py: leading-space-evidence-token'])
            self.assertFalse(SCAN.find_cross_refs(['local-untracked-token'], None, 12, [source]))
            self.assertFalse(SCAN.find_cross_refs(['local-ignored-token'], None, 12, [source]))
            original = SCAN.ROOT
            try:
                SCAN.ROOT = repo
                note = SCAN.Note(repo / 'fixture.md', 'fixture', {'aliases': ['APP-fixture']}, '')
                with contextlib.redirect_stdout(io.StringIO()) as output:
                    SCAN.report_note(note, 12, [SCAN.SourceRepo(repo, source.identity, 'APP-fixture', source.branch, source.commit)])
                self.assertIn('incluye cambios sin commit', output.getvalue())
                self.assertIn('no identifica los bytes escaneados', output.getvalue())
            finally:
                SCAN.ROOT = original

    def test_tracked_file_under_symlinked_directory_cannot_escape_repo(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            repo = base / 'repo'
            repo.mkdir()
            outside = base / 'outside'
            outside.mkdir()
            subprocess.run(['git', 'init', '-q', str(repo)], check=True)
            (repo / 'dir').mkdir()
            tracked = repo / 'dir/file.py'
            tracked.write_text('tracked evidence\n')
            subprocess.run(['git', '-C', str(repo), 'add', '.'], check=True)
            tracked.unlink()
            (repo / 'dir').rmdir()
            (outside / 'file.py').write_text('external-evidence-token\n')
            (repo / 'dir').symlink_to(outside, target_is_directory=True)
            self.assertEqual(SCAN.iter_source_files(repo), [])
            source = SCAN.SourceRepo(repo, 'example.invalid/fixture', 'fixture', 'main', 'a' * 40)
            self.assertEqual(SCAN.find_cross_refs(['external-evidence-token'], None, 12, [source]), [])

    def test_closure_requires_distinct_summary_paths(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            text = 'remaining: 0 verification items in 0 notes\n'
            (root / '00-Home.md').write_text(text)
            (root / 'Coverage.md').write_text(text)
            checkpoint = root / 'checkpoint.json'
            for paths in [['00-Home.md', '00-Home.md'], ['00-Home.md', './00-Home.md']]:
                checkpoint.write_text(json.dumps({'visible_coverage': {
                    'remaining_verification_items': 0, 'notes_with_verifications': 0, 'paths': paths,
                }}))
                self.assertEqual(CLOSURE.check(root, checkpoint)['status'], 'blocked')
            checkpoint.write_text(json.dumps({'visible_coverage': {
                'remaining_verification_items': 0, 'notes_with_verifications': 0,
                'paths': ['00-Home.md', 'Coverage.md'],
            }}))
            self.assertEqual(CLOSURE.check(root, checkpoint)['status'], 'pass')


if __name__ == '__main__':
    unittest.main()
