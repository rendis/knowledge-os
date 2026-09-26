#!/usr/bin/env python3
"""Real `kos init` conversations in disposable directories; no mocked input or writes.

Per ADR 0002, `kos init` asks the same questions the old checkout installer asked, but on stderr —
stdout stays JSON. `--discovery-root` is gone (instance.yaml no longer has `sources.discovery_roots`,
and Home no longer lists remotes found at init), so the answers below no longer touch a discovery
directory; a plain roundtrip of every other answer replaces that check.
"""
from __future__ import annotations

import os
import re
import select
import signal
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _kos  # noqa: E402

PROMPTS = ['Cell name', 'What the cell owns', 'Systems it owns', 'GitHub organization', 'Repository name prefixes',
           'Reference branches', 'Clouds the systems', 'Issue tracker URLs', 'Language of the notes']
ANSWERS = ['Commerce', 'Order fulfillment.', 'Orders, order-returns:Returns', 'acme', 'APP01-, APP02-',
           'develop,main', 'gcp, aws', 'https://acme.atlassian.net/jira/software/projects/ORD', 'en']


def snapshot(root):
    return {str(p.relative_to(root)): ('link', os.readlink(p)) if p.is_symlink()
            else ('file', p.read_bytes()) if p.is_file() else ('dir', None)
            for p in root.rglob('*')}


def _raw(text: str, section: str, key: str) -> str | None:
    """The unparsed rest-of-line for `key:` inside a `section:` block, as `dumpInstance` writes it."""
    m = re.search(rf'^{re.escape(section)}:\n(?:  .*\n)*?  {re.escape(key)}: (.*)$', text, re.MULTILINE)
    return m.group(1) if m else None


def scalar(text: str, section: str, key: str) -> str | None:
    raw = _raw(text, section, key)
    if raw is None:
        return None
    return raw[1:-1] if raw.startswith('"') and raw.endswith('"') else raw


def flow_list(text: str, section: str, key: str) -> list[str]:
    raw = _raw(text, section, key)
    if raw is None:
        return []
    inner = raw.strip()
    if inner.startswith('[') and inner.endswith(']'):
        inner = inner[1:-1]
    inner = inner.strip()
    return [] if not inner else [part.strip().strip('"') for part in inner.split(',')]


def top_level(text: str, key: str) -> str | None:
    m = re.search(rf'^{re.escape(key)}: (.*)$', text, re.MULTILINE)
    return m.group(1) if m else None


def system_ids(text: str) -> list[tuple[str, str]]:
    return re.findall(r'^  - id: "([^"]*)"\n    name: "([^"]*)"', text, re.MULTILINE)


def trackers(text: str) -> list[dict[str, str]]:
    return [{"id": i, "provider": p, "url": u} for i, p, u in
            re.findall(r'^  - id: "([^"]*)"\n    provider: "([^"]*)"\n    url: "([^"]*)"', text, re.MULTILINE)]


class InteractiveOnboardingTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.dest = self.root / 'vault'
        self.kos = str(_kos.binary(DIST))

    def command(self, *args):
        return [self.kos, 'init', '--vault', str(self.dest), *args]

    def run_init(self, answers, *args):
        env = {**os.environ, 'KOS_NO_UPDATE_CHECK': '1'}
        result = subprocess.run(self.command(*args), input=answers, text=True,
                                capture_output=True, cwd=self.root, timeout=30, env=env)
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
        result = self.run_init('\n' * len(PROMPTS))
        self.assertEqual(result.returncode, 0, result.stderr)
        instance = (self.dest / 'instance.yaml').read_text(encoding='utf-8')
        self.assertEqual(scalar(instance, 'cell', 'name'), 'Cell')
        self.assertEqual(scalar(instance, 'evidence', 'profile'), 'production-gate')
        self.assertEqual(scalar(instance, 'locale', 'notes'), 'es')
        self.assertEqual(top_level(instance, 'adapters'), '[]')
        self.assertEqual(system_ids(instance)[0][0], 'platform')
        self.assertEqual(flow_list(instance, 'sources', 'reference_branch_order'), ['main', 'master'])
        self.assertEqual(flow_list(instance, 'platform', 'providers'), [])

    def test_answers_persist_and_questions_go_to_stderr(self):
        result = self.run_init('\n'.join(ANSWERS) + '\n')
        self.assertEqual(result.returncode, 0, result.stderr)
        # stdout stays JSON; the conversation itself is on stderr, in order.
        self.assertFalse(any(p in result.stdout for p in PROMPTS))
        positions = [result.stderr.index(prompt) for prompt in PROMPTS]
        self.assertEqual(positions, sorted(positions))
        instance = (self.dest / 'instance.yaml').read_text(encoding='utf-8')
        self.assertEqual(system_ids(instance), [('orders', 'Orders'), ('order-returns', 'Returns')])
        self.assertEqual(scalar(instance, 'cell', 'name'), 'Commerce')
        self.assertEqual(scalar(instance, 'cell', 'purpose'), 'Order fulfillment.')
        self.assertEqual(scalar(instance, 'sources', 'github_org'), 'acme')
        self.assertEqual(flow_list(instance, 'sources', 'repo_prefixes'), ['APP01-', 'APP02-'])
        self.assertEqual(trackers(instance), [{'id': 'jira', 'provider': 'jira', 'url': 'https://acme.atlassian.net/jira/software/projects/ORD'}])
        self.assertEqual(scalar(instance, 'evidence', 'profile'), 'production-gate')
        self.assertEqual(scalar(instance, 'locale', 'notes'), 'en')
        self.assertEqual(flow_list(instance, 'sources', 'reference_branch_order'), ['develop', 'main'])
        self.assertEqual(flow_list(instance, 'platform', 'providers'), ['gcp', 'aws'])
        self.assertEqual(top_level(instance, 'adapters'), '[]')
        self.assertFalse((self.dest / '.knowledge-os-config.yaml').exists())

    def test_explicit_yes_and_existing_vault_rejection(self):
        result = self.run_init('', '--yes', '--cell-name', 'Commerce', '--purpose', 'Orders.',
                               '--system', 'orders:Orders', '--evidence-profile', 'mixed', '--locale', 'en')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(p in result.stdout for p in PROMPTS))
        self.assertFalse(any(p in result.stderr for p in PROMPTS))
        before = snapshot(self.dest)
        result = self.run_init('\n'.join(ANSWERS) + '\n')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(snapshot(self.dest), before)
        self.assertNotIn('Cell name', result.stdout)
        self.assertNotIn('Cell name', result.stderr)

    def test_keyboard_interrupt_at_prompt_writes_nothing(self):
        env = {**os.environ, 'KOS_NO_UPDATE_CHECK': '1'}
        process = subprocess.Popen(self.command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, cwd=self.root, env=env)
        try:
            ready, _, _ = select.select([process.stderr], [], [], 10)
            self.assertTrue(ready, 'kos init never reached a prompt')
            prefix = os.read(process.stderr.fileno(), 4096)
            self.assertIn(b'Cell name', prefix)
            process.send_signal(signal.SIGINT)
            stdout, stderr = process.communicate(timeout=10)
            print('\nTRANSCRIPT interrupt\n' + (stdout + prefix + stderr).decode())
            self.assertNotEqual(process.returncode, 0)
            self.assertIn(b'Initialization cancelled', prefix + stderr)
            self.assertNotIn(b'Traceback', prefix + stderr)
            self.assertFalse(self.dest.exists())
        finally:
            if process.poll() is None:
                process.kill()
                process.communicate()


if __name__ == '__main__':
    unittest.main(verbosity=2)
