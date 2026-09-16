"""Run paired, isolated Norte routing experiments with complete phase evidence."""
from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
import re
import signal
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

DIST = Path(__file__).resolve().parents[2]
MODEL = "gpt-5.6-sol"
EFFORT = "medium"
USAGE_KEYS = ("input_tokens", "cached_input_tokens", "output_tokens")


def save(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def fingerprint(root):
    return {p.relative_to(root).as_posix(): ("symlink:" + os.readlink(p) if p.is_symlink()
            else hashlib.sha256(p.read_bytes()).hexdigest())
            for p in sorted(root.rglob("*")) if p.is_file() or p.is_symlink()}


def aggregate_usage(items):
    return {key: sum(item[key] for item in items) if items and all(
        item.get(key) is not None for item in items) else None for key in USAGE_KEYS}


def event_usage(path):
    turns = []
    for line in path.read_text(encoding="utf-8").splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("type") == "turn.completed":
            turns.append(event.get("usage") or {})
    return aggregate_usage(turns)


def phase(root, vault, evidence, prompt):
    evidence.mkdir(parents=True)
    (evidence / "prompt.txt").write_text(prompt, encoding="utf-8")
    command = ["codex", "exec", "--skip-git-repo-check", "--ignore-user-config", "--ephemeral",
               "-m", MODEL, "-c", f'model_reasoning_effort="{EFFORT}"',
               "-c", 'approval_policy="never"', "-s", "workspace-write", "--json",
               "--output-last-message", str(evidence / "answer.md"), "-"]
    env = dict(os.environ, PATH=str(root / "bin") + os.pathsep + os.environ.get("PATH", ""),
               PYTHONDONTWRITEBYTECODE="1")
    started = time.monotonic()
    error = None
    with (evidence / "events.jsonl").open("w", encoding="utf-8") as out, (
            evidence / "stderr.log").open("w", encoding="utf-8") as err:
        try:
            process = subprocess.Popen(command, text=True, cwd=vault, env=env,
                                       stdin=subprocess.PIPE, stdout=out, stderr=err,
                                       start_new_session=True)
            process.communicate(input=prompt, timeout=240)
            code = process.returncode
        except subprocess.TimeoutExpired:
            # Only this phase's process group, never unrelated local agents.
            try:
                os.killpg(process.pid, signal.SIGTERM)
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            except ProcessLookupError:
                process.wait()
            # The CLI may have exited before its descendants; clear the group.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            code, error = None, "timeout after 240 seconds"
        except OSError as exc:
            code, error = None, str(exc)
    summary = {"phase": evidence.name, "exit": code, "error": error,
               "wall_seconds": time.monotonic() - started,
               "usage": event_usage(evidence / "events.jsonl"),
               "requested_model": MODEL, "requested_effort": EFFORT,
               "effective_model": None, "effective_effort": None,
               "effective_configuration_verified": False, "command": command}
    save(evidence / "summary.json", summary)
    return summary


def instructions(root):
    return ("Trabajas exclusivamente en este vault Norte de laboratorio y eres el trabajador "
            "asignado a tu propia tarea. Sigue el enrutamiento y las instrucciones actuales del vault. "
            "No delegues recursivamente ni crees agentes o trabajadores. No leas vaults reales ni "
            "busques fuera de este fixture. No uses red, conectores ni aplicaciones reales. "
            "No cambies contenido, configuracion ni Git. Se permiten solo los bloqueos temporales "
            "que los comandos oficiales de lectura crean y eliminan; no omitas sus validaciones. "
            "Obsidian esta desconectado; para todos "
            f'los comandos usa PATH="{root / "bin"}:$PATH". '
            "Responde en español, de forma concisa, citando evidencia concreta. Conserva la "
            "calidad y las revisiones exigidas; expresa limites o incertidumbre sin inventar evidencia. "
            "El runner proporciona una revision independiente del borrador antes de entregar la "
            "respuesta; no ejecutes trabajadores anidados para esa revision.\n\n")


def answer_text(evidence, name):
    path = evidence / name / "answer.md"
    return path.read_text(encoding="utf-8") if path.exists() else "Sin respuesta."


def review_result(evidence, name):
    raw = answer_text(evidence, name).strip()
    if raw.startswith("```json") and raw.endswith("```"):
        raw = raw[7:-3].strip()
    elif raw.startswith("```") and raw.endswith("```"):
        raw = raw[3:-3].strip()
    try:
        result = json.loads(raw)
    except json.JSONDecodeError:
        return {"accepted": False, "issues": ["Review did not return valid JSON"], "parse_error": True}
    if not isinstance(result, dict) or type(result.get("accepted")) is not bool or not isinstance(result.get("issues"), list):
        return {"accepted": False, "issues": ["Review JSON violates expected schema"], "parse_error": True}
    return result


def run_pair_member(base, materializer, case, repetition, variant, workers):
    root = base / "workers" / case["id"] / f"r{repetition:02d}-{variant}"
    evidence = base / "evidence" / case["id"] / f"r{repetition:02d}-{variant}"
    evidence.mkdir(parents=True)
    started = time.monotonic()
    vault = materializer.materialize(root / "vault")
    (root / "bin").mkdir()
    stub = root / "bin" / "obsidian"
    stub.write_text("#!/bin/sh\nprintf 'Obsidian disconnected in synthetic fixture\\n' >&2\nexit 1\n")
    stub.chmod(0o755)
    (root / "bin" / "python3").symlink_to(sys.executable)
    before = fingerprint(root)
    save(evidence / "before.json", before)
    setup_seconds = time.monotonic() - started
    started = time.monotonic()
    phases = []
    prefix = instructions(root)
    prompt = prefix + case["prompt"]
    if variant == "alternative":
        prompt += "\n\nEstrategia de este experimento:\n" + case["alternative"]
        children = case.get("children", [])
        if children:
            child_started = time.monotonic()
            def run_child(index_task):
                index, task = index_task
                return phase(root, vault, evidence / f"child-{index:02d}", prefix +
                             "Descomposicion fija definida por el experimento, no autonoma. "
                             "Resuelve exclusivamente esta parte de la tarea:\n" + task)
            with ThreadPoolExecutor(max_workers=workers) as pool:
                phases.extend(pool.map(run_child, enumerate(children, 1)))
            child_wall = time.monotonic() - child_started
            prompt += ("\n\nEsta es una descomposicion fija del experimento, no una decision "
                       "autonoma del agente. Sintetiza las salidas siguientes, comprueba su "
                       "evidencia y declara cualquier fallo o limite. No lances mas trabajadores.\n")
            for child in phases:
                answer_path = evidence / child["phase"] / "answer.md"
                answer = answer_path.read_text(encoding="utf-8") if answer_path.exists() else "Sin respuesta."
                prompt += "\n" + json.dumps({"worker": child["phase"], "exit": child["exit"],
                                               "error": child["error"]}, ensure_ascii=False) + "\n" + answer + "\n"
        else:
            child_wall = 0.0
    else:
        child_wall = 0.0
    phases.append(phase(root, vault, evidence / "parent", prompt))
    draft = answer_text(evidence, "parent")
    reviewer_prompt = (prefix + "Eres el revisor independiente de esta respuesta. Sigue la rubrica "
                       "response-quality del vault. Lee la evidencia decisiva del fixture y revisa "
                       "correccion, cobertura de la pregunta, citas, limites y afirmaciones operativas. "
                       "Tu rol es exclusivamente revisar el borrador de otro trabajador; no eres "
                       "su autor y no sometas tu dictamen a otro ciclo autor-revisor. No delegues. "
                       "Devuelve exclusivamente JSON {\"accepted\":bool,\"issues\":[]}.\n\n"
                       "Pregunta original:\n" + case["prompt"] + "\n\nBorrador:\n")
    phases.append(phase(root, vault, evidence / "review", reviewer_prompt + draft))
    review = review_result(evidence, "review")
    if not review["accepted"]:
        repair_prompt = (prefix + "Corrige una vez el borrador usando la revision independiente y "
                         "la evidencia del fixture; devuelve la respuesta completa corregida.\n\n"
                         "Pregunta original:\n" + case["prompt"] + "\n\nBorrador:\n" + draft +
                         "\n\nRevision:\n" + json.dumps(review, ensure_ascii=False))
        phases.append(phase(root, vault, evidence / "repair", repair_prompt))
        draft = answer_text(evidence, "repair")
        phases.append(phase(root, vault, evidence / "recheck", reviewer_prompt + draft))
        review = review_result(evidence, "recheck")
    (evidence / "answer.md").write_text(draft, encoding="utf-8")
    save(evidence / "quality-review.json", review)
    task_seconds = time.monotonic() - started
    after = fingerprint(root)
    save(evidence / "after.json", after)
    changed = sorted(p for p in before.keys() | after.keys() if before.get(p) != after.get(p))
    summary = {"case": case["id"], "repetition": repetition, "variant": variant,
               "wall_seconds": task_seconds, "setup_seconds": setup_seconds,
               "children_wall_seconds": child_wall,
               "phase_wall_seconds_sum": sum(p["wall_seconds"] for p in phases),
               "usage": aggregate_usage([p["usage"] for p in phases]), "phases": phases,
               "quality_review": review, "changed": changed, "ok": review["accepted"] and not changed and all(
                   p["exit"] == 0 and p["error"] is None for p in phases)}
    save(evidence / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False), flush=True)
    return summary


