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
    if args.task == 'overlay-absent':
        overlay = root / 'vault/.investigations-private/20260908-090000-reader-units'
        shutil.rmtree(overlay)  # exact synthetic fixture, never a consumer
    tasks = ['continuity-1', 'continuity-2', 'continuity-3'] if args.task == 'continuity' else [args.task]
    history = []
    results = []
    for task in tasks:
        prompt = (root / 'tasks' / f'{task}.txt').read_text()
        # Replay only actual conversational messages, never evaluator keys or predictions.
        request = ('Prior conversation:\n' + '\n'.join(history) + '\n\nCurrent request:\n' if history else '') + prompt
        before = fingerprint(root / 'vault')
        result = evaluator / f'{task}-result.md'
        command = ['codex', 'exec', '--ephemeral', '--skip-git-repo-check', '--ignore-user-config',
                   '-m', args.model, '-c', f'model_reasoning_effort="{args.effort}"',
                   '-c', 'approval_policy="never"', '-s', 'workspace-write', '--json',
                   '--output-last-message', str(result), '-']
        with (evaluator / f'{task}-events.jsonl').open('w') as events, (evaluator / f'{task}-stderr.txt').open('w') as errors:
            process = subprocess.run(command, input=request, text=True, cwd=root, stdout=events, stderr=errors)
        after = fingerprint(root / 'vault')
        changed = sorted(k for k in before['files'].keys() | after['files'].keys() if before['files'].get(k) != after['files'].get(k))
        results.append({'task': task, 'exit_code': process.returncode,
                        'prompt_sha256': hashlib.sha256(prompt.encode()).hexdigest(),
                        'changed_vault_paths': changed,
                        'before': before, 'after': after})
        (evaluator / 'observed.json').write_text(json.dumps(results, indent=2))
        if process.returncode:
            raise SystemExit(process.returncode)
        history.extend(['User: ' + prompt, 'Assistant: ' + result.read_text()])
        print(f'{task}: execution complete, {len(changed)} changed vault paths', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--distribution', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--evaluation-output', type=Path, required=True)
    parser.add_argument('--task', choices=['simple', 'diagnosis', 'cloud', 'missing', 'variants', 'continuity', 'overlay', 'overlay-absent', 'document', 'routes', 'operational'], required=True)
    parser.add_argument('--model', required=True)
    parser.add_argument('--effort', default='low')
    run(parser.parse_args())
