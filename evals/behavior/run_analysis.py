"""Run one blind offline analysis scenario; store traces outside executor scope."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import sys

from preserve_checkout import fingerprint


def run(args):
    root = args.output.resolve()
    evaluator = args.evaluation_output.resolve()
    subprocess.run([sys.executable, '-B', str(Path(__file__).with_name('prepare.py')),
                    '--distribution', str(args.distribution.resolve()), '--output', str(root),
                    '--scenario', 'analysis', '--evaluation-output', str(evaluator)], check=True)
    # A confirmed-cause note is an input only to variant hunting, never a diagnosis hint.
    if args.task != 'variants':
        (root / 'sources/review-note.md').unlink()
    if args.task != 'document':
        (root / 'sources/accepted-agreement.md').unlink()
    (root / 'package.md').unlink()
    shutil.rmtree(root / 'worktree')  # unrelated synthetic legacy workflow fixture
    if args.variant != 'original':
        observation = root / 'sources/observations.json'
        data = json.loads(observation.read_text())
        physical, reserved, result = {
            'alternate': (12, 2500, 9.5),
            'fractional': (14, 3250, 10.75),
            'boundary': (18, 2250, 15.75),
        }[args.variant]
        data['request'] = {'physical': physical, 'reserved_milliunits': reserved}
        data['interactive_result'] = result
        observation.write_text(json.dumps(data, indent=2))
        for path in [root / 'sources/incident.md', root / 'tasks/diagnosis.txt']:
            path.write_text(path.read_text().replace('8.5', str(result)))
        rubric = evaluator / 'rubric.json'
        rubric.write_text(rubric.read_text().replace('1500 milliunits', f'{reserved} milliunits'))
    offline_bin = root / 'bin'
    offline_bin.mkdir()
    obsidian = offline_bin / 'obsidian'
    obsidian.write_text('#!/bin/sh\n# Synthetic unavailable registration service; no host access.\nexit 1\n')
    obsidian.chmod(0o755)
    for task_path in (root / 'tasks').glob('*.txt'):
        task_path.write_text(task_path.read_text() + '\nAn offline executable stub is supplied at bin/obsidian. When using the local vault resolver, prepend the absolute bin/ directory to PATH so registration discovery cannot contact the host application.\n')
    tasks = ['continuity-1', 'continuity-2', 'continuity-3'] if args.task == 'continuity' else [args.task]
    requests = {task: (root / 'tasks' / f'{task}.txt').read_text() for task in tasks}
    shutil.rmtree(root / 'tasks')  # requests are supplied only at their actual conversation turn
    (root / 'task.txt').unlink()
    if args.task == 'overlay-absent':
        overlay = root / 'vault/.investigations-private/20260908-090000-reader-units'
        shutil.rmtree(overlay)  # exact synthetic fixture, never a consumer
    hashes = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
              for p in root.rglob('*') if p.is_file() and '.git' not in p.parts}
    (evaluator / 'baseline-hashes.json').write_text(json.dumps(hashes, indent=2, sort_keys=True))
    session_id = None
    results = []
    for task in tasks:
        prompt = requests[task]
        before = fingerprint(root / 'vault')
        result = evaluator / f'{task}-result.md'
        command = ['codex', 'exec', '--skip-git-repo-check', '--ignore-user-config',
                   '-m', args.model, '-c', f'model_reasoning_effort="{args.effort}"',
                   '-c', 'approval_policy="never"', '-s', 'workspace-write', '--json',
                   '--output-last-message', str(result)]
        if args.task != 'continuity':
            command.append('--ephemeral')
        command.extend(['resume', session_id, '-'] if session_id else ['-'])
        with (evaluator / f'{task}-events.jsonl').open('w') as events, (evaluator / f'{task}-stderr.txt').open('w') as errors:
            process = subprocess.run(command, input=prompt, text=True, cwd=root, stdout=events, stderr=errors)
        after = fingerprint(root / 'vault')
        changed = sorted(k for k in before['files'].keys() | after['files'].keys() if before['files'].get(k) != after['files'].get(k))
        results.append({'task': task, 'exit_code': process.returncode,
                        'prompt_sha256': hashlib.sha256(prompt.encode()).hexdigest(),
                        'changed_vault_paths': changed,
                        'before': before, 'after': after})
        (evaluator / 'observed.json').write_text(json.dumps(results, indent=2))
        if process.returncode:
            raise SystemExit(process.returncode)
        if args.task == 'continuity' and session_id is None:
            events = [json.loads(line) for line in (evaluator / f'{task}-events.jsonl').read_text().splitlines() if line.strip()]
            session_id = next(item['thread_id'] for item in events if item.get('type') == 'thread.started')
        print(f'{task}: execution complete, {len(changed)} changed vault paths', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--distribution', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--evaluation-output', type=Path, required=True)
    parser.add_argument('--task', choices=['simple', 'diagnosis', 'cross-source', 'cloud', 'missing', 'variants', 'continuity', 'overlay', 'overlay-absent', 'document', 'routes', 'operational'], required=True)
    parser.add_argument('--model', required=True)
    parser.add_argument('--effort', default='low')
    parser.add_argument('--variant', choices=['original', 'alternate', 'fractional', 'boundary'], default='original')
    run(parser.parse_args())
