"""Homologated benchmark of harness settings on real cell vaults: questions and publication flows.

python3 -B evals/benchmark/bench.py prepare --suite SUITE.json --work DIR
python3 -B evals/benchmark/bench.py qa      --suite SUITE.json --work DIR --setting claude:sonnet:low [--runs 3]
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


def cli(vault):
    arch = {"arm64": "arm64", "aarch64": "arm64"}.get(platform.machine(), "amd64")
    osname = {"Darwin": "darwin", "Linux": "linux", "Windows": "windows"}[platform.system()]
    return str(pathlib.Path(vault) / ".agents" / "bin" / f"vaultctl-{osname}-{arch}{'.exe' if osname == 'windows' else ''}")


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
        sh([str(DIST / "install.sh"), "update", "--dest", str(dst)])
        sh(["git", "-C", str(dst), "add", "-A"])
        sh(["git", "-C", str(dst), "commit", "-q", "--allow-empty", "-m", "chore(benchmark): fixture"], env=IDENTITY)
        sh([cli(dst), "discover", "run", "--vault", str(dst), "--classify", "off"])
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
    vaults = {k: str(work / "vaults" / k) for k in suite["vaults"]}
    for i in range(1, a.runs + 1):
        out = work / "qa" / tag(setting) / f"run{i}"
        with cf.ThreadPoolExecutor(a.parallel) as ex:
            list(ex.map(lambda q: runner.run_one(setting[0], q, vaults[q["vault"]], out, setting[1], setting[2]), [q for q in qs if q["vault"] in vaults]))
        subprocess.run([sys.executable, "-B", str(HERE.parent / "regression" / "judge.py"), "--questions", str(suite["_dir"] / suite["questions"]), "--answers", str(out)], check=True)


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
            rec = {"flow": f["id"], "run": i, "setting": setting}
            rec["author"] = runner.execute(setting[0], f["prompt"], dst, setting[1], setting[2], dst.parent / f"run{i}.author.txt", write=True, timeout=a.timeout)
            branch = sh(["git", "-C", str(dst), "branch", "--show-current"], check=False)
            rec["branch"] = branch
            dirty = sh(["git", "-C", str(dst), "status", "--porcelain", "--", ".", ":!.agents/state"], check=False)
            changed = sh(["git", "-C", str(dst), "diff", "--name-only", f"{base}...HEAD"], check=False).splitlines() if branch.startswith("sync/") else []
            rec["changed"], rec["uncommitted"] = changed, bool(dirty)
            gates = {}
            for note in [c for c in changed if c.startswith("20-Repos/") and c.endswith(".md")]:
                r = subprocess.run([cli(dst), "discover", "check", "--vault", str(dst), "--note", note], capture_output=True, text=True)
                try:
                    j = json.loads(r.stdout)
                    j = j["notes"][0] if "notes" in j else j
                    gates[note] = {"ok": j.get("ok"), "errors": [x for x in j.get("issues", []) if x["severity"] == "error"], "anchors": j.get("anchors"), "coverage": j.get("coverage")}
                except (ValueError, KeyError, IndexError):
                    gates[note] = {"ok": False, "errors": [r.stdout[-500:] + r.stderr[-500:]]}
            if branch.startswith("sync/"):
                v = subprocess.run([cli(dst), "sync", "verify", "--vault", str(dst)], capture_output=True, text=True).stdout
                try:
                    vj = json.loads(v[v.index("{", 1) if v.startswith('{"error"') else 0:])
                    rec["structural_issues_introduced"] = vj.get("new_structural_issues", [])
                except ValueError:
                    rec["structural_issues_introduced"] = ["unparsed sync verify output"]
            rec["gates"] = gates
            rec["gates_ok"] = bool(changed) and all(g.get("ok") for g in gates.values()) and not rec.get("structural_issues_introduced")
            if changed:
                prompt = REVIEW_PROMPT.format(branch=branch, base=base, cli=cli(dst), gates=json.dumps({k: {"ok": g.get("ok"), "errors": len(g.get("errors", []))} for k, g in gates.items()}))
                rv = runner.execute(REVIEWER["harness"], prompt, dst, REVIEWER["model"], REVIEWER["effort"], dst.parent / f"run{i}.review.txt", timeout=a.timeout)
                m = re.search(r"\{.*\}", rv.get("answer", ""), re.S)
                try:
                    verdict = json.loads(m.group(0)) if m else {}
                except ValueError:
                    verdict = {}
                rec["review"] = {"verdict": verdict.get("verdict", "error"), "findings": verdict.get("findings", []), "seconds": rv["seconds"], "usage": rv.get("usage")}
            after = source_state(dst)
            rec["sources_unchanged"] = before == after
            rec["sources_changed"] = sorted(k for k in set(before) | set(after) if before.get(k) != after.get(k))
            (dst.parent / f"run{i}.json").write_text(json.dumps(rec, ensure_ascii=False, indent=1))
            material = [x for x in rec.get("review", {}).get("findings", []) if x.get("severity") == "material"]
            print(f["id"], tag(setting), f"run{i}", "branch=" + branch, "changed=%d" % len(changed), "gates_ok=%s" % rec["gates_ok"],
                  "review=" + rec.get("review", {}).get("verdict", "none"), "material=%d" % len(material), "sources_unchanged=%s" % rec["sources_unchanged"], flush=True)


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
        recs = [json.loads(f.read_text()) for f in d.glob("*/run*.json")]
        if not recs:
            continue
        au = [r["author"] for r in recs]
        frows.append({"setting": d.name, "runs": len(recs), "completed": sum(bool(r["changed"]) for r in recs), "gates_ok": sum(r["gates_ok"] for r in recs),
                      "accepted": sum(r.get("review", {}).get("verdict") == "accept" for r in recs),
                      "material_findings": mean([len([x for x in r.get("review", {}).get("findings", []) if x.get("severity") == "material"]) for r in recs]),
                      "sources_unchanged": all(r["sources_unchanged"] for r in recs), "seconds": mean([x["seconds"] for x in au]),
                      "input_tokens": mean([(x.get("usage") or {}).get("input_total") for x in au]), "output_tokens": mean([(x.get("usage") or {}).get("output") for x in au]), "cost_usd": mean([x.get("cost_usd") for x in au])})
    out = {"fingerprint": fp, "qa": rows, "flows": frows}
    (work / "report.json").write_text(json.dumps(out, indent=1))
    f = lambda x, p=2: "—" if x is None else (f"{x:.{p}f}" if isinstance(x, float) else str(x))
    md = [f"# Benchmark report", "", f"Distribution `{fp.get('distribution')}` (kernel {fp.get('kernel')}), suite `{fp.get('suite')}`, questions `{fp.get('questions')}`, judge/reviewer `{fp.get('judge')}`, prepared {fp.get('prepared_at')}.", ""]
    if rows:
        md += ["## Questions", "", "| Setting | Runs | Score mean (min–max) | Violations / run | Time / run (s) | Input tokens / run | Output tokens / run | USD / run |", "|---|---|---|---|---|---|---|---|"]
        md += [f"| {r['setting']} | {r['runs']} | {f(r['score_mean'], 3)} ({f(r['score_min'], 3)}–{f(r['score_max'], 3)}) | {f(r['violations_per_run'], 1)} of {r['answers_per_run']} | {f(r['seconds'], 0)} | {f(r['input_tokens'], 0)} | {f(r['output_tokens'], 0)} | {f(r['cost_usd'])} |" for r in rows]
    if frows:
        md += ["", "## Publication flows", "", "| Setting | Runs | Branch with changes | Gates ok | Reviewer accept | Material findings / run | Sources unchanged | Author time (s) | Author input tokens | Author USD |", "|---|---|---|---|---|---|---|---|---|---|"]
        md += [f"| {r['setting']} | {r['runs']} | {r['completed']} | {r['gates_ok']} | {r['accepted']} | {f(r['material_findings'], 1)} | {r['sources_unchanged']} | {f(r['seconds'], 0)} | {f(r['input_tokens'], 0)} | {f(r['cost_usd'])} |" for r in frows]
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
            s.add_argument("--runs", type=int, default=3 if name == "qa" else 1)
            s.add_argument("--parallel", type=int, default=4)
            s.add_argument("--timeout", type=int, default=3600)
            s.add_argument("--flow", default="")
    sub.add_parser("report").add_argument("--work", required=True)
    a = p.parse_args()
    {"prepare": prepare, "qa": qa, "flow": flow, "report": report}[a.cmd](a)


if __name__ == "__main__":
    main()
