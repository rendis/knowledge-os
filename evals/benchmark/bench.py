"""Homologated benchmark of harness settings on real cell vaults: questions and publication flows.

python3 -B evals/benchmark/bench.py prepare --suite SUITE.json --work DIR
python3 -B evals/benchmark/bench.py qa      --suite SUITE.json --work DIR --setting claude:sonnet:low [--runs 3 | --quick] [--ids S1,S2]
python3 -B evals/benchmark/bench.py flow    --suite SUITE.json --work DIR --setting claude:sonnet:low [--runs 1]
python3 -B evals/benchmark/bench.py report  --work DIR

A setting is harness:model:effort (Cursor carries effort in its model id: cursor:grok-4.7-medium:).
Every setting answers the same frozen fixture with the same prompts, fresh sessions and the same
fixed judge/reviewer, which never learn who wrote the output. See README.md for the protocol.
"""
import argparse, concurrent.futures as cf, datetime, hashlib, json, os, pathlib, platform, re, shutil, subprocess, sys

HERE = pathlib.Path(__file__).resolve().parent
DIST = HERE.parent.parent
sys.path.insert(0, str(HERE.parent / "regression"))
import run as runner  # noqa: E402
import judge  # noqa: E402

REVIEWER = judge.JUDGE  # the flow reviewer is the same fixed grader family and setting as the QA judge
IDENTITY = {"GIT_AUTHOR_NAME": "benchmark", "GIT_AUTHOR_EMAIL": "benchmark@invalid", "GIT_COMMITTER_NAME": "benchmark", "GIT_COMMITTER_EMAIL": "benchmark@invalid"}
REVIEW_PROMPT = """Eres un revisor independiente de evidencia; no escribiste este cambio. En este vault (directorio actual) la rama `{branch}` propone cambios sobre `{base}`. Revisa `git diff {base}...{branch}` en modo de solo lectura: no modifiques archivos, no hagas commit, fetch ni push.
Sigue `.agents/skills/map-ecosystem/references/final-note-review.md`. Contrasta cada afirmación cambiada con las fuentes en el commit que cita (resuelve los checkouts con `{cli} config resolve --vault .` y lee con `git -C <repo> show <sha>:<ruta>`) y con los snapshots de `90-Meta/discovery/platform/`. Las anclas que pasaron G1 existen; juzga su interpretación. Verifica que no se perdió conocimiento válido de la nota base y que lo desconocido queda como límite.
Resultado de los gates: {gates}
Responde SOLO un objeto JSON: {{"verdict": "accept|revise", "findings": [{{"file": "...", "claim": "frase exacta", "evidence": "archivo:línea en commit que la contradice", "severity": "material|minor"}}]}}"""


def sh(args, cwd=None, env=None, check=True):
    r = subprocess.run(args, cwd=cwd, capture_output=True, text=True, env={**os.environ, **(env or {})})
    if check and r.returncode != 0:
        raise SystemExit(f"{' '.join(map(str, args))}: {r.stderr.strip()}")
    return r.stdout.strip()


def cli():
    """The kos of the current release (make release): its kernel is the one under test."""
    arch = {"arm64": "arm64", "aarch64": "arm64"}.get(platform.machine(), "amd64")
    osname = {"Darwin": "darwin", "Linux": "linux", "Windows": "windows"}[platform.system()]
    return str(DIST / "dist" / f"kos-{osname}-{arch}{'.exe' if osname == 'windows' else ''}")


def sha256(path):
    return hashlib.sha256(pathlib.Path(path).read_bytes()).hexdigest()[:16]


def load_suite(path):
    s = json.loads(pathlib.Path(path).read_text())
    s["_dir"] = pathlib.Path(path).resolve().parent
    return s


def parse_setting(text):
    harness, model, effort = (text.split(":") + ["", ""])[:3]
    if harness not in runner.DEFAULTS:
        raise SystemExit(f"unknown harness in setting {text!r}")
    return harness, model or runner.DEFAULTS[harness][0], effort or runner.DEFAULTS[harness][1]


def tag(setting):
    return re.sub(r"[^A-Za-z0-9.-]+", "_", "-".join(x for x in setting if x))


