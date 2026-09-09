#!/usr/bin/env python3
"""Redact credential literals from reviewer prose without changing its decisions."""
from __future__ import annotations
import argparse
import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile

spec = importlib.util.spec_from_file_location('manifest_contract', Path(__file__).with_name('git-change-manifest.py'))
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


def finalize(args):
    inputs = [getattr(args, key).resolve() for key in ('manifest', 'scaffold', 'analysis', 'review')]
    output = args.output.absolute()
    if output.is_symlink() or output.resolve() in inputs or args.repo.resolve() in output.resolve().parents:
        raise ValueError('output-conflicts-with-input')
    manifest, scaffold, analysis, review = [contract.read_json(p) for p in inputs]
    paths, environments, suspects, issues = contract.validate_manifest(manifest)
    issues += contract.repository_checkout_issues(args.repo, analysis.get('repository'))
    if issues:
        raise ValueError('analysis-invalid-or-stale')
    issues += contract.source_manifest_issues(args.repo, manifest)
    seed_nodes, scaffold_issues = contract.validate_scaffold(scaffold, manifest)
    issues += scaffold_issues
    literals = contract.credential_literals(args.repo, manifest)
    if not issues:
        issues += contract.validate_analysis(analysis, manifest, paths, environments, suspects,
                                            literals, scaffold, seed_nodes)
        issues += contract.source_evidence_issues(args.repo, manifest, analysis)
    head = contract.run_git(args.repo, 'rev-parse', '--verify', 'HEAD')
    if issues or head.returncode or head.stdout.decode().strip() != manifest.get('new_oid'):
        raise ValueError('analysis-invalid-or-stale')
    finalized = copy.deepcopy(review)
    changed = []
    findings = finalized.get('findings')
    if isinstance(findings, list):
        for index, finding in enumerate(findings):
            if not isinstance(finding, dict):
                continue
            surfaces = [(finding, 'reason', f'findings[{index}].reason')]
            if isinstance(finding.get('evidence'), dict):
                surfaces.append((finding['evidence'], 'anchor', f'findings[{index}].evidence.anchor'))
            for container, key, field in surfaces:
                value = container.get(key)
                if isinstance(value, str):
                    redacted = contract.redact_credential_literals(value, literals)
                    if redacted != value:
                        container[key] = redacted
                        changed.append(field)
    # Metadata and paths are immutable; exposures there block instead of being rewritten.
    if contract.credential_exposure_issues(finalized, literals):
        raise ValueError('credential-exposure-outside-review-prose')
    issues = contract.validate_review(finalized, analysis['repository'], manifest, scaffold, analysis, literals)
    issues += contract.source_evidence_issues(args.repo, manifest, analysis, finalized)
    if issues:
        raise ValueError('review-invalid-or-stale')
    payload = (json.dumps(finalized, ensure_ascii=False, sort_keys=True, indent=2) + '\n').encode()
    # Publish an already complete temporary file using a no-clobber atomic link.
    output.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix='.review-', dir=output.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(payload)
            stream.flush()
            os.fsync(stream.fileno())
        try:
            os.link(temporary, output)
        except FileExistsError:
            if output.is_symlink() or not output.is_file() or output.read_bytes() != payload:
                raise ValueError('output-already-different')
    finally:
        os.unlink(temporary)
    return {'status': 'pass', 'input_review_digest': contract.canonical_digest(review),
            'output_review_digest': contract.canonical_digest(finalized),
            'manifest_digest': contract.canonical_digest(manifest), 'scaffold_digest': contract.canonical_digest(scaffold),
            'analysis_digest': contract.canonical_digest(analysis), 'redacted_fields': changed}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ('repo', 'manifest', 'scaffold', 'analysis', 'review', 'output'):
        parser.add_argument('--' + key, type=Path, required=True)
    args = parser.parse_args()
    try:
        receipt = finalize(args)
    except (ValueError, OSError, contract.ContractError, contract.OperationalError):
        # Never reflect arbitrary artifact content or exception messages.
        print(json.dumps({'status': 'blocked', 'code': 'review-finalization-invalid'}))
        return 2
    print(json.dumps(receipt, sort_keys=True))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
