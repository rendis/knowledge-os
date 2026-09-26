"""Run the regression questions against installed cell vaults through a real agent harness.

python3 -B evals/regression/run.py --harness claude|codex|cursor --questions FILE --out DIR \
    --vault a=PATH --vault b=PATH [--model M] [--effort low|medium|high] [--ids S1,S2] [--parallel 4]

Each question runs in a fresh headless session whose working directory is the vault, exactly as a
developer would ask it. Answers, usage and duration are stored per question for judge.py.
`execute` is shared with evals/benchmark so questions and flows are measured the same way.
"""
import argparse, concurrent.futures as cf, json, os, pathlib, subprocess, tempfile, time

HERE = pathlib.Path(__file__).parent
SUFFIX = "\n\n(Read-only question: do not modify files or run actions with effects.)"
DEFAULTS = {"claude": ("opus", "medium"), "codex": ("gpt-5.5", "medium"), "cursor": ("", "")}


def isolated_env(harness):
    """Environment with the vault's instructions only: no personal memories, global instructions, hooks or
    plugins of the person running the benchmark. Codex gets a private home that links only its auth file;
    Claude skips user settings via --setting-sources. Cursor exposes no equivalent (a recorded limit)."""
    env = dict(os.environ)
    if harness == "codex":
        real = pathlib.Path(os.environ.get("CODEX_HOME", pathlib.Path.home() / ".codex"))
        home = pathlib.Path(os.environ.get("BENCH_CODEX_HOME", pathlib.Path(tempfile.gettempdir()) / "vault-bench-codex-home"))
        home.mkdir(parents=True, exist_ok=True)
        link = home / "auth.json"
        if not link.exists() and (real / "auth.json").exists():
            link.symlink_to(real / "auth.json")
        env["CODEX_HOME"] = str(home)
    return env


def command(harness, prompt, out_file, model, effort, write=False):
    if harness == "claude":
        return ["claude", "-p", prompt, "--model", model, "--effort", effort, "--output-format", "json", "--permission-mode", "bypassPermissions", "--setting-sources", "project,local"]
    if harness == "codex":
        sandbox = "danger-full-access" if write else "read-only"  # workspace-write keeps .git read-only
        return ["codex", "exec", "--skip-git-repo-check", "-m", model, "-c", f'model_reasoning_effort="{effort}"', "-s", sandbox, "--json", "--output-last-message", str(out_file), prompt]
    if harness == "cursor":
        return ["cursor-agent", "-p", "--output-format", "json", "--force"] + (["--model", model] if model else []) + [prompt]
    raise SystemExit("unknown harness")


def normalize_usage(harness, j):
    """Tokens as total input (cached included), cached input and output, comparable across harnesses."""
    if harness == "claude":
        mu = j.get("modelUsage") or {}
        if mu:
            cached = sum(v.get("cacheReadInputTokens", 0) for v in mu.values())
            total = sum(v.get("inputTokens", 0) + v.get("cacheReadInputTokens", 0) + v.get("cacheCreationInputTokens", 0) for v in mu.values())
            return {"input_total": total, "input_cached": cached, "output": sum(v.get("outputTokens", 0) for v in mu.values())}
        u = j.get("usage") or {}
        total = u.get("input_tokens", 0) + u.get("cache_read_input_tokens", 0) + u.get("cache_creation_input_tokens", 0)
        return {"input_total": total, "input_cached": u.get("cache_read_input_tokens", 0), "output": u.get("output_tokens", 0)}
    if harness == "cursor":
        u = j.get("usage") or {}
        total = u.get("inputTokens", 0) + u.get("cacheReadTokens", 0) + u.get("cacheWriteTokens", 0)
        return {"input_total": total, "input_cached": u.get("cacheReadTokens", 0), "output": u.get("outputTokens", 0)}
    return {"input_total": j.get("input_tokens", 0), "input_cached": j.get("cached_input_tokens", 0), "output": j.get("output_tokens", 0)}


def execute(harness, prompt, cwd, model, effort, out_file, write=False, timeout=1500):
    """Run one fresh headless session and return its answer, normalized usage, cost and duration."""
    out_file = pathlib.Path(out_file)
    start = time.time()
    try:
        r = subprocess.run(command(harness, prompt, out_file, model, effort, write), cwd=cwd, stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=timeout, env=isolated_env(harness))
        stdout, rc = r.stdout, r.returncode
    except subprocess.TimeoutExpired as e:
        stdout, rc = (e.stdout or b"").decode() if isinstance(e.stdout, bytes) else (e.stdout or ""), "timeout"
    rec = {"harness": harness, "model": model, "effort": effort, "seconds": round(time.time() - start, 1), "returncode": rc}
    if harness in ("claude", "cursor"):
        try:
            j = json.loads(stdout)
            rec["answer"] = j.get("result", "")
            rec["usage"] = normalize_usage(harness, j)
            rec["cost_usd"] = j.get("total_cost_usd")
            rec["turns"] = j.get("num_turns")
        except Exception:
            rec["answer"] = stdout[-8000:]
    else:
        rec["answer"] = out_file.read_text() if out_file.exists() else stdout[-8000:]
        raw = {"input_tokens": 0, "cached_input_tokens": 0, "output_tokens": 0}
        commands = 0
        for line in stdout.splitlines():
            try:
                ev = json.loads(line)
            except ValueError:
                continue
            if ev.get("type") == "turn.completed":
                for k in raw:
                    raw[k] += ev.get("usage", {}).get(k, 0)
            if ev.get("type") == "item.completed" and ev.get("item", {}).get("type") == "command_execution":
                commands += 1
        rec["usage"], rec["commands"] = normalize_usage("codex", raw), commands
    return rec


def run_one(harness, q, vault, out_dir, model, effort):
    out_dir.mkdir(parents=True, exist_ok=True)
    rec = {"id": q["id"], "vault": vault}
    rec.update(execute(harness, q["question"] + SUFFIX, vault, model, effort, out_dir / f"{q['id']}.last.txt"))
    (out_dir / f"{q['id']}.json").write_text(json.dumps(rec, ensure_ascii=False, indent=1))
    return rec


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--harness", required=True)
    p.add_argument("--questions", required=True, help="cell-owned question set (kept outside this repository)")
    p.add_argument("--out", required=True)
    p.add_argument("--vault", action="append", required=True)
    p.add_argument("--model", default="", help="model id; Cursor effort is part of its model id")
    p.add_argument("--effort", default="")
    p.add_argument("--ids", default="")
    p.add_argument("--parallel", type=int, default=4)
    a = p.parse_args()
    vaults = dict(v.split("=", 1) for v in a.vault)
    model, effort = a.model or DEFAULTS[a.harness][0], a.effort or DEFAULTS[a.harness][1]
    qs = [q for q in json.load(open(a.questions)) if q["vault"] in vaults and (not a.ids or q["id"] in a.ids.split(","))]
    with cf.ThreadPoolExecutor(a.parallel) as ex:
        for rec in ex.map(lambda q: run_one(a.harness, q, vaults[q["vault"]], pathlib.Path(a.out), model, effort), qs):
            print(rec["id"], rec["returncode"], rec["seconds"], "s", rec.get("cost_usd"), flush=True)


if __name__ == "__main__":
    main()
