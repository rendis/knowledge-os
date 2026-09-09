"""Synthetic end-to-end reviewer redaction; no detector exemptions."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from fixtures import make_repository_pair, package_artifacts, SOURCE_REPOSITORY, SOURCE_CLAIM, NODE, digest, write_json

ROOT = Path(__file__).resolve().parents[2]
CLI = ROOT / 'kernel/90-Meta/finalize-sync-review.py'
MANIFEST = CLI.with_name('git-change-manifest.py')
LITERAL = 'K9m2Q7v4R8x6B3n5'

class ReviewSafetyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.repo, _, _ = make_repository_pair(self.root)
        (self.repo / 'z.env').write_text('api_key="' + LITERAL + '"\n')
        subprocess.run(['git','-C',str(self.repo),'add','.'],check=True)
        subprocess.run(['git','-C',str(self.repo),'commit','--amend','--no-edit','-q'],check=True)
        subprocess.run(['git','-C',str(self.repo),'checkout','-q','-B','main'],check=True)
        self.paths = package_artifacts(self.root,self.repo,SOURCE_REPOSITORY,claim_id=SOURCE_CLAIM,requested_nodes=(NODE,),rejected_review=True)
        result = subprocess.run([sys.executable,'-B',str(MANIFEST),'finalize-analysis','--repo',str(self.repo), *[arg for key,p in zip(('manifest','scaffold','analysis'),self.paths[:3]) for arg in ('--'+key,str(p))]],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stdout)
        review=json.loads(self.paths[3].read_text())
        review['analysis_digest']=digest(json.loads(self.paths[2].read_text()))
        review['findings'][0]['reason']='Check '+LITERAL+' before accepting.'
        review['findings'][0]['evidence']['anchor']='Line containing '+LITERAL
        write_json(self.paths[3],review)
        self.review=review
        self.original=[p.read_bytes() for p in self.paths]
        self.out=self.root/'safe.json'

    def run_cli(self):
        return subprocess.run([sys.executable,'-B',str(CLI),'--repo',str(self.repo), *[arg for key,p in zip(('manifest','scaffold','analysis','review'),self.paths) for arg in ('--'+key,str(p))], '--output',str(self.out)],capture_output=True,text=True)

    def test_redact_preserve_idempotent_and_close(self):
        first=self.run_cli()
        self.assertEqual(first.returncode,0,first.stdout+first.stderr)
        self.assertNotIn(LITERAL,first.stdout+first.stderr+self.out.read_text())
        actual=json.loads(self.out.read_text())
        expected=json.loads(json.dumps(self.review).replace(LITERAL,'[redacted]'))
        self.assertEqual(actual,expected)
        self.assertEqual([p.read_bytes() for p in self.paths],self.original)
        self.assertEqual(self.run_cli().stdout,first.stdout)
        closed=subprocess.run([sys.executable,'-B',str(MANIFEST),'close-package','--repo',str(self.repo), *[arg for key,p in zip(('manifest','scaffold','analysis'),self.paths[:3]) for arg in ('--'+key,str(p))],'--review',str(self.out),'--production-ref','refs/heads/main','--analysis-date','2026-09-09','--output',str(self.root/'closed.json')],capture_output=True,text=True)
        self.assertEqual(closed.returncode,0,closed.stdout+closed.stderr)
        self.assertEqual(json.loads((self.root/'closed.json').read_text())['review']['verdict'],'revise')

    def test_accept_stays_accept(self):
        self.review.update(verdict='accept',findings=[])
        write_json(self.paths[3],self.review)
        self.assertEqual(self.run_cli().returncode,0)
        self.assertEqual(json.loads(self.out.read_text()),self.review)

    def test_invalid_digest_metadata_and_path_block(self):
        for kind in ('digest','metadata','path','unavailable'):
            review=json.loads(self.original[3])
            if kind=='digest': review['analysis_digest']='0'*64
            elif kind=='metadata': review['findings'][0]['category']=LITERAL
            elif kind=='path': review['findings'][0]['evidence']['path']=LITERAL
            else: review['findings'][0]['evidence']['path']='missing.txt'
            write_json(self.paths[3],review)
            result=self.run_cli()
            self.assertEqual(result.returncode,2,kind)
            self.assertNotIn(LITERAL,result.stdout+result.stderr)
            self.assertFalse(self.out.exists())

    def test_detached_head_and_stale_source(self):
        subprocess.run(['git','-C',str(self.repo),'checkout','--detach','-q'],check=True)
        self.assertEqual(self.run_cli().returncode,0)
        self.out.unlink()
        subprocess.run(['git','-C',str(self.repo),'checkout','--detach','-q','HEAD~1'],check=True)
        self.assertEqual(self.run_cli().returncode,2)
        self.assertFalse(self.out.exists())

    def test_no_clobber(self):
        self.out.write_text('existing bytes')
        self.assertEqual(self.run_cli().returncode,2)
        self.assertEqual(self.out.read_text(),'existing bytes')

if __name__=='__main__': unittest.main()
