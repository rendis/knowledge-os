"""Public investigation transaction regressions; no external worktree effects."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

DIST = Path(__file__).resolve().parents[2]
HELPER = DIST / 'kernel/.agents/skills/manage-investigation/scripts/investigation-case.py'

class Transactions(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'investigations'
        self.root.mkdir()

        self.case_id = '20260908-120000-binding'
        self.run_cli('open', '--id', self.case_id, '--title', 'Binding', '--objective', 'Verify binding', '--request-summary', 'Verify one development handoff binding.', '--dedupe-key', 'binding', '--purpose', 'knowledge', '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated')
        self.case = self.root / self.case_id / 'investigation.md'

    def run_cli(self, *args, ok=True):
        result = subprocess.run([sys.executable, '-B', str(HELPER), '--root', str(self.root), *args], capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, ok, result.stdout + result.stderr)
        return result

    def test_bind_retry_advance_and_rollback_preserve_freshness(self):
        (self.case.parent / 'exports' / 'S-001-story.md').write_text(f'---\nstory-id: S-001\nsource-investigation: {self.case_id}\n---\n# Story\n')
        original = self.case.read_text()
        digest = hashlib.sha256(b'delivery|ABC-123|example.invalid/team/repository').hexdigest()
        observation = {'story-id': 'S-001', 'tracker-id': 'delivery', 'provider': 'example', 'tracker-url': 'https://tracker.example.com', 'work-item-reference': 'ABC-123', 'repository-remote': 'example.invalid/team/repository', 'branch': 'issue/abc-123-change', 'handoff-id': digest, 'family': f'delivery-abc-123-{digest[:10]}--repository', 'revision': 'v0001', 'materialized-at': '2026-09-09T12:00:00+00:00'}
        path = Path(self.tmp.name) / 'observation.json'
        def bind(ok=True):
            path.write_text(json.dumps(observation))
            return self.run_cli('bind', '--id', self.case_id, '--observation', str(path), ok=ok)
        bind()
        bound = self.case.read_bytes()
        self.assertIn(next(x for x in original.splitlines() if x.startswith('updated-at:')), self.case.read_text())
        self.assertIn('unchanged', bind().stdout)
        self.assertEqual(bound, self.case.read_bytes())
        observation['revision'] = 'v0003'
        bind(False)
        self.assertEqual(bound, self.case.read_bytes())
        observation['revision'] = 'v0002'
        observation['materialized-at'] = 'invalid'
        bind(False)
        self.assertEqual(bound, self.case.read_bytes())
        observation['materialized-at'] = '2026-09-10T12:00:00+00:00'
        bind()
        self.run_cli('validate')
        self.assertFalse((self.root / '.open.lock').exists())

    def test_knowledge_close_without_story_or_export(self):
        self.case.write_text(self.case.read_text().replace('status: intake', 'status: validating'))
        self.run_cli('close', '--id', self.case_id, '--decision', 'complete', '--reason', 'Question answered', '--limitations', 'No production access')
        self.assertIn('status: closed', self.case.read_text())
        self.assertIn('No production access', self.case.read_text())
        self.run_cli('validate')

    def test_abandoned_intake_needs_no_export_or_assessment(self):
        self.case.write_text(self.case.read_text().replace('vault-outcome: none', 'vault-outcome: not-evaluated'))
        self.run_cli('close', '--id', self.case_id, '--decision', 'abandoned', '--reason', 'User discarded question', '--limitations', 'Not investigated')
        self.assertIn('vault-outcome: none', self.case.read_text())
        self.run_cli('validate')

    def test_coordinated_save_rejects_stale_and_credentials_and_can_delete_private(self):
        public_before = self.case.read_bytes()
        public_candidate = Path(self.tmp.name) / 'public.md'
        public_candidate.write_bytes(public_before.replace(b'### Scope\n', b'### Scope\n\n- Public scope.\n'))
        private_candidate = Path(self.tmp.name) / 'private.md'
        private_candidate.write_text(
            f'''---\nid: {self.case_id}\nauthority: private-overlay\nupdated-at: 2026-09-09T12:00:00+00:00\n---\n\n# Private overlay\n\n## Sensitive context\n\n- Internal participant identity is required for follow-up.\n\n## Private references\n\n- Protected source available to authorized team members.\n\n## History\n\n- 2026-09-09T12:00:00+00:00 — Overlay created.\n'''
        )
        private_root = Path(self.tmp.name) / '.investigations-private'
        initial_hash = hashlib.sha256(public_before).hexdigest()
        self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(public_candidate),
            '--expected-public-sha256', initial_hash,
            '--private-root', str(private_root),
            '--private-candidate', str(private_candidate),
        )
        saved_public = self.case.read_bytes()
        saved_private = (private_root / self.case_id / 'private.md').read_bytes()

        stale = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(public_candidate),
            '--expected-public-sha256', initial_hash,
            '--private-root', str(private_root),
            '--expected-private-sha256', hashlib.sha256(saved_private).hexdigest(),
            ok=False,
        )
        self.assertIn('stale_public_snapshot', stale.stderr)
        self.assertEqual(saved_public, self.case.read_bytes())
        self.assertEqual(saved_private, (private_root / self.case_id / 'private.md').read_bytes())

        invalid_public = Path(self.tmp.name) / 'invalid-public.md'
        invalid_public.write_bytes(saved_public.replace(b'### Scope\n', b'### Scope\n\n- /Users/example/private.md\n'))
        rolled_back = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(invalid_public),
            '--expected-public-sha256', hashlib.sha256(saved_public).hexdigest(),
            '--private-root', str(private_root),
            '--expected-private-sha256', hashlib.sha256(saved_private).hexdigest(),
            '--delete-private',
            ok=False,
        )
        self.assertIn('save_validation_failed', rolled_back.stderr)
        self.assertEqual(saved_public, self.case.read_bytes())
        self.assertEqual(saved_private, (private_root / self.case_id / 'private.md').read_bytes())

        private_candidate.write_text(private_candidate.read_text().replace('Protected source', 'password=synthetic-secret Protected source'))
        rejected = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(public_candidate),
            '--expected-public-sha256', hashlib.sha256(saved_public).hexdigest(),
            '--private-root', str(private_root),
            '--private-candidate', str(private_candidate),
            '--expected-private-sha256', hashlib.sha256(saved_private).hexdigest(),
            ok=False,
        )
        self.assertIn('secret_input', rejected.stderr)
        self.assertEqual(saved_public, self.case.read_bytes())
        self.assertEqual(saved_private, (private_root / self.case_id / 'private.md').read_bytes())

        public_candidate.write_bytes(saved_public)
        self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(public_candidate),
            '--expected-public-sha256', hashlib.sha256(saved_public).hexdigest(),
            '--private-root', str(private_root),
            '--expected-private-sha256', hashlib.sha256(saved_private).hexdigest(),
            '--delete-private',
        )
        self.assertFalse((private_root / self.case_id).exists())

    def test_context_template_is_exported_with_bounded_source_anchor(self):
        contract = (HELPER.parents[1] / 'references/export-contract.md').read_text()
        self.assertIn('development-context-template.md', contract)
        template = (HELPER.parents[1] / 'assets/development-context-template.md').read_text()
        for field in ('Vault remote:', 'Investigation ID:', 'Relevant investigation sections:', 'Relevant linked vault notes:'):
            self.assertIn(field, template)

    def test_validator_rejects_credentials_and_local_paths_anywhere_public(self):
        export = self.case.parent / 'exports' / 'unsafe.md'
        export.write_text('password=synthetic-secret\n', encoding='utf-8')
        invalid = self.run_cli('validate', ok=False)
        self.assertIn('credential-like value', invalid.stderr)
        export.write_text('local source: /Users/example/private/file.md\n', encoding='utf-8')
        invalid = self.run_cli('validate', ok=False)
        self.assertIn('absolute local path', invalid.stderr)

    def test_consolidation_rejects_a_stale_retiring_snapshot(self):
        retired_id = '20260908-120100-binding-duplicate'
        self.run_cli(
            'open', '--id', retired_id, '--title', 'Binding duplicate',
            '--objective', 'Verify duplicate consolidation',
            '--request-summary', 'Consolidate a semantically reviewed duplicate.',
            '--dedupe-key', 'binding-duplicate', '--purpose', 'knowledge',
            '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated',
        )
        mapping = self.case.parent / 'artifacts' / f'consolidation-{retired_id}-mapping.md'
        mapping.write_text(
            f'retired-id: {retired_id}\ndrafts: none\nlearning-assessment: reset\n\nReviewed mapping: no unique registers.\n',
            encoding='utf-8',
        )
        retired = self.root / retired_id / 'investigation.md'
        canonical_hash = hashlib.sha256(self.case.read_bytes()).hexdigest()
        stale_retired_hash = hashlib.sha256(retired.read_bytes()).hexdigest()
        retired.write_text(retired.read_text() + '\n', encoding='utf-8')
        stale = self.run_cli(
            'consolidate', '--canonical', self.case_id, '--retire', retired_id,
            '--expected-canonical-sha256', canonical_hash,
            '--expected-retire-sha256', stale_retired_hash,
            ok=False,
        )
        self.assertIn('stale_retiring_snapshot', stale.stderr)
        self.assertTrue(retired.exists())
        self.assertNotIn(retired_id, self.case.read_text())

if __name__ == '__main__':
    unittest.main()
