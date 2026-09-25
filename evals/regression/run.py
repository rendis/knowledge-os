"""Run the regression questions against installed cell vaults through a real agent harness.

python3 -B evals/regression/run.py --harness claude|codex|cursor --questions FILE --out DIR \
    --vault a=PATH --vault b=PATH [--ids S1,S2] [--parallel 4]

Each question runs in a fresh headless session whose working directory is the vault, exactly as a
developer would ask it. Answers, usage and duration are stored per question for judge.py.
"""
import argparse, concurrent.futures as cf, json, os, pathlib, subprocess, time

HERE = pathlib.Path(__file__).parent
SUFFIX = "\n\n(Consulta de solo lectura: no modifiques archivos ni ejecutes acciones con efectos.)"


def command(harness, prompt, out_file):
    if harness == "claude":
        return ["claude", "-p", prompt, "--model", "opus", "--effort", "medium", "--output-format", "json", "--permission-mode", "bypassPermissions"]
    if harness == "codex":
        return ["codex", "exec", "--skip-git-repo-check", "-m", "gpt-5.5", "-c", 'model_reasoning_effort="medium"', "-s", "read-only", "--json", "--output-last-message", str(out_file), prompt]
    if harness == "cursor":
        return ["cursor-agent", "-p", "--output-format", "json", "--force", prompt]
    raise SystemExit("unknown harness")


def run_one(harness, q, vault, out_dir):
    out_dir.mkdir(parents=True, exist_ok=True)
    last = out_dir / f"{q['id']}.last.txt"
    start = time.time()
    try:
        r = subprocess.run(command(harness, q["question"] + SUFFIX, last), cwd=vault, capture_output=True, text=True, timeout=1500)
        stdout, rc = r.stdout, r.returncode
    except subprocess.TimeoutExpired as e:
        stdout, rc = (e.stdout or b"").decode() if isinstance(e.stdout, bytes) else (e.stdout or ""), "timeout"
    rec = {"id": q["id"], "harness": harness, "vault": vault, "seconds": round(time.time() - start, 1), "returncode": rc}
    if harness in ("claude", "cursor"):
        try:
            j = json.loads(stdout)
            rec["answer"] = j.get("result", "")
            rec["usage"] = j.get("usage", {})
            rec["cost_usd"] = j.get("total_cost_usd")
            rec["turns"] = j.get("num_turns")
        except Exception:
            rec["answer"] = stdout[-8000:]
    else:
        rec["answer"] = last.read_text() if last.exists() else stdout[-8000:]
        usage = {"input_tokens": 0, "cached_input_tokens": 0, "output_tokens": 0}
        commands = 0
        for line in stdout.splitlines():
            try:
                ev = json.loads(line)
            except ValueError:
                continue
            if ev.get("type") == "turn.completed":
                for k in usage:
                    usage[k] += ev.get("usage", {}).get(k, 0)
            if ev.get("type") == "item.completed" and ev.get("item", {}).get("type") == "command_execution":
                commands += 1
        rec["usage"], rec["commands"] = usage, commands
    (out_dir / f"{q['id']}.json").write_text(json.dumps(rec, ensure_ascii=False, indent=1))
    return rec


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--harness", required=True)
    p.add_argument("--questions", required=True, help="cell-owned question set (kept outside this repository)")
    p.add_argument("--out", required=True)
    p.add_argument("--vault", action="append", required=True)
    p.add_argument("--ids", default="")
    p.add_argument("--parallel", type=int, default=4)
    a = p.parse_args()
    vaults = dict(v.split("=", 1) for v in a.vault)
    qs = [q for q in json.load(open(a.questions)) if q["vault"] in vaults and (not a.ids or q["id"] in a.ids.split(","))]
    with cf.ThreadPoolExecutor(a.parallel) as ex:
        for rec in ex.map(lambda q: run_one(a.harness, q, vaults[q["vault"]], pathlib.Path(a.out)), qs):
            print(rec["id"], rec["returncode"], rec["seconds"], "s", rec.get("cost_usd"), flush=True)


if __name__ == "__main__":
    main()
