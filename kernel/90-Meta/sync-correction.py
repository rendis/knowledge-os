#!/usr/bin/env python3
"""One correction workspace before ordinary close-package/checkpoint publication.

This is a local artifact contract, not a security boundary against a writer who
can delete the entire run. Independent reviewer identity remains orchestration's
responsibility, as in the existing review contract.
"""
from __future__ import annotations
import argparse
import importlib.util
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location('manifest_contract', Path(__file__).with_name('git-change-manifest.py'))
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


def safe(path: Path) -> Path:
    path = path.absolute()
    if '..' in path.parts or any(p.is_symlink() for p in (path, *path.parents)):
        raise ValueError('unsafe-path')
    return path


def read(path):
    return contract.read_json(safe(path))


def validate(repo, manifest, scaffold, analysis, review, current_ref):
    result = contract.check_analysis(repo, manifest, scaffold, analysis, current_ref)
    if result['status'] != 'pass':
        raise ValueError(result['code'])
    m, s, a, r = [read(p) for p in (manifest, scaffold, analysis, review)]
    secrets = contract.credential_literals(repo, m)
    finalized, _ = contract.finalize_analysis_payload(a, m, secrets)
    if finalized != a:
        raise ValueError('analysis-not-finalized')
    if contract.validate_review(r, a['repository'], m, s, a, secrets):
        raise ValueError('review-invalid-or-stale')
    return m, s, a, r


def prepare(args):
    paths = [safe(getattr(args, key)) for key in ('manifest', 'scaffold', 'analysis', 'review')]
    repo = safe(args.repo)
    if 'correction-1' in paths[2].parts:
        raise ValueError('correction-limit-reached')
    values = validate(repo, *paths, args.current_ref)
    if values[3]['verdict'] != 'revise':
        raise ValueError('correction-requires-revise')
    workspace = safe(paths[2].parent / 'correction-1')
    if workspace == repo or repo in workspace.parents or any(part in contract.KNOWLEDGE_NODE_ROOTS for part in workspace.parts):
        raise ValueError('source-workspace-forbidden')
    receipt = {'version': 1, 'attempt': 1, 'inputs': dict(zip(('manifest', 'scaffold', 'analysis', 'review'), [contract.canonical_digest(v) for v in values]))}
    # mkdir is the exclusive once-only reservation; interrupted preparation fails
    # closed rather than silently consuming another semantic attempt.
    workspace.mkdir()
    for name, value in zip(('manifest', 'scaffold', 'analysis', 'review'), values):
        target = workspace / ('initial-' + name + '.json')
        target.write_text(json.dumps(value, sort_keys=True) + '\n')
        target.chmod(0o444)
    (workspace / 'analysis.json').write_text(json.dumps(values[2], sort_keys=True) + '\n')
    (workspace / 'receipt.json').write_text(json.dumps(receipt, sort_keys=True) + '\n')
    (workspace / 'receipt.json').chmod(0o444)
    return {'status': 'pass', 'code': 'correction-prepared', 'workspace': str(workspace)}


def scope(initial, revised, review):
    for key in ('version', 'repository', 'old_oid', 'new_oid'):
        if initial[key] != revised[key]:
            raise ValueError('correction-envelope-changed')
    nodes = {node for f in review['findings'] for node in f['nodes']}
    paths = {f['evidence']['path'] for f in review['findings']}
    claims = {f['target'][7:] for f in review['findings'] if f['target'].startswith('claims.')}
    dependencies = set(claims)
    for node in initial['nodes']:
        if node['basename'] in nodes:
            dependencies.update(node['claim_ids'])
    for claim in initial['claims']:
        if claim['claim_id'] in dependencies:
            paths.update(e['path'] for e in claim['evidence'])
    for key, identity, allowed in (('nodes', 'basename', nodes), ('paths', 'path', paths), ('claims', 'claim_id', claims)):
        old = {v[identity]: v for v in initial[key]}
        new = {v[identity]: v for v in revised[key]}
        for item in old.keys() | new.keys():
            if old.get(item) == new.get(item) or item in allowed:
                continue
            # New claims stay attached to finding nodes. Existing validation
            # verifies frozen source evidence; the independent reviewer verifies
            # causal relevance of newly inspected dependencies.
            if key == 'claims' and item not in old and any(item in n['claim_ids'] and n['basename'] in nodes for n in revised['nodes']):
                continue
            raise ValueError('correction-outside-findings')
    for dimension, old in initial['checklist'].items():
        new = revised['checklist'][dimension]
        if old != new and not (any(f['target'].startswith('checklist.' + dimension) for f in review['findings']) or any(e['path'] in paths for e in old['evidence'] + new['evidence'])):
            raise ValueError('correction-outside-findings')


def check(args):
    w = safe(args.workspace)
    if w.name != 'correction-1':
        raise ValueError('invalid-correction-workspace')
    receipt = read(w / 'receipt.json')
    names = ('manifest', 'scaffold', 'analysis', 'review')
    values = [read(w / ('initial-' + name + '.json')) for name in names]
    expected = {'version': 1, 'attempt': 1, 'inputs': dict(zip(names, [contract.canonical_digest(v) for v in values]))}
    if receipt != expected:
        raise ValueError('initial-artifacts-changed')
    validate(safe(args.repo), *[w / ('initial-' + name + '.json') for name in names], args.current_ref)
    if values[3]['verdict'] != 'revise':
        raise ValueError('correction-requires-revise')
    revised = validate(safe(args.repo), w / 'initial-manifest.json', w / 'initial-scaffold.json', w / 'analysis.json', safe(args.review), args.current_ref)
    if revised[2] == values[2] or revised[3] == values[3]:
        raise ValueError('correction-needs-new-analysis-and-review')
    scope(values[2], revised[2], values[3])
    return {'status': 'pass', 'code': 'correction-checked', 'analysis_digest': contract.canonical_digest(revised[2]), 'review_digest': contract.canonical_digest(revised[3])}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    for command in ('prepare', 'check'):
        p = commands.add_parser(command)
        p.add_argument('--repo', type=Path, required=True)
        p.add_argument('--current-ref', required=True)
        p.add_argument('--review', type=Path, required=True)
        for field in (('manifest', 'scaffold', 'analysis') if command == 'prepare' else ('workspace',)):
            p.add_argument('--' + field, type=Path, required=True)
    args = parser.parse_args()
    try:
        result = prepare(args) if args.command == 'prepare' else check(args)
    except (ValueError, OSError, contract.ContractError, contract.OperationalError) as error:
        result = {'status': 'blocked', 'code': str(error)}
    print(json.dumps(result, sort_keys=True))
    return 0 if result['status'] == 'pass' else 1


if __name__ == '__main__':
    raise SystemExit(main())
