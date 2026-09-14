"""Persistence correspondence regressions, independent of a browser."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('context', ROOT / 'kernel/.agents/skills/explain-visually/scripts/check_visual_context.py')
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class ContextChecks(unittest.TestCase):
    def test_retained_pair_and_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            visual = root / 'flow.html'
            visual.write_text('<h1>Reviewed flow</h1>')
            meta = {'files': [{'path': visual.name, 'sha256': hashlib.sha256(visual.read_bytes()).hexdigest()}], 'sources': ['E-001'], 'embedded': False}
            context = root / 'flow.md'
            def write():
                context.write_text(f'<!-- visual-context {json.dumps(meta)} -->\nPurpose: trace a request. [Open](flow.html)')
            write()
            self.assertTrue(MODULE.check(context)['context_pass'])
            visual.write_text('<h1>Changed flow</h1>')
            self.assertFalse(MODULE.check(context)['context_pass'])
            meta['files'][0]['sha256'] = hashlib.sha256(visual.read_bytes()).hexdigest()
            write()
            self.assertTrue(MODULE.check(context)['context_pass'])
            visual.unlink()
            self.assertFalse(MODULE.check(context)['context_pass'])

    def test_missing_metadata_sources_and_embedded_diagram(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory) / 'flow.md'
            for body, expected in [('', False), ('<!-- visual-context {"files": [], "embedded": true, "sources": []} -->', False), ('<!-- visual-context {"files": [], "embedded": true, "sources": ["Synthetic example"]} -->\n```mermaid\nflowchart LR\n A --> B\n```', True)]:
                context.write_text(body)
                self.assertEqual(MODULE.check(context)['context_pass'], expected)

    def test_reference_links_and_unlisted_companion(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            visual = root / 'flow.html'
            visual.write_text('<h1>Flow</h1>')
            meta = {'files': [{'path': visual.name, 'sha256': hashlib.sha256(visual.read_bytes()).hexdigest()}], 'sources': ['E-001']}
            context = root / 'flow.md'
            body = f'<!-- visual-context {json.dumps(meta)} -->\n[Open][view]\n\n[view]: flow.html\n'
            context.write_text(body)
            self.assertTrue(MODULE.check(context)['context_pass'])
            (root / 'data.csv').write_text('units\n10\n')
            context.write_text(body + '\n[Data](data.csv)')
            self.assertFalse(MODULE.check(context)['context_pass'])
            meta['files'].append({'path': 'data.csv', 'sha256': hashlib.sha256((root / 'data.csv').read_bytes()).hexdigest()})
            context.write_text(f'<!-- visual-context {json.dumps(meta)} -->\n[Open][view]\n[view]: flow.html\n[Data](data.csv)')
            self.assertTrue(MODULE.check(context)['context_pass'])

    def test_embedded_examples_are_not_rendered_diagrams(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory) / 'flow.md'
            marker = '<!-- visual-context {"files": [], "sources": ["Synthetic"], "embedded": true} -->\n'
            for example in ['<!-- <svg></svg> -->', '```html\n<svg></svg>\n```', '```mermaid\n\n```', 'The literal <svg token is not a diagram.', '```html\n<svg></svg>\n````', '`<svg></svg>`', '    <svg></svg>']:
                context.write_text(marker + example)
                self.assertFalse(MODULE.check(context)['context_pass'])
            context.write_text(marker + '<svg aria-label="Flow"><text>Example</text></svg>')
            self.assertTrue(MODULE.check(context)['context_pass'])

    def test_self_hash_and_path_escape(self):
        with tempfile.TemporaryDirectory() as directory:
            context = Path(directory) / 'flow.md'
            for name in ['flow.md', '../outside.html', '/tmp/outside.html']:
                meta = {'files': [{'path': name, 'sha256': '0' * 64}], 'sources': ['E-001']}
                context.write_text(f'<!-- visual-context {json.dumps(meta)} -->\n[Open]({name})')
                self.assertFalse(MODULE.check(context)['context_pass'])


if __name__ == '__main__':
    unittest.main()