def source_state(vault):
    """HEAD and working-tree status of every source checkout the vault resolves; must not change."""
    cfg = pathlib.Path(vault) / ".knowledge-os-config.yaml"
    roots = re.findall(r'^\s*-\s*"?([^"\n]+?)"?\s*$', cfg.read_text(), re.M) if cfg.exists() else []
    state = {}
    for root in {r for r in roots if os.path.isdir(r)}:
        for d in sorted(pathlib.Path(root).iterdir()):
            if (d / ".git").exists():
                head = sh(["git", "-C", str(d), "rev-parse", "HEAD"], check=False)
                status = sh(["git", "-C", str(d), "status", "--porcelain"], check=False)
                state[d.name] = head + ":" + hashlib.sha256(status.encode()).hexdigest()[:12]
    return state


def prepare(a):
    suite, work = load_suite(a.suite), pathlib.Path(a.work)
    fp = {"prepared_at": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
          "distribution": sh(["git", "-C", str(DIST), "rev-parse", "--short=12", "HEAD"]) + ("-dirty" if sh(["git", "-C", str(DIST), "status", "--porcelain", "--", "internal", "kernel", "cmd", "scripts"]) else ""),
          "kernel": (DIST / "VERSION").read_text().strip(), "suite": sha256(a.suite),
          "questions": sha256(suite["_dir"] / suite["questions"]), "judge": judge.JUDGE, "reviewer": REVIEWER, "vaults": {}}
    for key, v in suite["vaults"].items():
        dst = work / "vaults" / key
        if dst.exists():
            shutil.rmtree(dst)
        sh(["git", "clone", "-q", "--no-local", v["source"], str(dst)])
        sh(["git", "-C", str(dst), "checkout", "-q", "-B", v["branch"], v["commit"]])
        # Keep the vault's declared identity (its real origin) for fetches; pushes are disabled.
        sh(["git", "-C", str(dst), "remote", "set-url", "origin", sh(["git", "-C", v["source"], "remote", "get-url", "origin"])])
        sh(["git", "-C", str(dst), "remote", "set-url", "--push", "origin", "DISABLED-benchmark"])
        for k, val in IDENTITY.items():
            if k.startswith("GIT_AUTHOR"):
                sh(["git", "-C", str(dst), "config", "user." + ("name" if k.endswith("NAME") else "email"), val])
        if v.get("overlay"):
            shutil.copytree(suite["_dir"] / v["overlay"], dst, dirs_exist_ok=True)
        shutil.copy(suite["_dir"] / v["config"], dst / ".knowledge-os-config.yaml")
        sh([cli(), "kernel", "update", "--vault", str(dst)])
        sh(["git", "-C", str(dst), "add", "-A"])
        sh(["git", "-C", str(dst), "commit", "-q", "--allow-empty", "-m", "chore(benchmark): fixture"], env=IDENTITY)
        sh([cli(), "discover", "run", "--vault", str(dst), "--classify", "off"])
        facts = sorted((dst / ".agents/state/discovery/facts").glob("*.json"))
        fp["vaults"][key] = {"source_commit": v["commit"], "fixture": sh(["git", "-C", str(dst), "rev-parse", "--short=12", "HEAD"]),
                             "facts": hashlib.sha256(b"".join(sha256(f).encode() for f in facts)).hexdigest()[:16]}
    fp["harnesses"] = {h: sh([h, "--version"], check=False).splitlines()[0] if shutil.which(h) else "absent" for h in ("claude", "codex", "cursor-agent")}
    (work / "fingerprint.json").write_text(json.dumps(fp, indent=1))
    print(json.dumps(fp, indent=1))


def qa(a):
    suite, work = load_suite(a.suite), pathlib.Path(a.work)
    setting = parse_setting(a.setting)
    qs = json.loads((suite["_dir"] / suite["questions"]).read_text())
    if a.quick:  # the suite's discriminating subset, one pass: screening a new model against the baseline
        qs = [q for q in qs if q["id"] in set(suite.get("quick", []))]
        if not qs:
            raise SystemExit("the suite declares no quick question ids")
    if a.ids:  # a named subset, e.g. a question added to measure one behavior
        wanted = set(a.ids.split(","))
        qs = [q for q in qs if q["id"] in wanted]
        if not qs:
            raise SystemExit("no question matches --ids")
    runs = a.runs or (1 if a.quick else 3)
    vaults = {k: str(work / "vaults" / k) for k in suite["vaults"]}
    for i in range(1, runs + 1):
        out = work / "qa" / (tag(setting) + ("-quick" if a.quick else "") + ("-" + a.ids.replace(",", "-") if a.ids else "")) / f"run{i}"
        with cf.ThreadPoolExecutor(a.parallel) as ex:
            list(ex.map(lambda q: runner.run_one(setting[0], q, vaults[q["vault"]], out, setting[1], setting[2]), [q for q in qs if q["vault"] in vaults]))
        subprocess.run([sys.executable, "-B", str(HERE.parent / "regression" / "judge.py"), "--questions", str(suite["_dir"] / suite["questions"]), "--answers", str(out)], check=True)


