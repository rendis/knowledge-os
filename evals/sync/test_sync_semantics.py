#!/usr/bin/env python3
"""Regression contracts for evidence package checks and no-change cursors."""
from __future__ import annotations

import importlib.util
import json
import sys
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from fixtures import NODE, SOURCE_CLAIM, SOURCE_REPOSITORY, digest, make_repository_pair, package_artifacts

META = Path(__file__).resolve().parents[2] / 'kernel' / '90-Meta'
sys.path.insert(0, str(META))


def load_module(name: str, filename: str):
    spec = importlib.util.spec_from_file_location(name, META / filename)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


manifest_tool = load_module('sync_semantics_manifest', 'git-change-manifest.py')
# Inventory discovers its cell configuration at import time, so load an installed
# fixture rather than replacing its configuration/parser behavior with mocks.
_inventory_fixture = tempfile.TemporaryDirectory()
_inventory_root = Path(_inventory_fixture.name) / 'vault'
subprocess.run([str(META.parents[1] / 'install.sh'), 'init', '--dest', str(_inventory_root), '--cell-name', 'Fixture', '--purpose', 'Sync regression', '--system', 'fixture:Fixture', '--yes'], check=True, capture_output=True)
_inventory_spec = importlib.util.spec_from_file_location('sync_semantics_inventory', _inventory_root / '90-Meta' / 'vault-inventory.py')
inventory_tool = importlib.util.module_from_spec(_inventory_spec)
sys.modules[_inventory_spec.name] = inventory_tool
_inventory_spec.loader.exec_module(inventory_tool)


def save(path: Path, value: dict) -> None:
    path.write_text(json.dumps(value), encoding='utf-8')


