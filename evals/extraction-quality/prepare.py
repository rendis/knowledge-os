"""Prepare a blind extraction package using the distribution's production helpers."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def run(*args, cwd=None):
    env = dict(os.environ, GIT_AUTHOR_DATE='2026-09-01T12:00:00Z', GIT_COMMITTER_DATE='2026-09-01T12:00:00Z')
    return subprocess.run(args, cwd=cwd, env=env, check=True, text=True, capture_output=True).stdout


def write(root, name, body):
    p = root / name
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body)
    return p


def commit(repo, message):
    run('git', 'add', '.', cwd=repo)
    run('git', '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', message, cwd=repo)
    return run('git', 'rev-parse', 'HEAD', cwd=repo).strip()


def prepare(dist, root):
    if root.exists():
        raise ValueError('Output must be a fresh directory')
    root.mkdir(parents=True)
    vault = root / 'vault'
    run('python3', '-B', str(dist / 'scripts/knowledge_os.py'), 'init', '--dest', str(vault), '--cell-name', 'Cedar Fulfilment', '--purpose', 'Fulfilment contracts', '--system', 'cedar:Cedar', '--evidence-profile', 'documented-source', '--locale', 'en', '--yes')
    producer = root / 'sources/availability-service'
    producer.mkdir(parents=True)
    run('git', 'init', '-q', '-b', 'main', cwd=producer)
    run('git', 'remote', 'add', 'origin', 'https://example.invalid/availability-service.git', cwd=producer)
    write(producer, 'availability.py', 'def available(physical, reserved_milliunits):\n    if physical is None:\n        return {"available": None, "observed": False}\n    return {"available": max(0, physical - round(reserved_milliunits / 1000)), "observed": True}\n')
    write(producer, 'routes.py', 'from availability import available\nROUTES = {"GET /availability": available}\nEVENT_SINKS = ["legacy-audit"]\n')
    write(producer, 'config/production.json', '{"reservation_authority": "ledger-primary", "timeout_ms": 2500}\n')
    write(producer, 'config/preview.json', '{"reservation_authority": "ledger-sandbox", "timeout_ms": 2500}\n')
    write(producer, 'contracts/availability.md', '# Availability response\nGET /availability returns available (units, nullable number) and observed (boolean). Missing physical measurement is null/false; measured zero is 0/true. Ledger owns reservation identities; local availability cache is a replica.\n')
    write(producer, 'contracts/dispatch-consumer.md', '# Dispatch consumer contract\nDispatch calls GET /availability. It requests integer saleable units and allocates exactly when observed is true and available >= requested_units; no rounding occurs in the consumer.\n')
    old = commit(producer, 'fixture: previous availability contract')
    write(producer, 'availability.py', 'def available(physical, reserved_milliunits):\n    if physical is None:\n        return {"available": None, "observed": False}\n    return {"available": max(0, physical - reserved_milliunits / 1000), "observed": True}\n')
    write(producer, 'routes.py', 'from availability import available\nROUTES = {"GET /availability": available}\nEVENT_SINKS = []\n')
    write(producer, 'config/production.json', '{"reservation_authority": "ledger-secondary", "timeout_ms": 2500}\n')
    write(producer, 'config/preview.json', '{"reservation_authority": "ledger-experiment", "timeout_ms": 2500}\n')
    new = commit(producer, 'fix: preserve fractional reservations and remove legacy sink')
    consumer = root / 'sources/dispatch-service'
    consumer.mkdir()
    run('git', 'init', '-q', '-b', 'main', cwd=consumer)
    run('git', 'remote', 'add', 'origin', 'https://example.invalid/dispatch-service.git', cwd=consumer)
    write(consumer, 'dispatch.py', 'AVAILABILITY_ROUTE = "GET /availability"\ndef can_allocate(response, requested_units):\n    return response["observed"] and response["available"] >= requested_units\n')
    write(consumer, 'README.md', '# Dispatch\nCalls availability-service GET /availability. Each request is an integer count of saleable units. No rounding or unit conversion occurs here.\n')
    consumer_oid = commit(consumer, 'fixture: unchanged dispatch consumer')
    notes = {
        '20-Repos/cedar/availability-service.md': '# availability-service\nGET /availability subtracts rounded whole-unit reservations from measured stock, clamped at zero. Publishes to [[legacy-audit]]. Missing measurement is null/false; measured zero is 0/true. Ledger owns reservation identities; the local cache is a replica. Source inspected at the previous commit; deployment not inspected.\n',
        '20-Repos/cedar/dispatch-service.md': '# dispatch-service\nConsumes [[availability-service]] GET /availability and allocates when observed is true and available >= requested units. Requests are integer units.\n',
        '30-Flujos/Allocation.md': '# Allocation\n[[dispatch-service]] reads [[availability-service]]. Reservation quantities are rounded to whole units before subtraction, allowing requests up to that computed stock. Missing measurement requires a warehouse observation.\n',
        '40-Integraciones/legacy-audit.md': '# legacy-audit\nReceives availability-service events. Also receives invoice-service events, established by its independently inspected contract; invoice-service is outside this package.\n',
        '60-Operacion/Reservation routing.md': '# Reservation routing\nPrevious versioned production config names ledger-primary and preview names ledger-sandbox. No runtime routing or deployment receipt has been observed. Timeout is 2500 ms in both.\n'
    }
    context = [str(write(vault, name, body)) for name, body in notes.items()]
    package = root / 'package'
    package.mkdir()
    helper = vault / '90-Meta/git-change-manifest.py'
    manifest = write(package, 'manifest.json', run('python3', '-B', str(helper), 'build', '--repo', str(producer), '--old', old, '--new', new))
    scaffold = write(package, 'scaffold.json', run('python3', '-B', str(helper), 'init-analysis', '--manifest', str(manifest), '--repository', 'availability-service', '--node', 'availability-service', '--node', 'Allocation', '--node', 'legacy-audit', '--node', 'Reservation routing'))
    card = dict(marker='SYNC_PACKAGE_WORKER_V1', role='extractor', evidence_profile='documented-source', run_root=str(package), repository='availability-service', source_checkout=str(producer), production_ref='refs/heads/main', old_oid=old, new_oid=new, manifest=str(manifest), scaffold=str(scaffold), analysis=str(package/'analysis.json'), review=None, check_result=None, vault_context=context, search_roots=[str(consumer)], allowed_write=str(package/'analysis.json'))
    write(package, 'worker-card.json', json.dumps(card, indent=2)+'\n')
    write(root, 'source-bindings.json', json.dumps({'producer_old': old, 'producer_new': new, 'consumer_oid': consumer_oid}, indent=2)+'\n')
    write(root, 'task.txt', f'''Use the installed synchronize-ecosystem package-worker instructions at {vault}/.agents/skills/synchronize-ecosystem/SKILL.md. Complete the extractor assignment in {package}/worker-card.json. Read its referenced sources/context at the frozen revisions and produce the production analysis artifact at allowed_write. The consumer search root is unchanged and frozen at {consumer_oid}. Do not change sources, vault, scaffold, manifest or instructions. This is offline versioned evidence; no live system access is available. Do not read other evaluation directories, preparation scripts, oracle rubrics, memories, or internet sources. Do not delegate. Return the artifact path and any contract blocker.\n''')
    return root/'task.txt'


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--distribution', required=True, type=Path)
    p.add_argument('--output', required=True, type=Path)
    a = p.parse_args()
    print(prepare(a.distribution.resolve(), a.output.resolve()))