REPAIR_PROMPT = """En este vault, la rama `{branch}` recibió una revisión independiente con veredicto revise y estos hallazgos:
{findings}
Estado de los gates: {gates}
Corrige solo las afirmaciones señaladas y los gates que fallan, contra la evidencia y sin reabrir el análisis. Commitea en la misma rama, corre `discover check` en las notas de repositorio tocadas y `sync verify`. No registres review, no ejecutes `sync finish`, no hagas push. Los repositorios fuente son de solo lectura (git fetch permitido, nada más). Termina con los commits y el resultado de los gates."""


def evaluate(dst, base, out_prefix, timeout):
    """Deterministic gates plus the fixed reviewer's verdict for the branch currently checked out."""
    ev = {"branch": sh(["git", "-C", str(dst), "branch", "--show-current"], check=False)}
    on_branch = ev["branch"].startswith("sync/")
    ev["changed"] = sh(["git", "-c", "core.quotePath=false", "-C", str(dst), "diff", "--name-only", f"{base}...HEAD"], check=False).splitlines() if on_branch else []
    ev["uncommitted"] = bool(sh(["git", "-C", str(dst), "status", "--porcelain", "--", ".", ":!.agents/state"], check=False))
    ev["note_gates"], ev["stale_neighbours"], ev["structural_issues_introduced"] = [], [], []
    if on_branch:
        v = subprocess.run([cli(), "sync", "verify", "--vault", str(dst)], capture_output=True, text=True).stdout
        try:
            vj = json.loads(v[v.index("{", 1):] if v.startswith('{"error') else v)
            ev["note_gates"], ev["stale_neighbours"] = vj.get("note_gates", []), vj.get("stale_neighbours", [])
            ev["structural_issues_introduced"] = vj.get("new_structural_issues", [])
        except ValueError:
            ev["structural_issues_introduced"] = ["unparsed sync verify output"]
    ev["gates_ok"] = bool(ev["changed"]) and not ev["uncommitted"] and all(g.get("ok") for g in ev["note_gates"]) and not ev["stale_neighbours"] and not ev["structural_issues_introduced"]
    ev["review"] = {"verdict": "none", "findings": []}
    if ev["changed"]:
        gates = {"note_gates": ev["note_gates"], "stale_neighbours": ev["stale_neighbours"], "structural_issues_introduced": ev["structural_issues_introduced"]}
        prompt = REVIEW_PROMPT.format(branch=ev["branch"], base=base, cli=cli(), gates=json.dumps(gates, ensure_ascii=False))
        rv = runner.execute(REVIEWER["harness"], prompt, dst, REVIEWER["model"], REVIEWER["effort"], f"{out_prefix}.review.txt", timeout=timeout)
        m = re.search(r"\{.*\}", rv.get("answer", ""), re.S)
        try:
            verdict = json.loads(m.group(0)) if m else {}
        except ValueError:
            verdict = {}
        ev["review"] = {"verdict": verdict.get("verdict", "error"), "findings": verdict.get("findings", []), "seconds": rv["seconds"], "usage": rv.get("usage")}
    ev["material"] = len([x for x in ev["review"]["findings"] if x.get("severity") == "material"])
    ev["accepted"] = ev["gates_ok"] and ev["review"]["verdict"] == "accept" and ev["material"] == 0
    return ev


