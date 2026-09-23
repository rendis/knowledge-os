"""Public investigation transaction regressions; no external worktree effects."""
import hashlib
import json
from pathlib import Path
import shutil
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
        self.unpublished = Path(self.tmp.name) / '.investigations'
        subprocess.run(['git', 'init', '-q', self.tmp.name], check=True)
        subprocess.run(['git', '-C', self.tmp.name, 'config', 'user.name', 'Test Recorder'], check=True)
        subprocess.run(['git', '-C', self.tmp.name, 'config', 'user.email', 'recorder@example.invalid'], check=True)

        self.case_id = '20260908-120000-binding'
        self.run_cli('open', '--id', self.case_id, '--title', 'Binding', '--objective', 'Verify binding', '--request-summary', 'Verify one development handoff binding.', '--dedupe-key', 'binding', '--purpose', 'knowledge', '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated')
        self.case = self.unpublished / self.case_id / 'investigation.md'

    def run_cli(self, *args, ok=True):
        result = subprocess.run([sys.executable, '-B', str(HELPER), '--root', str(self.root), *args], capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, ok, result.stdout + result.stderr)
        return result

    def tree_sha(self, case_dir=None):
        return json.loads(self.run_cli('snapshot', '--case-dir', str(case_dir or self.case.parent)).stdout)['tree_sha256']

    def test_bind_retry_advance_and_rollback_preserve_freshness(self):
        (self.case.parent / 'exports' / 'S-001-story.md').write_text(f'---\nstory-id: S-001\nsource-investigation: {self.case_id}\n---\n# Story\n')
        original = self.case.read_text()
        digest = hashlib.sha256(b'delivery|ABC-123|example.invalid/team/repository').hexdigest()
        observation = {'story-id': 'S-001', 'tracker-id': 'delivery', 'provider': 'example', 'tracker-url': 'https://tracker.example.com', 'work-item-reference': 'ABC-123', 'repository-remote': 'example.invalid/team/repository', 'branch': 'issue/abc-123-change', 'handoff-id': digest, 'family': f'delivery-abc-123-{digest[:10]}--repository', 'revision': 'v0001', 'materialized-at': '2026-09-09T12:00:00+00:00'}
        path = Path(self.tmp.name) / 'observation.json'
        def bind(ok=True, public_sha=None, story_sha=None):
            path.write_text(json.dumps(observation))
            return self.run_cli(
                'bind', '--id', self.case_id, '--observation', str(path),
                '--expected-public-sha256', public_sha or hashlib.sha256(self.case.read_bytes()).hexdigest(),
                '--expected-story-sha256', story_sha or hashlib.sha256((self.case.parent / 'exports' / 'S-001-story.md').read_bytes()).hexdigest(),
                ok=ok,
            )
        public_sha = hashlib.sha256(self.case.read_bytes()).hexdigest()
        self.case.write_text(self.case.read_text().replace('### Facts\n', '### Facts\n\n- E-001: Scope changed after review.\n'))
        changed = self.case.read_bytes()
        self.assertIn('stale_public_snapshot', bind(False, public_sha=public_sha).stderr)
        self.assertEqual(changed, self.case.read_bytes())
        story = self.case.parent / 'exports' / 'S-001-story.md'
        story_sha = hashlib.sha256(story.read_bytes()).hexdigest()
        story.write_text(story.read_text() + '\nChanged implementation scope.\n')
        self.assertIn('stale_story_snapshot', bind(False, story_sha=story_sha).stderr)
        self.assertEqual(changed, self.case.read_bytes())
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
        self.assertFalse((self.unpublished / '.open.lock').exists())

    def test_reconciliation_content_then_lifecycle_uses_fresh_snapshot(self):
        before = self.case.read_bytes()
        candidate = Path(self.tmp.name) / 'reconciled.md'
        candidate.write_bytes(before.replace(b'### Facts\n', b'### Facts\n\n- E-001: Both contributions require the missing source.\n'))
        self.run_cli(
            'save', '--id', self.case_id, '--public-candidate', str(candidate),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--source', 'common-base b1; current c1; contribution c2', '--target', 'E-001',
        )
        saved = self.case.read_bytes()
        def block(digest, ok=True):
            return self.run_cli(
                'transition', '--id', self.case_id, '--to', 'blocked',
                '--blocked-on', 'Required source unavailable',
                '--reason', 'Reviewed lifecycle disposition from c1 and c2',
                '--source', 'common-base b1; current c1; contribution c2',
                '--expected-public-sha256', digest, ok=ok,
            )
        self.assertIn('stale_public_snapshot', block(hashlib.sha256(before).hexdigest(), False).stderr)
        self.assertEqual(saved, self.case.read_bytes())
        block(hashlib.sha256(saved).hexdigest())
        self.assertIn('status: blocked', self.case.read_text())
        self.assertIn('E-001: Both contributions', self.case.read_text())
        self.run_cli('validate')

    def test_knowledge_close_without_story_or_export(self):
        before = self.case.read_bytes()
        candidate = Path(self.tmp.name) / 'knowledge-complete.md'
        candidate.write_bytes(before.replace(
            b'### Facts\n',
            b'### Facts\n\n- E-001 (fact) - The inspected contract answers the stated question.\n',
        ))
        self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic contract fixture', '--target', 'E-001',
        )
        current = self.case.read_bytes()
        self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Question answered', '--limitations', 'No production access',
            '--source', 'synthetic closure review', '--evidence', 'E-001',
            '--expected-public-sha256', hashlib.sha256(current).hexdigest(),
        )
        text = self.case.read_text()
        self.assertIn('status: closed', text)
        self.assertIn('closure-outcome: completed', text)
        self.assertIn('No production access', text)
        self.assertIn('evidence: `E-001`', text)
        self.run_cli('validate')

    def test_abandoned_investigation_needs_no_export_or_assessment(self):
        self.case.write_text(self.case.read_text().replace('vault-outcome: none', 'vault-outcome: not-evaluated'))
        before = self.case.read_bytes()
        self.run_cli(
            'close', '--id', self.case_id, '--decision', 'abandoned',
            '--reason', 'User discarded question', '--limitations', 'Not investigated',
            '--source', 'synthetic abandonment decision',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
        )
        text = self.case.read_text()
        self.assertIn('vault-outcome: none', text)
        self.assertIn('closure-outcome: abandoned', text)
        self.run_cli('validate')

    def test_lifecycle_transitions_are_traced_cas_guarded_and_reopen_cleanly(self):
        initial = self.case.read_bytes()
        blocked = self.run_cli(
            'transition', '--id', self.case_id, '--to', 'blocked',
            '--blocked-on', 'carrier API: access denied',
            '--reason', 'The required source is unavailable',
            '--source', 'synthetic access check',
            '--expected-public-sha256', hashlib.sha256(initial).hexdigest(),
            '--timestamp', '2026-09-10T10:00:00+00:00',
        )
        self.assertIn('"to": "blocked"', blocked.stdout)
        blocked_bytes = self.case.read_bytes()
        blocked_text = blocked_bytes.decode()
        self.assertIn('status: blocked', blocked_text)
        self.assertIn('blocked-on: "carrier API: access denied"', blocked_text)
        self.assertNotIn('resume-to:', blocked_text)
        self.assertIn('recorded by Test Recorder <recorder@example.invalid>', blocked_text)

        stale = self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Access restored', '--source', 'synthetic access check',
            '--expected-public-sha256', hashlib.sha256(initial).hexdigest(),
            ok=False,
        )
        self.assertIn('stale_public_snapshot', stale.stderr)
        self.assertEqual(blocked_bytes, self.case.read_bytes())

        self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Access restored', '--source', 'synthetic access check',
            '--expected-public-sha256', hashlib.sha256(blocked_bytes).hexdigest(),
            '--timestamp', '2026-09-10T10:05:00+00:00',
        )
        active = self.case.read_bytes()
        self.assertIn(b'status: investigating', active)
        self.assertNotIn(b'blocked-on:', active)

        candidate = Path(self.tmp.name) / 'lifecycle-evidence.md'
        candidate.write_bytes(active.replace(
            b'### Facts\n',
            b'### Facts\n\n- E-001 (fact) - The restored source resolves the objective.\n',
        ))
        self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(active).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic restored contract', '--target', 'E-001',
        )
        resolved = self.case.read_bytes()
        self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Objective verified', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'E-001',
            '--expected-public-sha256', hashlib.sha256(resolved).hexdigest(),
            '--timestamp', '2026-09-10T10:10:00+00:00',
        )
        closed = self.case.read_bytes()
        self.assertIn(b'closure-outcome: completed', closed)

        self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'New evidence changes the prior conclusion',
            '--source', 'synthetic follow-up report',
            '--expected-public-sha256', hashlib.sha256(closed).hexdigest(),
            '--timestamp', '2026-09-10T10:15:00+00:00',
        )
        reopened = self.case.read_text()
        self.assertIn('status: investigating', reopened)
        self.assertNotIn('closure-outcome:', reopened)
        self.assertIn('closed as `completed`', reopened)
        self.assertIn('Investigation reopened', reopened)
        self.run_cli('validate')

    def test_save_cannot_bypass_lifecycle_commands(self):
        before = self.case.read_bytes()
        candidate = Path(self.tmp.name) / 'bypass.md'
        candidate.write_bytes(before.replace(b'status: investigating', b'status: blocked'))
        rejected = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic bypass attempt', '--target', 'Q-001',
            ok=False,
        )
        self.assertIn('status_transition_required', rejected.stderr)
        self.assertEqual(before, self.case.read_bytes())

        duplicate = Path(self.tmp.name) / 'duplicate-status.md'
        duplicate.write_bytes(before.replace(
            b'status: investigating',
            b'status: blocked\nstatus: investigating',
        ))
        rejected_duplicate = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(duplicate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic duplicate-key attempt', '--target', 'Q-001',
            ok=False,
        )
        self.assertIn('frontmatter repeats status', rejected_duplicate.stderr)
        self.assertEqual(before, self.case.read_bytes())

        semantic_duplicate = Path(self.tmp.name) / 'semantic-duplicate-status.md'
        semantic_duplicate.write_bytes(before.replace(
            b'status: investigating',
            b'status: investigating\nstatus : closed',
        ))
        rejected_semantic_duplicate = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(semantic_duplicate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic YAML key ambiguity attempt', '--target', 'Q-001',
            ok=False,
        )
        self.assertIn('canonical unquoted key', rejected_semantic_duplicate.stderr)
        self.assertEqual(before, self.case.read_bytes())

    def test_close_rejects_history_only_evidence_reference(self):
        before = self.case.read_bytes()
        history_only = before.decode().replace(
            '## History\n',
            '## History\n\n- E-999: fabricated History-only evidence reference.\n',
        )
        candidate = Path(self.tmp.name) / 'history-only-target.md'
        candidate.write_text(history_only)
        rejected_save = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic History-only target', '--target', 'E-999',
            ok=False,
        )
        self.assertIn('Every affected register ID must exist', rejected_save.stderr)
        self.assertEqual(before, self.case.read_bytes())

        self.case.write_text(history_only)
        current = self.case.read_bytes()
        rejected = self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Unsupported closure', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'E-999',
            '--expected-public-sha256', hashlib.sha256(current).hexdigest(),
            ok=False,
        )
        self.assertIn('existing register IDs', rejected.stderr)
        self.assertEqual(current, self.case.read_bytes())

    def test_story_evidence_must_belong_to_the_same_investigation(self):
        export = self.case.parent / 'exports' / 'S-001-story.md'
        export.write_text(
            '---\nstory-id: S-001\nsource-investigation: another-case\n---\n# Story\n',
            encoding='utf-8',
        )
        before = self.case.read_bytes()
        rejected = self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Story prepared', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'S-001',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            ok=False,
        )
        self.assertIn('existing register IDs', rejected.stderr)
        self.assertEqual(before, self.case.read_bytes())

        export.write_text(
            f'---\nstory-id: S-001\nsource-investigation: {self.case_id}\n'
            'source-investigation : another-case\n---\n# Story\n',
            encoding='utf-8',
        )
        ambiguous = self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Story prepared', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'S-001',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            ok=False,
        )
        self.assertIn('existing register IDs', ambiguous.stderr)
        self.assertEqual(before, self.case.read_bytes())

        export.write_text(
            f'---\nstory-id: S-001\nsource-investigation: {self.case_id}\n---\n# Story\n',
            encoding='utf-8',
        )
        self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Story prepared', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'S-001',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
        )
        self.assertIn('closure-outcome: completed', self.case.read_text())

    def test_legacy_states_and_invalid_lifecycle_metadata_are_rejected(self):
        original = self.case.read_bytes()
        for replacement, expected in (
            (b'status: intake', 'invalid status'),
            (b'status: closed\nclosure-outcome: unexpected', 'valid closure-outcome'),
            (b'status: investigating\nresume-to: investigating', 'resume-to is legacy'),
        ):
            self.case.write_bytes(original.replace(b'status: investigating', replacement))
            invalid = self.run_cli('validate', ok=False)
            self.assertIn(expected, invalid.stderr)
        self.case.write_bytes(original)

    def test_prior_public_statuses_can_be_converted_case_by_case(self):
        second_id = '20260908-120150-prior-public'
        self.run_cli(
            'open', '--id', second_id, '--title', 'Prior public case',
            '--objective', 'Convert the prior public lifecycle',
            '--request-summary', 'Convert a previously versioned public case.',
            '--dedupe-key', 'prior-public-case', '--purpose', 'development',
            '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated',
        )
        second = self.unpublished / second_id / 'investigation.md'
        self.case.write_text(self.case.read_text().replace('status: investigating', 'status: intake', 1))
        second.write_text(second.read_text().replace('status: investigating', 'status: ready-to-export', 1))

        first_before = self.case.read_bytes()
        first = self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Adopt the three-state lifecycle',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(first_before).hexdigest(),
        )
        self.assertIn('"migration": true', first.stdout)
        self.assertIn('pre-existing case errors remain', first.stdout)
        self.assertIn('status: investigating', self.case.read_text())
        self.assertIn('Migrated prior state `intake`', self.case.read_text())

        second_before = second.read_bytes()
        converted = self.run_cli(
            'transition', '--id', second_id, '--to', 'investigating',
            '--reason', 'Adopt the three-state lifecycle',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(second_before).hexdigest(),
        )
        self.assertIn('"migration": true', converted.stdout)
        self.assertIn('Migrated prior state `ready-to-export`', second.read_text())
        self.run_cli('validate')

    def test_close_cannot_bypass_prior_public_lifecycle_migration(self):
        original = self.case.read_bytes()
        self.case.write_bytes(original.replace(
            b'status: investigating', b'status: validating', 1
        ))
        legacy = self.case.read_bytes()
        rejected = self.run_cli(
            'close', '--id', self.case_id, '--decision', 'complete',
            '--reason', 'Objective verified', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'E-001',
            '--expected-public-sha256', hashlib.sha256(legacy).hexdigest(),
            ok=False,
        )
        self.assertIn('must be migrated to investigating', rejected.stderr)
        self.assertEqual(legacy, self.case.read_bytes())

        self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Adopt the three-state lifecycle',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(legacy).hexdigest(),
        )
        self.assertIn('Migrated prior state `validating`', self.case.read_text())

        closed_before = self.case.read_bytes()
        self.case.write_bytes(closed_before.replace(
            b'status: investigating', b'status: closed', 1
        ))
        prior_closed = self.case.read_bytes()
        self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Prior closure outcome requires explicit review',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(prior_closed).hexdigest(),
        )
        self.assertIn('Migrated prior state `closed`', self.case.read_text())
        self.run_cli('validate')

        self.case.write_text(self.case.read_text().replace(
            'status: investigating',
            'status: investigating\nresume-to: validating',
            1,
        ))
        obsolete = self.case.read_bytes()
        rejected = self.run_cli(
            'transition', '--id', self.case_id, '--to', 'blocked',
            '--blocked-on', 'synthetic dependency',
            '--reason', 'Attempted non-conversion transition',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(obsolete).hexdigest(),
            ok=False,
        )
        self.assertIn('conversion must end in investigating', rejected.stderr)
        self.assertEqual(obsolete, self.case.read_bytes())
        self.run_cli(
            'transition', '--id', self.case_id, '--to', 'investigating',
            '--reason', 'Remove obsolete lifecycle metadata',
            '--source', 'requested public lifecycle migration',
            '--expected-public-sha256', hashlib.sha256(obsolete).hexdigest(),
        )
        self.assertNotIn('resume-to:', self.case.read_text())
        self.run_cli('validate')

    def test_undecided_purpose_is_active_but_cannot_complete(self):
        undecided_id = '20260908-120200-undecided'
        self.run_cli(
            'open', '--id', undecided_id, '--title', 'Undecided case',
            '--objective', 'Classify the requested outcome',
            '--request-summary', 'Determine whether the outcome is knowledge or development.',
            '--dedupe-key', 'undecided-case', '--purpose', 'undecided',
            '--vault-outcome', 'not-evaluated', '--learning-outcome', 'not-evaluated',
        )
        path = self.unpublished / undecided_id / 'investigation.md'
        self.assertIn('status: investigating', path.read_text())
        before = path.read_bytes()
        rejected = self.run_cli(
            'close', '--id', undecided_id, '--decision', 'complete',
            '--reason', 'Premature completion', '--limitations', 'none',
            '--source', 'synthetic closure review', '--evidence', 'E-001',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            ok=False,
        )
        self.assertIn('resolved purpose', rejected.stderr)
        self.assertEqual(before, path.read_bytes())
        self.run_cli(
            'close', '--id', undecided_id, '--decision', 'abandoned',
            '--reason', 'Requester discontinued classification',
            '--limitations', 'The intended outcome remains undecided',
            '--source', 'synthetic abandonment decision',
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
        )
        closed = path.read_text()
        self.assertIn('purpose: undecided', closed)
        self.assertIn('closure-outcome: abandoned', closed)

    def test_development_and_mixed_close_against_objective_without_export(self):
        for offset, purpose in enumerate(('development', 'mixed'), start=3):
            case_id = f'20260908-120{offset}00-{purpose}-scope'
            self.run_cli(
                'open', '--id', case_id, '--title', f'{purpose.title()} scope',
                '--objective', 'Produce an agreed implementation specification',
                '--request-summary', 'Define the implementation boundary; implementation is out of scope.',
                '--dedupe-key', f'{purpose}-scope', '--purpose', purpose,
                '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated',
            )
            path = self.unpublished / case_id / 'investigation.md'
            before = path.read_bytes()
            candidate = Path(self.tmp.name) / f'{purpose}-scope.md'
            candidate.write_bytes(before.replace(
                b'## Acceptance criteria\n',
                b'## Acceptance criteria\n\n- AC-001 (met) - The agreed repository boundary and verification behavior are explicit.\n',
            ))
            self.run_cli(
                'save', '--id', case_id,
                '--public-candidate', str(candidate),
                '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
                '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
                '--source', 'synthetic specification review', '--target', 'AC-001',
            )
            current = path.read_bytes()
            self.run_cli(
                'close', '--id', case_id, '--decision', 'complete',
                '--reason', 'The agreed specification objective is verified',
                '--limitations', 'Implementation and deployment are outside scope',
                '--source', 'synthetic specification acceptance', '--evidence', 'AC-001',
                '--expected-public-sha256', hashlib.sha256(current).hexdigest(),
            )
            self.assertIn('closure-outcome: completed', path.read_text())
            self.assertFalse(any((path.parent / 'exports').iterdir()))
        self.run_cli('validate')

    def test_coordinated_save_rejects_stale_and_credentials_and_can_delete_private(self):
        public_before = self.case.read_bytes()
        public_candidate = Path(self.tmp.name) / 'public.md'
        public_candidate.write_bytes(
            public_before.replace(b'### Scope\n', b'### Scope\n\n- Public scope.\n')
            .replace(b'## Open questions\n', b'## Open questions\n\n- Q-001 (open) - Confirm the private follow-up.\n')
        )
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
            '--source', 'synthetic mixed-input fixture', '--target', 'Q-001', '--private-target', 'Q-001',
        )
        saved_public = self.case.read_bytes()
        saved_private = (private_root / self.case_id / 'private.md').read_bytes()

        stale = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(public_candidate),
            '--expected-public-sha256', initial_hash,
            '--private-root', str(private_root),
            '--expected-private-sha256', hashlib.sha256(saved_private).hexdigest(),
            '--source', 'synthetic repeated fixture', '--target', 'Q-001',
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
            '--source', 'synthetic invalid public fixture', '--target', 'Q-001', '--private-target', 'Q-001',
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
            '--source', 'synthetic secret fixture', '--target', 'Q-001', '--private-target', 'Q-001',
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
            '--source', 'synthetic overlay removal', '--private-target', 'Q-001',
        )
        self.assertFalse((private_root / self.case_id).exists())

    def test_context_template_is_exported_with_bounded_source_anchor(self):
        contract = (HELPER.parents[1] / 'references/export-contract.md').read_text()
        self.assertIn('development-context-template.md', contract)
        template = (HELPER.parents[1] / 'assets/development-context-template.md').read_text()
        for field in ('Vault remote:', 'Investigation ID:', 'Relevant investigation sections:', 'Relevant linked vault notes:'):
            self.assertIn(field, template)

    def test_load_discovers_private_overlay_by_id_and_reports_absence(self):
        without_overlay = self.run_cli('load', '--id', self.case_id)
        payload = json.loads(without_overlay.stdout)
        self.assertFalse(payload['private']['available'])
        self.assertEqual(payload['private']['sha256'], 'absent')

        private_root = Path(self.tmp.name) / '.investigations-private'
        private_path = private_root / self.case_id / 'private.md'
        private_path.parent.mkdir(parents=True)
        private_path.write_text(
            f'''---\nid: {self.case_id}\nauthority: private-overlay\nupdated-at: 2026-09-09T12:00:00+00:00\n---\n\n# Private overlay\n\n## Sensitive context\n\n- Restricted participant mapping.\n\n## Private references\n\n- Protected register.\n\n## History\n\n- 2026-09-09T12:00:00+00:00 — Imported with unknown historical recorder.\n''',
            encoding='utf-8',
        )
        loaded = json.loads(self.run_cli('load', '--id', self.case_id).stdout)
        self.assertTrue(loaded['private']['available'])
        self.assertEqual(Path(loaded['private']['path']).resolve(), private_path.resolve())
        self.assertEqual(loaded['private']['sha256'], hashlib.sha256(private_path.read_bytes()).hexdigest())

        private_path.write_text(private_path.read_text().replace('Protected register.', 'token=synthetic-load-secret'))
        rejected = self.run_cli('load', '--id', self.case_id, ok=False)
        self.assertIn('private_validation_failed', rejected.stderr)

    def test_two_git_identities_are_traced_and_exact_repeat_is_noop(self):
        original = self.case.read_bytes()
        self.assertIn(b'recorded by Test Recorder <recorder@example.invalid>', original)

        subprocess.run(['git', '-C', self.tmp.name, 'config', 'user.name', 'Second Recorder'], check=True)
        subprocess.run(['git', '-C', self.tmp.name, 'config', 'user.email', 'second@example.invalid'], check=True)
        candidate = Path(self.tmp.name) / 'second.md'
        candidate.write_bytes(
            original.replace(
                b'## Open questions\n',
                b'## Open questions\n\n- Q-001 (open) - Confirm the observed discrepancy.\n',
            )
        )
        saved = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(original).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic operator report', '--target', 'Q-001',
            '--timestamp', '2026-09-10T10:00:00+00:00',
        )
        saved_bytes = self.case.read_bytes()
        self.assertIn(b'recorded by Test Recorder <recorder@example.invalid>', saved_bytes)
        self.assertIn(b'recorded by Second Recorder <second@example.invalid>', saved_bytes)
        self.assertIn(b'Updated registers `Q-001`', saved_bytes)
        self.assertNotIn(b'approved by Second Recorder', saved_bytes)

        candidate.write_bytes(saved_bytes)
        repeated = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(saved_bytes).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'synthetic operator report', '--target', 'Q-001',
        )
        self.assertIn('unchanged', repeated.stdout)
        self.assertEqual(saved_bytes, self.case.read_bytes())

    def test_missing_git_identity_stops_open_without_creating_case(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'investigations'
            root.mkdir()
            subprocess.run(['git', 'init', '-q', tmp], check=True)
            subprocess.run(['git', '-C', tmp, 'config', 'user.name', ''], check=True)
            subprocess.run(['git', '-C', tmp, 'config', 'user.email', ''], check=True)
            result = subprocess.run(
                [sys.executable, '-B', str(HELPER), '--root', str(root), 'open',
                 '--id', '20260910-120000-no-identity', '--title', 'No identity',
                 '--objective', 'Verify the write gate', '--request-summary', 'Verify the write gate.',
                 '--dedupe-key', 'no-identity', '--purpose', 'knowledge',
                 '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated'],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('git_identity_missing', result.stderr)
            self.assertEqual(list(root.iterdir()), [])

    def test_spanish_note_locale_localizes_new_case_and_attribution(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'investigations'
            root.mkdir()
            (Path(tmp) / 'instance.yaml').write_text('locale:\n  notes: es\n', encoding='utf-8')
            subprocess.run(['git', 'init', '-q', tmp], check=True)
            subprocess.run(['git', '-C', tmp, 'config', 'user.name', 'Registrador'], check=True)
            subprocess.run(['git', '-C', tmp, 'config', 'user.email', 'registrador@example.invalid'], check=True)
            result = subprocess.run(
                [sys.executable, '-B', str(HELPER), '--root', str(root), 'open',
                 '--id', '20260910-120000-spanish-case', '--title', 'Caso en español',
                 '--objective', 'Verificar el idioma', '--request-summary', 'Validar el formato localizado.',
                 '--dedupe-key', 'spanish-case', '--purpose', 'knowledge',
                 '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated',
                 '--source-ref', 'prueba localizada'],
                capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            text = (Path(tmp) / '.investigations' / '20260910-120000-spanish-case' / 'investigation.md').read_text()
            self.assertIn('## Resumen de la solicitud', text)
            self.assertIn('## Historial', text)
            self.assertIn('Expediente creado; registrado por Registrador', text)
            self.assertNotIn('## Request summary', text)

    def test_naive_timestamps_are_rejected_without_writes(self):
        before = self.case.read_bytes()
        candidate = Path(self.tmp.name) / 'naive-save.md'
        candidate.write_bytes(
            before.replace(
                b'## Open questions\n',
                b'## Open questions\n\n- Q-001 (open) - Confirm timestamp handling.\n',
            )
        )
        rejected_save = self.run_cli(
            'save', '--id', self.case_id,
            '--public-candidate', str(candidate),
            '--expected-public-sha256', hashlib.sha256(before).hexdigest(),
            '--private-root', str(Path(self.tmp.name) / '.investigations-private'),
            '--source', 'timestamp fixture', '--target', 'Q-001',
            '--timestamp', '2026-09-14T10:00:00',
            ok=False,
        )
        self.assertIn('invalid_timestamp', rejected_save.stderr)
        self.assertEqual(before, self.case.read_bytes())

        rejected_open = self.run_cli(
            'open', '--id', '20260914-100000-naive-open', '--title', 'Naive open',
            '--objective', 'Reject a naive timestamp', '--request-summary', 'Reject a naive timestamp.',
            '--dedupe-key', 'naive-open', '--purpose', 'knowledge',
            '--vault-outcome', 'none', '--learning-outcome', 'not-evaluated',
            '--timestamp', '2026-09-14T10:00:00',
            ok=False,
        )
        self.assertIn('invalid_timestamp', rejected_open.stderr)
        self.assertFalse((self.root / '20260914-100000-naive-open').exists())
        self.assertFalse((self.unpublished / '20260914-100000-naive-open').exists())

        self.case.write_bytes(before.replace(
            b'updated-at: ', b'updated-at: 2026-09-14T10:00:00\nlegacy-updated-at: ', 1
        ))
        invalid = self.run_cli('validate', ok=False)
        self.assertIn('updated-at must be an ISO-8601 timestamp with offset', invalid.stderr)

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
        retired = self.unpublished / retired_id / 'investigation.md'
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

    def test_open_defaults_to_unpublished_and_publish_moves_the_case(self):
        listed = json.loads(self.run_cli('list').stdout)
        self.assertEqual(listed['cases'][0]['visibility'], 'unpublished')
        loaded = json.loads(self.run_cli('load', '--id', self.case_id).stdout)
        self.assertEqual(loaded['visibility'], 'unpublished')
        published = self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', loaded['public']['tree_sha256'],
            '--source', 'synthetic publish review',
        )
        self.assertIn('published', published.stdout)
        self.assertFalse(self.case.exists())
        public = self.root / self.case_id / 'investigation.md'
        self.assertTrue(public.is_file())
        self.assertIn('Published investigation', public.read_text())
        listed = json.loads(self.run_cli('list').stdout)
        self.assertEqual(listed['cases'][0]['visibility'], 'published')

    def test_publish_rejects_unshareable_content_even_with_reviewed_name(self):
        dump = self.case.parent / 'artifacts' / 'yaak-collection.json'
        dump.write_text('{"path": "/Users/example/local-only"}\n', encoding='utf-8')
        rejected = self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', self.tree_sha(),
            '--source', 'synthetic publish review',
            ok=False,
        )
        self.assertIn('publish_unsafe', rejected.stderr)
        self.assertTrue(self.case.exists())
        self.assertFalse((self.root / self.case_id).exists())

    def test_publish_binds_resource_tree_and_retains_excluded_bytes(self):
        method = self.case.parent / 'artifacts' / 'A-001-smoke-check.py'
        method.write_text('print("portable verifier")\n', encoding='utf-8')
        local = self.case.parent / 'artifacts' / 'scratch.txt'
        local.write_text('local draft\n', encoding='utf-8')
        reviewed = self.tree_sha()
        method.write_text('print("changed after review")\n', encoding='utf-8')
        stale = self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', reviewed,
            '--retain-local', 'artifacts/scratch.txt',
            '--source', 'synthetic resource review', ok=False,
        )
        self.assertIn('stale_case_tree', stale.stderr)
        self.assertTrue(self.case.exists())
        self.assertFalse((self.root / self.case_id).exists())
        reviewed = self.tree_sha()
        self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', reviewed,
            '--retain-local', 'artifacts/scratch.txt',
            '--source', 'synthetic resource review',
        )
        self.assertEqual((self.root / self.case_id / 'artifacts' / method.name).read_text(), 'print("changed after review")\n')
        self.assertFalse((self.root / self.case_id / 'artifacts' / 'scratch.txt').exists())
        retained = Path(self.tmp.name) / '.investigations-private' / self.case_id / 'local' / 'publish-retained' / reviewed / 'artifacts' / 'scratch.txt'
        self.assertEqual(retained.read_bytes(), b'local draft\n')

    def test_save_resources_is_scoped_and_rolls_back_invalid_candidate(self):
        self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', self.tree_sha(),
            '--source', 'synthetic initial publication',
        )
        live = self.root / self.case_id
        candidate = Path(self.tmp.name) / 'candidate'
        shutil.copytree(live, candidate)
        note = candidate / 'investigation.md'
        note.write_text(note.read_text().replace('## Evidence', '- A-001 — Reviewed case method at `artifacts/A-001-method/check.py`.\n\n## Evidence'))
        method = candidate / 'artifacts' / 'A-001-method' / 'check.py'
        method.parent.mkdir()
        method.write_text('print("reviewed")\n')
        live_digest = self.tree_sha(live)
        candidate_digest = self.tree_sha(candidate)
        method.write_text('print("changed after review")\n')
        stale = self.run_cli(
            'save-resources', '--id', self.case_id, '--candidate-dir', str(candidate),
            '--expected-tree-sha256', live_digest,
            '--expected-candidate-tree-sha256', candidate_digest,
            '--target', 'A-001', '--source', 'synthetic method review', ok=False,
        )
        self.assertIn('stale_candidate_tree', stale.stderr)
        self.assertFalse((live / 'artifacts' / 'A-001-method').exists())
        method.write_text('print("reviewed")\n')
        candidate_digest = self.tree_sha(candidate)
        scope_file = candidate / 'exports' / 'unreviewed.txt'
        scope_file.write_text('wrong scope\n')
        invalid = self.run_cli(
            'save-resources', '--id', self.case_id, '--candidate-dir', str(candidate),
            '--expected-tree-sha256', live_digest,
            '--expected-candidate-tree-sha256', self.tree_sha(candidate),
            '--target', 'A-001', '--source', 'synthetic method review', ok=False,
        )
        self.assertIn('resource_scope_invalid', invalid.stderr)
        scope_file.unlink()
        note.write_text(note.read_text().replace('## Evidence\n', '## Evidence\n\n- E-001 — Unreviewed collateral change.\n'))
        collateral = self.run_cli(
            'save-resources', '--id', self.case_id, '--candidate-dir', str(candidate),
            '--expected-tree-sha256', live_digest,
            '--expected-candidate-tree-sha256', self.tree_sha(candidate),
            '--target', 'A-001', '--source', 'synthetic method review', ok=False,
        )
        self.assertIn('resource_scope_invalid', collateral.stderr)
        self.assertNotIn('Unreviewed collateral change', (live / 'investigation.md').read_text())
        note.write_text(note.read_text().replace('\n- E-001 — Unreviewed collateral change.\n', ''))
        candidate_digest = self.tree_sha(candidate)
        result = self.run_cli(
            'save-resources', '--id', self.case_id, '--candidate-dir', str(candidate),
            '--expected-tree-sha256', live_digest,
            '--expected-candidate-tree-sha256', candidate_digest,
            '--target', 'A-001', '--source', 'synthetic method review',
        )
        self.assertIn('resources_saved', result.stdout)
        self.assertEqual((live / 'artifacts' / 'A-001-method' / 'check.py').read_text(), 'print("reviewed")\n')
        self.assertIn('A-001', (live / 'investigation.md').read_text())
        self.assertFalse((live / 'investigation.md').read_bytes().endswith(b'\n\n'))
        self.assertIn('stale_case_tree', self.run_cli(
            'save-resources', '--id', self.case_id, '--candidate-dir', str(candidate),
            '--expected-tree-sha256', live_digest,
            '--expected-candidate-tree-sha256', candidate_digest,
            '--target', 'A-001', '--source', 'synthetic method review', ok=False,
        ).stderr)

    def test_visibility_conflict_and_legacy_unpublished_are_reported(self):
        self.run_cli(
            'publish', '--id', self.case_id,
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--expected-tree-sha256', self.tree_sha(),
            '--source', 'synthetic publish review',
        )
        duplicate = self.unpublished / self.case_id
        duplicate.mkdir(parents=True)
        (duplicate / 'investigation.md').write_text(self.case.read_text() if self.case.exists() else (self.root / self.case_id / 'investigation.md').read_text())
        conflict = self.run_cli('load', '--id', self.case_id, ok=False)
        self.assertIn('visibility_conflict', conflict.stderr)

        legacy_id = '20260908-130000-legacy-filter'
        legacy_dir = self.unpublished / legacy_id
        legacy_dir.mkdir()
        (legacy_dir / 'investigation.md').write_text(
            '---\nid: 20260908-130000-legacy-filter\nstatus: intake\n---\n\n## Original request\n\nOld case.\n',
            encoding='utf-8',
        )
        loaded = json.loads(self.run_cli('load', '--id', legacy_id).stdout)
        self.assertEqual(loaded['status'], 'legacy')

    def test_unpublished_retirement_does_not_write_the_public_ledger(self):
        closed = self.run_cli(
            'close', '--id', self.case_id, '--decision', 'abandoned',
            '--reason', 'Local-only experiment ended',
            '--limitations', 'Never published',
            '--source', 'synthetic local close',
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
        )
        self.assertIn('closed', closed.stdout)
        retired = self.run_cli(
            'retire', '--id', self.case_id, '--authorized',
            '--expected-public-sha256', hashlib.sha256(self.case.read_bytes()).hexdigest(),
            '--reason', 'Discard unpublished experiment',
            '--source', 'synthetic local retirement',
            '--dependency-review', 'No published consumers',
            '--absorption-review', 'No durable knowledge',
            '--summary', 'Unpublished case discarded locally',
        )
        self.assertIn('"visibility": "unpublished"', retired.stdout)
        self.assertFalse(self.case.parent.exists())
        self.assertFalse((self.root / 'retired.md').exists())

if __name__ == '__main__':
    unittest.main()
