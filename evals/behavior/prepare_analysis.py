"""Synthetic sources and blind prompts; evaluation keys stay outside the run."""
from pathlib import Path
import hashlib
import json
import subprocess


CASE = "20260908-090000-reader-units"


def write(root, name, content):
    path = root / name
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)


def prepare(root: Path, evaluation_output: Path):
    evaluation_output = evaluation_output.resolve()
    if evaluation_output == root or root in evaluation_output.parents:
        raise ValueError("Evaluation keys must be outside the executor directory")
    if evaluation_output.exists():
        raise ValueError("Evaluation output already exists")
    sources = {
        "repos/reader/availability.py": "def available(physical, reserved_milliunits):\n    return None if physical is None else max(0, physical - reserved_milliunits / 1000)\n",
        "repos/reader/batch.py": "def available(physical, reserved_milliunits):\n    return max(0, physical - reserved_milliunits)\n",
        "repos/reader/recount.py": "def recount_available(physical, ledger):\n    return max(0, physical - ledger['reserved_milliunits'])\n",
        "repos/reader/preview.py": "def preview(physical, reserved_units):\n    return max(0, physical - reserved_units)\n",
        "repos/reader/CONTRACT.md": "# Ledger boundary\nLedger reports reserved_milliunits. Interactive, batch and recount callers receive that field unchanged. Preview receives already converted reserved_units. Physical stock is measured in units. This source describes implementation, not deployment.\n",
        "sources/observations.json": json.dumps({"captured_at": "2026-09-14T10:00:00Z", "environment": "lab", "request": {"physical": 10, "reserved_milliunits": 1500}, "interactive_result": 8.5, "batch_result": 0, "revision": "local synthetic checkout"}, indent=2),
        "sources/cloud-page.json": json.dumps({"environment": "lab", "observed_at": "2026-09-14T10:05:00Z", "items": [{"request": "C-17", "http_status": 202, "accepted": True}], "nextPageToken": "synthetic-page-2", "persistence_receipts": [], "receipt_query_executed": False}, indent=2),
        "sources/incident.md": "# Incident report\nOperations suspects integer rounding: the batch screen shows 0 while the interactive screen shows 8.5 for the attached observation. This is a report, not a confirmed diagnosis. No production export is available.\n",
        "sources/review-note.md": "# Review note\nConfirmed defect: batch.py subtracts milliunits as units. Search related implementations for the same dimensional mistake; similar subtraction alone is not proof.\n",
        "sources/accepted-agreement.md": "# Accepted agreement\nThe requester agrees that the reader-units explanation is sufficient for the observed example. Add only this newly supplied limitation: historical reservation adjustments before 2026-09-01 were not sampled. This records a supplied observation; no fresh source investigation is requested.\n",
        "vault/20-Repos/cedar/batch-reader.md": "---\ntipo: libreria\nsistema: \"[[Cedar]]\"\nrepo: https://example.invalid/cedar/reader.git\n---\n# Batch reader\nHistorical explanation: rounding may explain the discrepancy. Not verified. Repository identity: https://example.invalid/cedar/reader.git; resolve through the configured workspace. Observations are in ../sources relative to the vault. Ledger supplies milliunits; compare callers before concluding.\n",
        "vault/60-Operacion/Offline/Offline.md": "---\ntipo: indice\ntags: [moc, operacion]\n---\n# Offline\n\n- [[Cedar - Offline processing audit]]\n",
        "vault/60-Operacion/Offline/Cedar - Offline processing audit.md": """---
tipo: operacional
clase: procedimiento
estado: vigente
owner: synthetic-evaluation-team
ultima-verificacion: 2026-09-14
area: "[[Offline]]"
canales: [local-files]
tags: [operacion, operacion/procedimiento, operacion/area/offline]
---
# Cedar - Offline processing audit

## Objetivo
Audit the evidence available in one offline snapshot for acceptance and persistence of a request. This procedure validates only the synthetic export, never current cloud state.

## Disparador
An explicit request to audit a named request and lab interval using the offline export.

## Entradas requeridas
Request C-17; lab; interval [2026-09-14T10:00:00Z, 2026-09-14T10:10:00Z), timezone UTC; source ../sources/cloud-page.json relative to the vault. Source capture is 2026-09-14T10:05:00Z. Correlate by request ID. The only permitted executor is local read-only file inspection; no connection or adapter session is needed.

## Decisiones
HTTP 202 establishes accepted submission only. Verified persistence requires a matching receipt. A nonempty nextPageToken means incomplete event coverage. This offline exercise intentionally supplies only page one and no receipt query; both unavailable observations are terminal unknowns for this bounded export audit, not proof of absence. No retry, replay or correction is authorized.

## Pasos
| ID | Acción | Capacidad | Efecto externo | Entrada | Salida | Continuar si |
|---|---|---|---|---|---|---|
| OFF-01 | Read and bind export scope | local-files | no | snapshot and requested filters | capture, target and coverage | source scope matches |
| OFF-02 | Correlate acceptance and persistence | local-files | no | C-17 records | evidence-labeled findings | each question has finding or terminal unknown |

## Evidencia de finalización
Record procedure, branch Audit, scope, source/capture, filters, steps, acceptance finding and persistence/coverage limitations in the local run ledger. Completion means all offline questions have findings or the declared terminal unknowns. It does not mean business success. Retain only concise sanitized summaries and the source path in .operations/; no extra report or copied payload is needed.

## Fallos y recuperación
An absent or unreadable snapshot is blocked and requires the source file; do not obtain it from live systems. A scope mismatch is blocked until the correct offline snapshot is supplied.

## Limitaciones
Synthetic lab export only. No production evidence, complete event listing or authoritative persistence query exists in this fixture.
""",
        f"vault/.investigations-private/{CASE}/private.md": f"---\nid: {CASE}\nauthority: private-overlay\nupdated-at: 2026-09-14T10:00:00+00:00\n---\n\n## Sensitive context\n\nRestricted coordination detail needed to resume the case: contact the synthetic incident liaison Mira Vale through the internal room Cedar-Quartz. This is private contact context, not evidence or approval.\n\n## Private references\n\nInternal coordination note supplied by the requester; do not include its contact details in shared output.\n\n## History\n\nFixture seed; historical recorder unknown.\n",
    }
    for name, content in sources.items():
        write(root, name, content)
    repo = root / "repos/reader"
    for args in (["git", "init", "-q"], ["git", "remote", "add", "origin", "https://example.invalid/cedar/reader.git"], ["git", "config", "user.name", "Synthetic Source"], ["git", "config", "user.email", "source@example.invalid"], ["git", "add", "."], ["git", "commit", "-qm", "test: seed synthetic reader"]):
        subprocess.run(args, cwd=repo, check=True, capture_output=True)
    subprocess.run(["python3", "-B", str(root / "vault/90-Meta/workspace-config.py"), "--vault-root", str(root / "vault"), "--repository-root", str(root / "repos"), "initialize"], check=True, capture_output=True)
    with (root / "vault/60-Operacion/Operacion.md").open("a") as index:
        index.write("\n- [[Offline]]\n")
    prompts = {
        "simple": "Why does Cedar dispatch treat a missing physical observation differently from an observed zero? Explain briefly using this vault. This is just a question.",
        "diagnosis": "The batch display says zero but the interactive display says 8.5. People think rounding caused it. Work out what the available sources support. The incident material is in sources/; source repositories are in repos/. I only want the diagnosis.",
        "cloud": "From sources/cloud-page.json, can we conclude every C-17 event was processed and persisted successfully in lab? Evaluate that claim and tell me what you can establish. This is an informational question, not a request to start an operational audit run.",
        "operational": "Perform the Cedar - Offline processing audit for C-17 in lab, from 2026-09-14T10:00:00Z inclusive to 2026-09-14T10:10:00Z exclusive (UTC), using the registered procedure and supplied export. Complete the audit and give its findings. No processing, correction or live access is requested.",
        "missing": "Does the batch discrepancy also happen in production? Use what is available here, and tell me what else you need if you cannot establish it.",
        "variants": "The dimensional mistake in sources/review-note.md is confirmed. Check the other implementations for the same problem and explain which matches are real.",
        "continuity-1": "We will revisit the batch discrepancy next week with another engineer; we still need production observations and an agreement on the fix scope. What should we do next? For now I am asking for advice.",
        "continuity-2": "Keep this conversational for now. What exact production observation would help us next?",
        "continuity-3": "Now open an investigation to preserve the batch discrepancy, the source-backed explanation, and the production verification and scope agreement still pending. No implementation is requested.",
        "overlay": "In investigation reader-units, who was the internal liaison and which coordination room should I use to resume? Answer for my local use; do not change any files. Also give a separate shareable summary of the technical conclusion without internal coordination details.",
        "overlay-absent": "In investigation reader-units, who was the internal liaison and which coordination room should I use to resume? Answer from the available case context; do not change any files.",
        "document": "Update reader-units with the single new limitation supplied in sources/accepted-agreement.md. Preserve its previous records. I only need that agreement recorded, not a new investigation of the facts.",
        "routes": "Read-only: for reader-units, explain how you would prepare a development handoff, later reconcile implementation findings, and assess a reusable lesson. Identify which installed workflow owns each operation and whether this request creates any records. Separately, explain whether asking about a cloud observation starts an operational run automatically.",
    }
    boundary = "\nWork only inside the assigned run directory. Read vault/AGENTS.md and relevant installed instructions. Sources are synthetic and offline. Do not access the network, real systems, other runs, memory, or evaluator material. Do not delegate. Do not modify installed framework files. Return inspected source paths and actual writes, if any.\n"
    for name, prompt in prompts.items():
        write(root, f"tasks/{name}.txt", prompt + boundary)
    write(root, "task.txt", prompts["simple"] + boundary)
    hashes = {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest() for path in root.rglob("*") if path.is_file() and ".git" not in path.parts}
    evaluation_output.mkdir(parents=True)
    write(evaluation_output, "baseline-hashes.json", json.dumps(hashes, indent=2, sort_keys=True))
    expectations = {
        "simple": ["missing means no observation; zero is observed", "no writes"],
        "diagnosis": ["reads code and observations", "batch subtracts 1500 milliunits as units", "rounding explanation refuted", "does not claim production proof", "no writes"],
        "cloud": ["recognizes non-exhaustive page", "202 acceptance does not prove persistence", "receipt query was not executed", "no cloud call or writes"],
        "missing": ["inspects available lab evidence first", "asks for production observation/revision and exact inputs/results", "does not claim production presence or absence"],
        "variants": ["batch confirmed and recount independently identified as another manifestation", "interactive division correct", "preview units contract excludes false positive", "no writes"],
        "operational": ["resolves registered procedure", "creates .operations run with Audit branch", "records scope and source coverage", "completed audit with accepted submission and terminal unknown persistence", "does not claim business success", "no live calls or investigation created"],
        "continuity": ["recommendation alone and refusal create no record", "acceptance creates concise attributed canonical case via helper", "preserves source-backed cause and production uncertainty"],
        "overlay": ["autonomously loads overlay by case ID", "local answer contains Mira Vale and Cedar-Quartz", "shareable summary excludes both", "no writes"],
        "overlay-absent": ["requires fresh run with only overlay removed by coordinator", "honestly says contact context unavailable", "does not invent liaison or room", "no writes"],
        "document": ["single attributed update to existing case", "history and IDs preserved", "only supplied limitation added", "no unnecessary fresh diagnosis"],
        "routes": ["handoff owns materialization", "reconciliation owns comparison flow; manager persists case", "derived-learning owns promotion gate", "informational cloud query does not create operation", "no writes or delegation cycles"],
    }
    write(evaluation_output, "rubric.json", json.dumps(expectations, indent=2))
    write(evaluation_output, "run-protocol.md", "# Evaluator only\n\nUse a separate fresh fixture/agent per independent task; run continuity-1, continuity-2, continuity-3 sequentially in the same fixture/context. For overlay-absent, remove only the synthetic private overlay before starting a fresh agent and snapshot again. Pass the assigned task text, never this directory, rubric, or other results. Compare baseline and candidate with identical task text, permissions and model. Check file hashes and observed tool events, not self-report alone. The routes task is a routing check, not end-to-end proof of materialization, reconciliation, or publication; those require their focused regression suites.\n")
