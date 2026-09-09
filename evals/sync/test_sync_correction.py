"""Public CLI regressions for the single pre-checkpoint correction."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from fixtures import make_repository_pair, package_artifacts, SOURCE_REPOSITORY, SOURCE_CLAIM, NODE, digest, write_json

ROOT = Path(__file__).resolve().parents[2]
CLI = ROOT / 'kernel/90-Meta/sync-correction.py'
MANIFEST = CLI.with_name('git-change-manifest.py')

class CorrectionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.repo, _, _ = make_repository_pair(self.root)
        self.paths = package_artifacts(self.root, self.repo, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM, requested_nodes=(NODE,), rejected_review=True)
        subprocess.run(['git', '-C', str(self.repo), 'checkout', '-q', '-B', 'main'], check=True)
        self.workspace = self.paths[2].parent / 'correction-1'
        self.original = [p.read_bytes() for p in self.paths]

    def run_cli(self, command, *extra):
        result = subprocess.run([sys.executable, '-B', str(CLI), command, '--repo', str(self.repo), '--current-ref', 'refs/heads/main', *extra], capture_output=True, text=True)
        self.assertTrue(result.stdout, result.stderr)
        return result.returncode, json.loads(result.stdout)

    def prepare(self):
        return self.run_cli('prepare', *[arg for key, p in zip(('manifest','scaffold','analysis','review'), self.paths) for arg in ('--' + key, str(p))])

    def repair(self):
        analysis = json.loads((self.workspace / 'analysis.json').read_text())
        analysis['claims'][0]['statement'] = 'Corrected fixture claim backed by the same source.'
        write_json(self.workspace / 'analysis.json', analysis)
        result = subprocess.run([sys.executable, '-B', str(MANIFEST), 'finalize-analysis', '--repo', str(self.repo), '--manifest', str(self.paths[0]), '--scaffold', str(self.paths[1]), '--analysis', str(self.workspace / 'analysis.json')], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        analysis = json.loads((self.workspace / 'analysis.json').read_text())
        review = json.loads(self.paths[3].read_text())
        review.update(analysis_digest=digest(analysis), verdict='accept', findings=[])
        write_json(self.workspace / 'review.json', review)

    def check(self):
        return self.run_cli('check', '--workspace', str(self.workspace), '--review', str(self.workspace / 'review.json'))

    def test_success_preserves_initial_and_closes_existing_package(self):
        self.assertEqual(self.prepare()[0], 0)
        self.repair()
        self.assertEqual(self.check()[0], 0)
        result = subprocess.run([sys.executable, '-B', str(MANIFEST), 'close-package', '--repo', str(self.repo), '--manifest', str(self.paths[0]), '--scaffold', str(self.paths[1]), '--analysis', str(self.workspace / 'analysis.json'), '--review', str(self.workspace / 'review.json'), '--production-ref', 'refs/heads/main', '--analysis-date', '2026-09-08', '--output', str(self.root / 'closed.json')], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([p.read_bytes() for p in self.paths], self.original)

    def test_stale_or_nonrevise_review_creates_nothing(self):
        for update in ({'analysis_digest': '0'*64}, {'verdict': 'blocked'}, {'verdict': 'accept', 'findings': []}):
            review = json.loads(self.original[3])
            review.update(update)
            write_json(self.paths[3], review)
            self.assertNotEqual(self.prepare()[0], 0)
            self.assertFalse(self.workspace.exists())

    def test_omitted_claim_added_without_rewriting_accurate_claim(self):
        review = json.loads(self.paths[3].read_text())
        review['findings'][0]['target'] = 'paths.component.txt'
        write_json(self.paths[3], review)
        self.assertEqual(self.prepare()[0], 0)
        analysis = json.loads((self.workspace / 'analysis.json').read_text())
        accurate = json.loads(json.dumps(analysis['claims'][0]))
        analysis['claims'].append({**accurate, 'claim_id': 'omitted-claim', 'statement': 'Previously omitted documented behavior.'})
        next(n for n in analysis['nodes'] if n['basename'] == NODE)['claim_ids'].append('omitted-claim')
        next(p for p in analysis['paths'] if p['path'] == 'component.txt')['claim_ids'].append('omitted-claim')
        write_json(self.workspace / 'analysis.json', analysis)
        review.update(analysis_digest=digest(analysis), verdict='accept', findings=[])
        write_json(self.workspace / 'review.json', review)
        self.assertEqual(self.check()[0], 0)
        self.assertEqual(analysis['claims'][0], accurate)

    def test_omitted_claim_can_cite_unchanged_contract_but_not_new_node(self):
        (self.repo / 'contract.txt').write_text('The consumer preserves the public contract.\n')
        subprocess.run(['git', '-C', str(self.repo), 'add', 'contract.txt'], check=True)
        subprocess.run(['git', '-C', str(self.repo), 'commit', '-qm', 'add contract'], check=True)
        (self.repo / 'component.txt').write_text('The caller uses the unchanged contract.\n')
        subprocess.run(['git', '-C', str(self.repo), 'commit', '-qam', 'update caller'], check=True)
        package_root = self.root / 'multi-path'
        package_root.mkdir()
        self.paths = package_artifacts(package_root, self.repo, SOURCE_REPOSITORY, claim_id=SOURCE_CLAIM, requested_nodes=(NODE,), rejected_review=True)
        self.workspace = self.paths[2].parent / 'correction-1'
        review = json.loads(self.paths[3].read_text())
        review['findings'][0]['target'] = 'paths.component.txt'
        write_json(self.paths[3], review)
        self.assertEqual(self.prepare()[0], 0)
        analysis = json.loads((self.workspace / 'analysis.json').read_text())
        accurate = json.loads(json.dumps(analysis['claims'][0]))
        analysis['claims'].append({
            'claim_id': 'omitted-claim', 'statement': 'Caller preserves the unchanged consumer contract.',
            'evidence': [{'path': 'component.txt', 'anchor': 'caller'}, {'path': 'contract.txt', 'anchor': 'public contract'}],
        })
        next(n for n in analysis['nodes'] if n['basename'] == NODE)['claim_ids'].append('omitted-claim')
        analysis['paths'][0]['claim_ids'].append('omitted-claim')
        write_json(self.workspace / 'analysis.json', analysis)
        review.update(analysis_digest=digest(analysis), verdict='accept', findings=[])
        write_json(self.workspace / 'review.json', review)
        self.assertEqual(self.check()[0], 0)
        self.assertEqual(analysis['claims'][0], accurate)
        analysis['nodes'].append({'basename': 'unrelated-node', 'action': 'create', 'reason': 'Unrelated expansion.', 'claim_ids': ['omitted-claim']})
        analysis['nodes'].sort(key=lambda n: n['basename'])
        write_json(self.workspace / 'analysis.json', analysis)
        review['analysis_digest'] = digest(analysis)
        write_json(self.workspace / 'review.json', review)
        self.assertEqual(self.check()[1]['code'], 'correction-outside-findings')

    def test_second_prepare_and_recursive_attempt_rejected(self):
        self.assertEqual(self.prepare()[0], 0)
        self.assertNotEqual(self.prepare()[0], 0)
        self.paths = (self.paths[0], self.paths[1], self.workspace / 'analysis.json', self.paths[3])
        self.assertEqual(self.prepare()[1]['code'], 'correction-limit-reached')

    def test_symlink_output_is_rejected_without_writes(self):
        dest = self.root / 'untouched'
        dest.mkdir()
        self.workspace.symlink_to(dest, target_is_directory=True)
        self.assertEqual(self.prepare()[1]['code'], 'unsafe-path')
        self.assertEqual(list(dest.iterdir()), [])

    def test_stale_final_review_and_unrelated_node_change_rejected(self):
        self.assertEqual(self.prepare()[0], 0)
        self.repair()
        write_json(self.workspace / 'review.json', json.loads(self.original[3]))
        self.assertNotEqual(self.check()[0], 0)
        review = json.loads(self.original[3])
        review.update(verdict='accept', findings=[])
        write_json(self.workspace / 'review.json', review)
        analysis = json.loads((self.workspace / 'analysis.json').read_text())
        other = next(n for n in analysis['nodes'] if n['basename'] != NODE)
        other['reason'] = 'Unrelated alteration.'
        write_json(self.workspace / 'analysis.json', analysis)
        review = json.loads((self.workspace / 'review.json').read_text())
        review['analysis_digest'] = digest(analysis)
        write_json(self.workspace / 'review.json', review)
        self.assertEqual(self.check()[1]['code'], 'correction-outside-findings')

if __name__ == '__main__':
    unittest.main()
