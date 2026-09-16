"""Prepare an offline consumer; evaluator material never enters its workspace."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(root, name, value):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(value, encoding="utf-8")


def prepare(distribution, worker, evaluator):
    worker, evaluator = Path(worker).resolve(), Path(evaluator).resolve()
    if worker == evaluator or worker in evaluator.parents or evaluator in worker.parents:
        raise ValueError("Worker and evaluator directories must be disjoint")
    if worker.exists() or evaluator.exists():
        raise ValueError("Use fresh output directories")
    worker.mkdir(parents=True)
    subprocess.run([
        "python3", "-B", str(Path(distribution) / "scripts/knowledge_os.py"),
        "init", "--dest", str(worker / "vault"), "--cell-name", "Aster",
        "--purpose", "Offline inventory evidence", "--system", "aster:Aster",
        "--evidence-profile", "documented-source", "--locale", "en", "--yes",
    ], check=True, capture_output=True, text=True)
    sources = {
        "sources/stores.json": json.dumps({
            "environment": "lab", "captured_at": "2026-09-10T12:00:00Z",
            "rows": [
                {"id": "S101", "provider": "A", "reader_flag": 1, "stock_enabled": 0},
                {"id": "S202", "provider": "B", "reader_flag": 1, "stock_enabled": 1},
            ],
        }, indent=2),
        "sources/report.csv": "store_id,display_label\nS101,no recent load\nS202,no recent load\n",
        "sources/service.py": '''"""Complete synthetic dispatcher source snapshot, revision fixture-v1."""

def get_reader_stores(connection):
    return connection.execute("SELECT id FROM stores WHERE reader_flag = 1").fetchall()


def send_a(connection):
    return connection.execute("SELECT id FROM stores WHERE provider = 'A'").fetchall()


def send_b(connection):
    return connection.execute("SELECT id FROM stores WHERE provider = 'B' AND stock_enabled = 1").fetchall()


JOBS = {"send-A": send_a, "send-B": send_b}


def dispatch(job, connection):
    return JOBS[job](connection)
''',
        "sources/runtime.json": json.dumps({
            "environment": "lab", "job": "send-A", "revision": "fixture-v1",
            "observed_at": "2026-09-10T12:05:00Z", "status": "completed",
            "store_ids_logged": False, "delivery_receipts_available": False,
            "interval": ["2026-09-10T12:00:00Z", "2026-09-10T12:06:00Z"],
        }, indent=2),
        "sources/scope.md": "# Export scope\nAll files are synthetic. The service file contains the complete local dispatcher and its job registry at fixture-v1. The store export and runtime event cover lab only at the stated times. There is no live connection, current snapshot, per-store trace, or delivery receipt in this package.\n",
    }
    for name, content in sources.items():
        write(worker, name, content)
    evaluation = json.loads((HERE / "rubric.json").read_text())
    evaluation["source_sha256"] = {name: digest(worker / name) for name in sources}
    evaluation["prior_review_injection"] = {
        "synthetic_fixture_record": True,
        "review_status": "accepted",
        "reviewer": "fixture-independent-reviewer",
        "scope": {"environment": "lab", "revision": "fixture-v1", "store": "S101",
                  "snapshot": "2026-09-10T12:00:00Z"},
        "conclusion": "At the supplied lab snapshot, S101 belongs to provider A. send-A selects provider A stores without filtering reader_flag or stock_enabled. This establishes source-level selection, not delivery to S101.",
        "evidence_sha256": {name: evaluation["source_sha256"][name] for name in
                            ("sources/stores.json", "sources/service.py")},
        "reviewed_criteria": ["support", "scope", "sufficiency", "continuity", "clarity"],
    }
    write(evaluator, "evaluation.json", json.dumps(evaluation, indent=2) + "\n")
    write(worker, "task.txt", "Read vault/AGENTS.md and follow the installed framework. This is an offline, read-only question using sources/. Do not read outside this workspace or use live services. Subagents must use the same boundary. Do not modify sources or the framework.\n\n" + evaluation["turns"][0]["prompt"] + "\n")
    return evaluation


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--worker", required=True, type=Path)
    parser.add_argument("--evaluator", required=True, type=Path)
    args = parser.parse_args()
    prepare(HERE.parents[1], args.worker, args.evaluator)
