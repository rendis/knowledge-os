"""Prepare an offline flow-answer fixture in an installed consumer."""

import argparse
import hashlib
import json
from pathlib import Path
import subprocess


HERE = Path(__file__).resolve().parent


def write(root, name, content):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def run(*args, cwd=None):
    subprocess.run(args, cwd=cwd, check=True, capture_output=True, text=True)


def prepare(distribution, worker, evaluator):
    distribution = Path(distribution).resolve()
    worker, evaluator = Path(worker).resolve(), Path(evaluator).resolve()
    if worker == evaluator or worker in evaluator.parents or evaluator in worker.parents:
        raise ValueError("Worker and evaluator directories must be disjoint")
    if worker.exists() or evaluator.exists():
        raise ValueError("Use fresh output directories")

    worker.mkdir(parents=True)
    run(
        "python3", "-B", str(distribution / "scripts/knowledge_os.py"),
        "init", "--dest", str(worker / "vault"), "--cell-name", "Boreal",
        "--purpose", "Synthetic exit pass evidence", "--system", "boreal:Boreal",
        "--evidence-profile", "documented-source", "--locale", "en", "--yes",
    )

    files = {
        "vault/30-Flujos/Exit pass.md": '''---
tipo: flujo
sistema: "[[Boreal]]"
---
# Exit pass

## Documented sequence

1. A mobile client uses QR endpoints compatible with [[exit-http]]. Gateway routing is #por-confirmar.
2. [[exit-http]] forwards validation, association and burn requests to [[exit-bff]], which forwards them to [[exit-adapter]].
3. Validation checks the local high-value product table by EAN and can query Catalog. A qualifying product gets a generated QR; a rejected validation returns HTTP 422.
4. The intended store sequence is to associate a ticket with the QR, then scan and burn it at exit. The burn operation registers the exit status.

## Component view

`mobile client -- gateway #por-confirmar --> exit-http --> exit-bff --> exit-adapter --> Catalog`.
The adapter writes QR events to a trace store.
''',
        "vault/20-Repos/Boreal/exit-http.md": '''---
tipo: servicio
sistema: "[[Boreal]]"
participa-en: ["[[Exit pass]]"]
---
# exit-http

Forwards the three QR business endpoints to [[exit-bff]].
''',
        "vault/20-Repos/Boreal/exit-bff.md": '''---
tipo: servicio
sistema: "[[Boreal]]"
participa-en: ["[[Exit pass]]"]
---
# exit-bff

Forwards the three QR business endpoints to [[exit-adapter]].
''',
        "vault/20-Repos/Boreal/exit-adapter.md": '''---
tipo: adapter
sistema: "[[Boreal]]"
repo: https://example.invalid/boreal/exit-adapter.git
participa-en: ["[[Exit pass]]"]
---
# exit-adapter

The configured checkout contains the source for validation, association and burn in `flow.py`.
''',
        "repos/exit-adapter/flow.py": '''"""QR adapter domain operations."""


def validate_ean(ean, store_active, local_products, catalog_lookup, traces):
    if not store_active:
        return {"status": 422, "reason": "invalid_store"}
    product = local_products.get(ean)
    if product is None:
        product = catalog_lookup(ean)
        if product is None or not product["high_value"]:
            return {"status": 422, "reason": "not_high_value"}
    qr = "Q-" + ean
    traces.append((qr, "generated"))
    return {"status": 201, "qr": qr}


def associate(qr, ticket, traces):
    traces.append((qr, "associated", ticket))
    return {"status": 200, "qr": qr}


def burn(qr, traces):
    traces.append((qr, "burned"))
    return {"status": 200, "qr": qr}
''',
        "sources/scope.md": '''# Package inventory

This offline synthetic package contains the installed vault and one configured adapter checkout. It has no live service connector or production data.
''',
    }
    for name, content in files.items():
        write(worker, name, content)

    repo = worker / "repos/exit-adapter"
    for args in (
        ("git", "init", "-q"),
        ("git", "remote", "add", "origin", "https://example.invalid/boreal/exit-adapter.git"),
        ("git", "config", "user.name", "Synthetic Source"),
        ("git", "config", "user.email", "source@example.invalid"),
        ("git", "add", "."),
        ("git", "commit", "-qm", "test: seed synthetic flow"),
    ):
        run(*args, cwd=repo)
    run(
        "python3", "-B", str(worker / "vault/90-Meta/workspace-config.py"),
        "--vault-root", str(worker / "vault"),
        "--repository-root", str(worker / "repos"), "initialize",
    )

    evaluation = json.loads((HERE / "rubric.json").read_text(encoding="utf-8"))
    evaluation["source_sha256"] = {
        name: hashlib.sha256((worker / name).read_bytes()).hexdigest()
        for name in files
    }
    evaluation["repository_commit"] = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=repo, check=True,
        capture_output=True, text=True,
    ).stdout.strip()
    evaluator.mkdir(parents=True)
    write(evaluator, "evaluation.json", json.dumps(evaluation, indent=2) + "\n")
    boundary = (
        "Read vault/AGENTS.md and follow the installed framework. Work only in this "
        "offline synthetic workspace. Do not use live services, the network, memory, "
        "other runs or evaluator material. Do not modify sources or framework files. "
        "Use only the configured repository binding before reading repository code.\n\n"
    )
    write(worker, "task.txt", boundary + evaluation["turns"][0]["prompt"] + "\n")
    return evaluation


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--worker", required=True, type=Path)
    parser.add_argument("--evaluator", required=True, type=Path)
    args = parser.parse_args()
    prepare(HERE.parents[1], args.worker, args.evaluator)
