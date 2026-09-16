"""Retain compact answer/usage/command evidence without repeated instruction dumps."""
import argparse
import hashlib
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('campaign', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    base = args.campaign.resolve()
    manifest = json.loads((base / 'campaign.json').read_text())
    results = json.loads((base / 'results.json').read_text())
    def clean(text):
        return text.replace(str(base), '${CAMPAIGN}').replace(str(base).replace('/private/tmp/', '/tmp/'), '${CAMPAIGN}')
    rows = []
    for run in results:
        key = f"{run['case']}/r{run['repetition']:02d}-{run['variant']}"
        evidence = base / 'evidence' / key
        record = {k: v for k, v in run.items() if k != 'phases'}
        record['id'] = key
        record['answer'] = clean((evidence / 'answer.md').read_text())
        record['phases'] = []
        for phase in run['phases']:
            path = evidence / phase['phase']
            item = {k: v for k, v in phase.items() if k != 'command'}
            item['answer'] = clean((path / 'answer.md').read_text()) if (path / 'answer.md').exists() else None
            events = path / 'events.jsonl'
            item['events_sha256'] = hashlib.sha256(events.read_bytes()).hexdigest()
            item['prompt_sha256'] = hashlib.sha256((path / 'prompt.txt').read_bytes()).hexdigest()
            item['commands'] = []
            for line in events.read_text().splitlines():
                event = json.loads(line)
                command = event.get('item', {})
                if event.get('type') == 'item.completed' and command.get('type') == 'command_execution':
                    output = command.get('aggregated_output', '')
                    item['commands'].append({'command': clean(command['command']), 'exit': command.get('exit_code'),
                                             'output_sha256': hashlib.sha256(output.encode()).hexdigest()})
            record['phases'].append(item)
        rows.append(record)
    manifest['distribution'] = '${FROZEN_DISTRIBUTION}'
    args.output.write_text(json.dumps({'manifest': manifest, 'runs': rows,
        'evidence_note': 'Command output hashes refer to retained raw captures; fixture sources and final answers are available for independent scoring. No repeated instruction dumps are included.'}, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
