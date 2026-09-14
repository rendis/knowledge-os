#!/usr/bin/env python3
"""Real installer conversations in disposable directories; no mocked input or writes."""
import os
from pathlib import Path
import select
import signal
import subprocess
import sys
import tempfile
import unittest

DIST = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(DIST / 'kernel' / '90-Meta'))
from instance import load_instance

PROMPTS = ['Systems as', 'Trackers as', 'Cell name', 'Cell purpose',
           'Evidence profile', 'Note locale', 'Adapters (']
ANSWERS = ['orders:Orders', 'work:github:https://example.org/issues',
           'Commerce', 'Order fulfillment.', 'documented-source', 'en', 'reports']


def snapshot(root):
    return {str(p.relative_to(root)): ('link', os.readlink(p)) if p.is_symlink()
            else ('file', p.read_bytes()) if p.is_file() else ('dir', None)
            for p in root.rglob('*')}


class InteractiveOnboardingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.dest = self.root / 'vault'

    def command(self, *args):
        return ['sh', str(DIST / 'install.sh'), 'init', '--dest', str(self.dest), *args]

    def run_init(self, answers, *args):
        result = subprocess.run(self.command(*args), input=answers, text=True,
                                capture_output=True, cwd=self.root, timeout=30)
        print(f'\nTRANSCRIPT {self.id()} args={args!r}\n{result.stdout}{result.stderr}')
        return result

    def test_eof_at_every_prompt_writes_nothing(self):
        for index in range(len(ANSWERS)):
            with self.subTest(prompt=PROMPTS[index]):
                result = self.run_init(''.join(a + '\n' for a in ANSWERS[:index]))
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Initialization cancelled', result.stderr)
                self.assertFalse(self.dest.exists())

    def test_enter_defaults_is_intentional(self):
        result = self.run_init('\n' * 7)
        self.assertEqual(result.returncode, 0, result.stderr)
        instance = load_instance(self.dest / 'instance.yaml')
        self.assertEqual(instance['cell']['name'], 'Cell')
        self.assertEqual(instance['evidence']['profile'], 'production-gate')
        self.assertEqual(instance['locale']['notes'], 'es')
        self.assertEqual(instance['adapters'], [])

    def test_answers_persist_without_local_clone_authority(self):
        discovery = self.root / 'discovery'
        discovery.mkdir()
        (discovery / 'sentinel').write_text('untouched')
        before = snapshot(discovery)
        result = self.run_init('\n'.join(ANSWERS) + '\n', '--discovery-root', str(discovery))
        self.assertEqual(result.returncode, 0, result.stderr)
        positions = [result.stdout.index(prompt) for prompt in PROMPTS]
        self.assertEqual(positions, sorted(positions))
        instance = load_instance(self.dest / 'instance.yaml')
        self.assertEqual(instance['systems'][0]['id'], 'orders')
        self.assertEqual(instance['cell'], {'name': 'Commerce', 'purpose': 'Order fulfillment.'})
        self.assertEqual(instance['trackers'], [{'id': 'work', 'provider': 'github', 'url': 'https://example.org/issues'}])
        self.assertEqual(instance['evidence']['profile'], 'documented-source')
        self.assertEqual(instance['locale']['notes'], 'en')
        self.assertEqual(instance['adapters'], ['reports'])
        self.assertEqual(instance['sources']['discovery_roots'], [str(discovery)])
        self.assertEqual(snapshot(discovery), before)
        self.assertFalse((self.dest / '.knowledge-os-config.yaml').exists())
        self.assertNotIn('clone', result.stdout.lower())

    def test_explicit_yes_and_existing_vault_rejection(self):
        result = self.run_init('', '--yes', '--cell-name', 'Commerce', '--purpose', 'Orders.',
                               '--system', 'orders:Orders', '--evidence-profile', 'mixed', '--locale', 'en')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(p in result.stdout for p in PROMPTS))
        before = snapshot(self.dest)
        result = self.run_init('\n'.join(ANSWERS) + '\n')
        self.assertEqual(result.returncode, 2)
        self.assertEqual(snapshot(self.dest), before)
        self.assertNotIn('Systems as', result.stdout)

    def test_keyboard_interrupt_at_prompt_writes_nothing(self):
        process = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, cwd=self.root)
        try:
            ready, _, _ = select.select([process.stdout], [], [], 10)
            self.assertTrue(ready, 'installer never reached a prompt')
            prefix = os.read(process.stdout.fileno(), 4096)
            self.assertIn(b'Systems as', prefix)
            process.send_signal(signal.SIGINT)
            stdout, stderr = process.communicate(timeout=10)
            print('\nTRANSCRIPT interrupt\n' + (prefix + stdout + stderr).decode())
            self.assertNotEqual(process.returncode, 0)
            self.assertIn(b'Initialization cancelled', stderr)
            self.assertNotIn(b'Traceback', stderr)
            self.assertFalse(self.dest.exists())
        finally:
            if process.poll() is None:
                process.kill()
                process.communicate()


if __name__ == '__main__':
    unittest.main(verbosity=2)
