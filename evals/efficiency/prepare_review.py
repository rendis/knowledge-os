"""Prepare arm-blinded answer bundles after a campaign; retain mapping privately."""
import argparse
import hashlib
import json
import random
import re
from pathlib import Path


def normalize(text):
    return re.sub(r'/private/tmp/dv-efficiency-[^\s)\]"<>]*?/vault/', '${VAULT}/', text)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('campaign', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    results = json.loads((args.campaign / 'results.json').read_text())
    cases = {c['id']: c for c in json.loads((args.campaign / 'campaign.json').read_text())['cases']}
    fixture = Path(__file__).resolve().parents[1] / 'graph-economy/fixtures/norte'
    evidence = '\n\n'.join('FILE: ' + str(p.relative_to(fixture)) + '\n' + p.read_text()
                           for p in sorted(fixture.rglob('*.md')))
    frozen = Path(json.loads((args.campaign / "campaign.json").read_text())["distribution"]) / "kernel"
    evidence += "\n\nCOMMON SCOPE: No real network/application access or reads outside the fixture are authorized. No repository roots are configured. Obsidian is disconnected through a fixture stub. These policies are installed unchanged in every fixture.\n"
    for name in ("90-Meta/Auditoria - Framework.md", "90-Meta/vault-resolution.md", "90-Meta/response-quality.md", ".agents/skills/manage-investigation/references/readiness-and-lifecycle.md"):
        evidence += "\nFILE: " + name + "\n" + (frozen / name).read_text()
    rows, mapping = [], {}
    for run in results:
        key = f"{run['case']}/r{run['repetition']:02d}-{run['variant']}"
        opaque = hashlib.sha256(('blind-20260916-' + key).encode()).hexdigest()[:12]
        answer = (args.campaign / 'evidence' / key / 'answer.md').read_text()
        rows.append({'id': opaque, 'case': run['case'], 'question': cases[run['case']]['prompt'], 'answer': normalize(answer)})
        mapping[opaque] = key
    random.Random(916).shuffle(rows)
    args.output.mkdir(parents=True, exist_ok=True)
    (args.output / 'mapping-private.json').write_text(json.dumps(mapping, indent=2) + '\n')
    (args.output / 'sources.md').write_text(evidence)
    for index in range(0, len(rows), 12):
        (args.output / f'batch-{index//12+1}.json').write_text(json.dumps(rows[index:index+12], ensure_ascii=False, indent=2) + '\n')


if __name__ == '__main__':
    main()