def flow(a):
    suite, work = load_suite(a.suite), pathlib.Path(a.work)
    setting = parse_setting(a.setting)
    for f in suite["flows"]:
        if a.flow and f["id"] != a.flow:
            continue
        for i in range(1, a.runs + 1):
            dst = work / "flows" / tag(setting) / f["id"] / f"run{i}"
            if dst.exists():
                shutil.rmtree(dst)
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copytree(work / "vaults" / f["vault"], dst, symlinks=True)
            base = suite["vaults"][f["vault"]]["branch"]
            before = source_state(dst)
            prefix = dst.parent / f"run{i}"
            rec = {"flow": f["id"], "run": i, "setting": setting, "rounds": []}
            author = runner.execute(setting[0], f["prompt"], dst, setting[1], setting[2], f"{prefix}.author0.txt", write=True, timeout=a.timeout)
            ev = evaluate(dst, base, f"{prefix}.r0", a.timeout)
            rec["rounds"].append({"author": author, **ev})
            # Protocol: a revise verdict or failing gate gets a focused repair and a new review, bounded.
            for n in range(1, a.repairs + 1):
                if ev["accepted"] or not ev["changed"]:
                    break
                prompt = REPAIR_PROMPT.format(branch=ev["branch"], findings=json.dumps(ev["review"]["findings"], ensure_ascii=False, indent=1),
                                              gates=json.dumps({"gates_ok": ev["gates_ok"], "note_gates": ev["note_gates"], "stale_neighbours": ev["stale_neighbours"]}, ensure_ascii=False))
                author = runner.execute(setting[0], prompt, dst, setting[1], setting[2], f"{prefix}.author{n}.txt", write=True, timeout=a.timeout)
                ev = evaluate(dst, base, f"{prefix}.r{n}", a.timeout)
                rec["rounds"].append({"author": author, **ev})
            first, last = rec["rounds"][0], rec["rounds"][-1]
            after = source_state(dst)
            rec.update({"changed": last["changed"], "first_pass_accepted": first["accepted"], "accepted": last["accepted"], "repairs": len(rec["rounds"]) - 1,
                        "first_pass_material": first["material"], "final_material": last["material"], "gates_ok": last["gates_ok"],
                        "stale_neighbours": last["stale_neighbours"], "sources_unchanged": before == after,
                        "sources_changed": sorted(k for k in set(before) | set(after) if before.get(k) != after.get(k)),
                        "author_seconds": sum(r["author"]["seconds"] for r in rec["rounds"]),
                        "author_cost_usd": sum(r["author"].get("cost_usd") or 0 for r in rec["rounds"]) or None,
                        "author_input_tokens": sum((r["author"].get("usage") or {}).get("input_total", 0) for r in rec["rounds"]),
                        "author_output_tokens": sum((r["author"].get("usage") or {}).get("output", 0) for r in rec["rounds"])})
            (dst.parent / f"run{i}.json").write_text(json.dumps(rec, ensure_ascii=False, indent=1))
            print(f["id"], tag(setting), f"run{i}", "first_pass_accepted=%s" % rec["first_pass_accepted"], "accepted=%s" % rec["accepted"], "repairs=%d" % rec["repairs"],
                  "material=%d->%d" % (rec["first_pass_material"], rec["final_material"]), "sources_unchanged=%s" % rec["sources_unchanged"], flush=True)


def mean(xs):
    xs = [x for x in xs if x is not None]
    return sum(xs) / len(xs) if xs else None


