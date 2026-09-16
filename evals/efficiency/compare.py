"""Compare paired outcomes with semantic scores and retain failed-run expenditure."""
import argparse
import json
import statistics
from pathlib import Path

METRICS = ('input_tokens', 'cached_input_tokens', 'uncached_input_tokens', 'output_tokens', 'wall_seconds')


def value(row, metric):
    return row['wall_seconds'] if metric == 'wall_seconds' else row['usage'][metric]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ledger', type=Path, required=True)
    parser.add_argument('--scores', type=Path, required=True, help='JSON list: id, accepted, reason, trace_verified; IDs match ledger')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    runs = json.loads(args.ledger.read_text())['runs']
    scores = {s['id']: s for s in json.loads(args.scores.read_text())}
    if set(scores) != {r['id'] for r in runs}:
        raise ValueError('Semantic scores must cover exactly every outcome')
    for row in runs:
        row['semantic'] = scores[row['id']]
        row['trace_verified'] = scores[row['id']].get('trace_verified')
        row['accepted'] = row['ok'] and scores[row['id']]['accepted'] is True and row['trace_verified'] is True
    comparisons = []
    for case in sorted({r['case'] for r in runs}):
        group = [r for r in runs if r['case'] == case]
        arms = {arm: [r for r in group if r['variant'] == arm] for arm in ('baseline', 'alternative')}
        cost = {}
        for arm, rows in arms.items():
            passed = sum(r['accepted'] for r in rows)
            cost[arm] = {'accepted': passed, 'runs': len(rows)}
            for metric in METRICS:
                values = [value(r, metric) for r in rows]
                total = sum(values) if all(v is not None for v in values) else None
                cost[arm][metric] = {'total': total, 'per_accepted': total / passed if passed and total is not None else None}
        pairs = []
        for repetition in sorted({r['repetition'] for r in group}):
            pair = {r['variant']: r for r in group if r['repetition'] == repetition}
            comparable = len(pair) == 2 and all(r['accepted'] and all(value(r, m) is not None for m in METRICS) for r in pair.values())
            deltas = {m: value(pair['alternative'], m) - value(pair['baseline'], m) for m in METRICS} if comparable else None
            percentages = {m: 100 * (value(pair['alternative'], m) / value(pair['baseline'], m) - 1) if value(pair['baseline'], m) else None for m in METRICS} if comparable else None
            pairs.append({'repetition': repetition, 'comparable': comparable, 'alternative_minus_baseline': deltas, 'percent_change': percentages})
        valid = [p for p in pairs if p['comparable']]
        medians = {m: statistics.median(p['alternative_minus_baseline'][m] for p in valid) for m in METRICS} if valid else None
        pct = {m: statistics.median(p['percent_change'][m] for p in valid) if all(p['percent_change'][m] is not None for p in valid) else None for m in METRICS} if valid else None
        comparisons.append({'case': case, 'all_attempts': cost, 'pairs': pairs, 'median_paired_delta': medians, 'median_paired_percent_change': pct})
    args.output.write_text(json.dumps({'runs': runs, 'comparisons': comparisons}, ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