class SyncSemanticsEval(unittest.TestCase):
    def check(self, repo: Path, artifacts: tuple) -> dict:
        return manifest_tool.check_analysis(repo, *artifacts[:3], 'refs/heads/main')

    def test_check_exposes_repository_and_ignores_question_key_order(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, _ = make_repository_pair(root)
            artifacts = package_artifacts(root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM, requested_nodes=(NODE,))
            baseline = self.check(source, artifacts)
            self.assertEqual(baseline['status'], 'pass', baseline)
            self.assertEqual(baseline.get('repository'), SOURCE_REPOSITORY)
            analysis = json.loads(artifacts[2].read_text())
            for item in analysis['checklist'].values():
                item['questions'] = dict(reversed(list(item['questions'].items())))
            save(artifacts[2], analysis)
            reordered = self.check(source, artifacts)
            self.assertEqual(reordered, baseline)

    def test_check_rejects_missing_and_extra_questions(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, _ = make_repository_pair(root)
            artifacts = package_artifacts(root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM, requested_nodes=(NODE,))
            original = json.loads(artifacts[2].read_text())
            for mutation in ('missing', 'extra'):
                with self.subTest(mutation=mutation):
                    analysis = json.loads(json.dumps(original))
                    questions = next(iter(analysis['checklist'].values()))['questions']
                    if mutation == 'missing':
                        questions.pop(next(iter(questions)))
                    else:
                        questions['invented-question'] = 'not-observed'
                    save(artifacts[2], analysis)
                    result = self.check(source, artifacts)
                    self.assertEqual(result['status'], 'blocked', result)
                    self.assertIn('minimum-sweep-questions-incomplete', {i['code'] for i in result['issues']})

    def test_gate_distinguishes_existing_no_change_new_no_node_and_limited_review(self) -> None:
        for case, expected in (
            ('existing', 'no-documentation-change'),
            ('new', 'no-durable-node'),
            ('blocked-review', 'inspection-limited'),
            ('revise-review', 'review-rejected'),
            ('missing-review', 'inspection-limited'),
        ):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                source, _, oids = make_repository_pair(root)
                artifacts = package_artifacts(root, source, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM, requested_nodes=(NODE,))
                manifest = json.loads(artifacts[0].read_text())
                scaffold = json.loads(artifacts[1].read_text())
                if case == 'new':
                    manifest = manifest_tool.build_new_manifest(source, oids[SOURCE_REPOSITORY])
                    scaffold = manifest_tool.build_analysis_scaffold(manifest, SOURCE_REPOSITORY, [])
                analysis = json.loads(json.dumps(scaffold))
                for item in analysis['paths']:
                    item.update(disposition='not-documentable', reason='Fixture change introduces no durable fact.', claim_ids=[])
                for item in analysis['checklist'].values():
                    item.update(status='checked', reason='Complete fixture inspection.', evidence=[{'path': 'component.txt', 'anchor': 'complete file'}])
                    item['questions'] = dict.fromkeys(item['questions'], 'not-observed')
                for node in analysis['nodes']:
                    node.update(action='update' if case != 'new' and node['basename'] == manifest_tool.repository_basename(SOURCE_REPOSITORY) else 'no-change', reason='No durable fact changed.', claim_ids=[])
                analysis.update(claims=[], blockers=[], result='no-change' if case == 'new' else 'traceability-only')
                review = dict(version=3, repository=SOURCE_REPOSITORY, manifest_digest=digest(manifest), scaffold_digest=digest(scaffold), analysis_digest=digest(analysis), verdict='accept', findings=[])
                if case in {'blocked-review', 'revise-review'}:
                    review.update(verdict='blocked' if case == 'blocked-review' else 'revise', findings=[{'target': '$', 'category': 'evidence', 'reason': 'Review incomplete.', 'nodes': [manifest_tool.repository_basename(SOURCE_REPOSITORY)], 'evidence': {'path': 'component.txt', 'anchor': 'complete file'}}])
                for path, payload in zip(artifacts, (manifest, scaffold, analysis, review)):
                    save(path, payload)
                if case == 'missing-review':
                    artifacts[3].unlink()
                gate = manifest_tool.gate_batch_sources([[source, *artifacts]], [SOURCE_REPOSITORY], {SOURCE_REPOSITORY: ('main', '2026-09-06')})
                self.assertIn('repositories', gate, gate)
                self.assertEqual(gate['repositories'][0]['cursor_decision'], expected, gate)
                self.assertEqual(gate['acknowledgements'][0]['decision'], expected, gate)
                self.assertEqual(gate['write_groups'], [], gate)
                if case == 'blocked-review':
                    self.assertEqual(gate['repositories'][0]['fallback_reason'], 'review-limited')
                if case == 'revise-review':
                    self.assertEqual(gate['repositories'][0]['fallback_reason'], 'review-revise')
                if case == 'existing':
                    closed = manifest_tool.close_package(source, *artifacts, 'refs/heads/main', '2026-09-06')
                    self.assertEqual(closed['status'], 'complete', closed)
                    closed_path = root / 'closed.json'
                    save(closed_path, closed)
                    self.assertEqual(manifest_tool.gate_batch([closed_path], [SOURCE_REPOSITORY]), gate)
                    self.assertEqual(manifest_tool.validate_gate_for_write(gate)[2], [])
                    for mutation in ({'is_new': True}, {'analysis_fallback': True}, {'verdict': 'blocked'}):
                        with self.subTest(invalid_no_change=mutation):
                            invalid = json.loads(json.dumps(gate))
                            invalid['repositories'][0].update(mutation)
                            issues = manifest_tool.validate_gate_for_write(invalid)[2]
                            self.assertIn({'code': 'gate-contract-invalid', 'field': 'gate.repositories[0].cursor_decision'}, issues)

    def test_inventory_no_change_ack_requires_note_and_expires_on_remote_change(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source, _, oids = make_repository_pair(root)
            current = oids[SOURCE_REPOSITORY][:12]
            old = manifest_tool.resolve_commit(source, 'HEAD~1')[:12]
            note = root / '20-Repos' / 'systems' / 'source-adapter.md'
            note.parent.mkdir(parents=True)
            note.write_text(f'---\naliases: [{SOURCE_REPOSITORY}]\ncommit-analizado: "{old}"\nrama-analizada: main\n---\nExisting knowledge.\n', encoding='utf-8')
            ack_path = root / inventory_tool.SYNC_ACKNOWLEDGEMENTS
            ack_path.parent.mkdir(parents=True)
            save(ack_path, {'version': 1, 'repositories': [{'repository': SOURCE_REPOSITORY, 'branch': 'main', 'analyzed_sha': current, 'decision': 'no-documentation-change', 'analysis_date': '2026-09-06'}]})
            github = inventory_tool.GitHubContext('fixture', 'fixture', '')
            remote = {'name': SOURCE_REPOSITORY, 'branch': 'main', 'sha': current}
            with patch.object(inventory_tool, 'CORE_APP_PREFIXES', ('APP00000-',)), patch.object(inventory_tool, 'load_org', return_value=[remote]):
                self.assertEqual(inventory_tool.build_inventory(root, 'fixture', github)[0]['status'], 'acknowledged-no-change')
                original_note = note.read_text()
                note.write_text(original_note.replace(old, current))
                self.assertEqual(inventory_tool.build_inventory(root, 'fixture', github)[0]['status'], 'acknowledged-no-change')
                note.write_text(original_note)
                remote['sha'] = 'f' * 12
                self.assertEqual(inventory_tool.build_inventory(root, 'fixture', github)[0]['status'], 'changed')
                remote['sha'] = current
                note.unlink()
                with self.assertRaises(inventory_tool.OperationalError):
                    inventory_tool.build_inventory(root, 'fixture', github)


if __name__ == '__main__':
    unittest.main()
