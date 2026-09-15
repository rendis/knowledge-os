"""Generic extraction envelope and persisted schema-2 package compatibility."""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest
from fixtures import make_repository_pair, package_artifacts, SOURCE_REPOSITORY, SOURCE_CLAIM, NODE
ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'kernel/90-Meta'))
spec = importlib.util.spec_from_file_location('extraction_manifest', ROOT/'kernel/90-Meta/git-change-manifest.py')
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)

class ExtractionContract(unittest.TestCase):
    def test_generic_and_persisted_package_contracts(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);repo,_,_=make_repository_pair(root)
            manifest,scaffold,analysis,review=package_artifacts(root,repo,SOURCE_REPOSITORY,claim_id=SOURCE_CLAIM,requested_nodes=(NODE,))
            raw_manifest=json.loads(manifest.read_text()); raw=json.loads(analysis.read_text())
            self.assertEqual(raw['version'],3)
            self.assertEqual(m.check_analysis(repo,manifest,scaffold,analysis,'refs/heads/main')['status'],'pass')
            # Restore a persisted v2 shape, without transforming its own claims.
            old=m.build_analysis_scaffold(raw_manifest,SOURCE_REPOSITORY,[NODE],analysis_version=2)
            scaffold.write_text(json.dumps(old));raw['version']=2
            for d,item in raw['checklist'].items():
                item['questions']={q:'not-observed' for q in old['checklist'][d]['questions']}
            analysis.write_text(json.dumps(raw));before=analysis.read_bytes()
            self.assertEqual(m.check_analysis(repo,manifest,scaffold,analysis,'refs/heads/main')['status'],'pass')
            self.assertEqual(analysis.read_bytes(),before)
            raw['version']=3;analysis.write_text(json.dumps(raw))
            result=m.check_analysis(repo,manifest,scaffold,analysis,'refs/heads/main')
            self.assertEqual(result['status'],'blocked')
            self.assertIn('scaffold-version-mismatch',{i['code'] for i in result['issues']})

if __name__=='__main__':unittest.main()