def load_cases(path, only):
    cases = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(cases, list) or not cases:
        raise ValueError("cases must be a nonempty JSON list")
    ids = set()
    for case in cases:
        if not isinstance(case, dict) or not isinstance(case.get("id"), str) or not re.fullmatch(
                r"[A-Za-z0-9][A-Za-z0-9_-]*", case["id"]):
            raise ValueError("case id must be a safe unique path component")
        if case["id"] in ids:
            raise ValueError("duplicate case id: " + case["id"])
        ids.add(case["id"])
        if any(not isinstance(case.get(key), str) or not case[key].strip() for key in ("prompt", "alternative")):
            raise ValueError("each case needs nonempty prompt and alternative strings")
        if "children" in case and (not isinstance(case["children"], list) or any(
                not isinstance(child, str) or not child.strip() for child in case["children"])):
            raise ValueError("children must be a list of nonempty task strings")
    if only:
        unknown = set(only) - ids
        if unknown:
            raise ValueError("unknown case IDs: " + ", ".join(sorted(unknown)))
        cases = [case for case in cases if case["id"] in only]
    return cases


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--distribution", type=Path, required=True)
    parser.add_argument("--repetitions", type=int, default=3)
    parser.add_argument("--workers", type=int, default=2)
    parser.add_argument("--concurrency", type=int, default=3,
                        help="independent case pairs in parallel; each pair remains sequential")
    parser.add_argument("--only", nargs="+", action="extend")
    args = parser.parse_args()
    if args.repetitions < 1 or args.workers < 1 or args.concurrency < 1:
        parser.error("repetitions, workers and concurrency must be positive")
    distribution = args.distribution.expanduser().resolve()
    if (distribution / "VERSION").read_text().strip() != "0.10.2":
        parser.error("--distribution must be the frozen 0.10.2 distribution")
    cases = load_cases(args.cases, args.only)
    output = args.output.expanduser().resolve()
    if output.exists() and any(output.iterdir()):
        parser.error("--output must be absent or empty to preserve prior evidence")
    spec = importlib.util.spec_from_file_location("norte_materialize", DIST / "evals/graph-economy/materialize.py")
    materializer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(materializer)
    materializer.DIST = distribution
    materializer.INSTALL = distribution / "install.sh"
    output.mkdir(parents=True, exist_ok=True)
    head = subprocess.run(["git", "-C", str(distribution), "rev-parse", "HEAD"],
                          text=True, capture_output=True, check=False)
    status = subprocess.run(["git", "-C", str(distribution), "status", "--porcelain"],
                            text=True, capture_output=True, check=False)
    if head.returncode or head.stdout.strip() != "ca5a6985d677c2b3e2a699edac67285b05f81fb4" or status.returncode or status.stdout.strip():
        parser.error("distribution must be clean at the registered 0.10.2 commit ca5a698")
    fixture_hashes = fingerprint(DIST / "evals/graph-economy/fixtures/norte")
    fixture_hashes["materialize.py"] = hashlib.sha256((DIST / "evals/graph-economy/materialize.py").read_bytes()).hexdigest()
    source_hashes = {name: hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None
                     for name, path in {"cases": args.cases.resolve(), "runner": Path(__file__).resolve(),
                                        "criteria": Path(__file__).with_name("criteria.md")}.items()}
    save(output / "campaign.json", {"distribution": str(distribution), "version": "0.10.2",
         "model": MODEL, "effort": EFFORT, "repetitions": args.repetitions,
         "workers": args.workers, "concurrency": args.concurrency,
         "source_sha256": source_hashes, "fixture_sha256": fixture_hashes, "distribution_head": head.stdout.strip() if head.returncode == 0 else None,
         "cases": cases, "decomposition": "fixed experiment tasks"})
    started = time.monotonic()
    results = []
    for repetition in range(1, args.repetitions + 1):
        order = ("baseline", "alternative") if repetition % 2 else ("alternative", "baseline")
        def run_pair(case, repetition=repetition, order=order):
            return [run_pair_member(output, materializer, case, repetition, variant, args.workers)
                    for variant in order]
        with ThreadPoolExecutor(max_workers=args.concurrency) as pool:
            for pair in pool.map(run_pair, cases):
                results.extend(pair)
                save(output / "results.json", results)
    save(output / "summary.json", {"runs": results, "elapsed_seconds": time.monotonic() - started,
         "wall_seconds": sum(r["wall_seconds"] for r in results),
         "setup_seconds_sum": sum(r["setup_seconds"] for r in results),
         "phase_wall_seconds_sum": sum(r["phase_wall_seconds_sum"] for r in results),
         "usage": aggregate_usage([r["usage"] for r in results]),
         "by_variant": {variant: {"runs": len(group), "wall_seconds_sum": sum(r["wall_seconds"] for r in group),
                         "usage": aggregate_usage([r["usage"] for r in group])}
                        for variant in ("baseline", "alternative")
                        for group in [[r for r in results if r["variant"] == variant]]}})
    return int(any(not r["ok"] for r in results))


if __name__ == "__main__":
    raise SystemExit(main())
