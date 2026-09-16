"""Build a compact measured ledger; semantic acceptance is supplied separately."""
import argparse
import json
import statistics
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('campaign', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    results = json.loads((args.campaign / 'results.json').read_text())
    rows = []
    for run in results:
        phases = run['phases']
        usage = run['usage']
        inp, cached = usage['input_tokens'], usage['cached_input_tokens']
        rows.append({
            'id': f"{run['case']}/r{run['repetition']:02d}-{run['variant']}",
            **{k: run[k] for k in ('case', 'repetition', 'variant', 'wall_seconds', 'setup_seconds', 'ok', 'changed')},
            'usage': {**usage, 'uncached_input_tokens': inp - cached if inp is not None and cached is not None else None},
            'review': run['quality_review'], 'phase_count': len(phases),
            'repaired': any(p['phase'] == 'repair' for p in phases),
            'phases': [{k: p[k] for k in ('phase', 'exit', 'error', 'wall_seconds', 'usage')} for p in phases],
        })
    comparisons = []
    for case in sorted({r['case'] for r in rows}):
        groups = {arm: [r for r in rows if r['case'] == case and r['variant'] == arm]
                  for arm in ('baseline', 'alternative')}
        def measures(group):
            result = {'runs': len(group), 'runtime_accepted': sum(r['ok'] for r in group)}
            for metric in ('input_tokens', 'cached_input_tokens', 'uncached_input_tokens', 'output_tokens', 'wall_seconds'):
                values = [r['wall_seconds'] if metric == 'wall_seconds' else r['usage'][metric] for r in group]
                result[metric] = {'median': statistics.median(values), 'total': sum(values)} if all(v is not None for v in values) else None
            return result
        comparisons.append({'case': case, **{arm: measures(group) for arm, group in groups.items()}})
    args.output.write_text(json.dumps({'runs': rows, 'comparisons': comparisons}, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