def report(a):
    work = pathlib.Path(a.work)
    fp = json.loads((work / "fingerprint.json").read_text()) if (work / "fingerprint.json").exists() else {}
    rows, frows = [], []
    for d in sorted((work / "qa").glob("*")) if (work / "qa").exists() else []:
        runs = []
        for r in sorted(d.glob("run*")):
            g = json.loads((r / "grades.json").read_text()) if (r / "grades.json").exists() else None
            recs = [json.loads(f.read_text()) for f in r.glob("*.json") if f.name != "grades.json"]
            if not g or not recs:
                continue
            runs.append({"score": mean([x["score"] for x in g.values()]), "violations": sum(len(x.get("violations", [])) for x in g.values()),
                         "seconds": sum(x["seconds"] for x in recs), "input": sum((x.get("usage") or {}).get("input_total", 0) for x in recs),
                         "output": sum((x.get("usage") or {}).get("output", 0) for x in recs), "cost": sum(x.get("cost_usd") or 0 for x in recs) or None, "n": len(recs)})
        if runs:
            rows.append({"setting": d.name, "runs": len(runs), "score_mean": mean([r["score"] for r in runs]), "score_min": min(r["score"] for r in runs),
                         "score_max": max(r["score"] for r in runs), "violations_per_run": mean([r["violations"] for r in runs]), "answers_per_run": runs[0]["n"],
                         "seconds": mean([r["seconds"] for r in runs]), "input_tokens": mean([r["input"] for r in runs]), "output_tokens": mean([r["output"] for r in runs]), "cost_usd": mean([r["cost"] for r in runs])})
    for d in sorted((work / "flows").glob("*")) if (work / "flows").exists() else []:
        recs = [json.loads(f.read_text()) for f in d.glob("*/run*.json") if not f.name.endswith((".review.txt", ".txt"))]
        recs = [r for r in recs if "rounds" in r]
        if not recs:
            continue
        frows.append({"setting": d.name, "runs": len(recs), "first_pass_accepted": sum(r["first_pass_accepted"] for r in recs), "accepted": sum(r["accepted"] for r in recs),
                      "repairs": mean([r["repairs"] for r in recs]), "first_pass_material": mean([r["first_pass_material"] for r in recs]),
                      "stale_neighbours_first_pass": mean([len(r["rounds"][0]["stale_neighbours"]) for r in recs]),
                      "sources_unchanged": all(r["sources_unchanged"] for r in recs), "seconds": mean([r["author_seconds"] for r in recs]),
                      "input_tokens": mean([r["author_input_tokens"] for r in recs]), "output_tokens": mean([r["author_output_tokens"] for r in recs]), "cost_usd": mean([r["author_cost_usd"] for r in recs])})
    out = {"fingerprint": fp, "qa": rows, "flows": frows}
    (work / "report.json").write_text(json.dumps(out, indent=1))
    f = lambda x, p=2: "—" if x is None else (f"{x:.{p}f}" if isinstance(x, float) else str(x))
    md = [f"# Benchmark report", "", f"Distribution `{fp.get('distribution')}` (kernel {fp.get('kernel')}), suite `{fp.get('suite')}`, questions `{fp.get('questions')}`, judge/reviewer `{fp.get('judge')}`, prepared {fp.get('prepared_at')}.", ""]
    if rows:
        md += ["## Questions", "", "| Setting | Runs | Score mean (min–max) | Violations / run | Time / run (s) | Input tokens / run | Output tokens / run | USD / run |", "|---|---|---|---|---|---|---|---|"]
        md += [f"| {r['setting']} | {r['runs']} | {f(r['score_mean'], 3)} ({f(r['score_min'], 3)}–{f(r['score_max'], 3)}) | {f(r['violations_per_run'], 1)} of {r['answers_per_run']} | {f(r['seconds'], 0)} | {f(r['input_tokens'], 0)} | {f(r['output_tokens'], 0)} | {f(r['cost_usd'])} |" for r in rows]
    if frows:
        md += ["", "## Publication flows (author until accepted, bounded repairs)", "", "| Setting | Runs | Accepted first pass | Accepted after repairs | Repairs / run | Material findings first pass | Stale neighbours first pass | Sources unchanged | Author time (s) | Author input tokens | Author USD |", "|---|---|---|---|---|---|---|---|---|---|---|"]
        md += [f"| {r['setting']} | {r['runs']} | {r['first_pass_accepted']} | {r['accepted']} | {f(r['repairs'], 1)} | {f(r['first_pass_material'], 1)} | {f(r['stale_neighbours_first_pass'], 1)} | {r['sources_unchanged']} | {f(r['seconds'], 0)} | {f(r['input_tokens'], 0)} | {f(r['cost_usd'])} |" for r in frows]
    (work / "report.md").write_text("\n".join(md) + "\n")
    print("\n".join(md))


def main():
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)
    for name in ("prepare", "qa", "flow"):
        s = sub.add_parser(name)
        s.add_argument("--suite", required=True, help="cell-owned suite.json (kept outside this repository)")
        s.add_argument("--work", required=True)
        if name != "prepare":
            s.add_argument("--setting", required=True)
            s.add_argument("--runs", type=int, default=0 if name == "qa" else 1)
            s.add_argument("--quick", action="store_true", help="qa only: the suite's quick subset, one run")
            s.add_argument("--ids", default="", help="qa only: comma-separated question ids to run")
            s.add_argument("--parallel", type=int, default=4)
            s.add_argument("--timeout", type=int, default=3600)
            s.add_argument("--flow", default="")
            s.add_argument("--repairs", type=int, default=2)
    sub.add_parser("report").add_argument("--work", required=True)
    a = p.parse_args()
    {"prepare": prepare, "qa": qa, "flow": flow, "report": report}[a.cmd](a)


if __name__ == "__main__":
    main()
