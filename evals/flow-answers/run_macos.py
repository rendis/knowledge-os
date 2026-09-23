"""Run the synthetic conversation with an OS-enforced answer-key boundary on macOS."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

from prepare import HERE, prepare


def sandbox_profile(root, distribution):
    readable = [root / "worker", root / "codex-home", root / "bin"]
    writable = [root / "worker", root / "codex-home"]
    read_rules = "\n".join(f'    (subpath "{path}")' for path in readable)
    write_rules = "\n".join(f'    (subpath "{path}")' for path in writable)
    return f"""(version 1)
(allow default)
(deny file-read*
    (subpath "/Users")
    (subpath "/private/tmp")
    (subpath "{Path.home()}")
    (subpath "{distribution}")
)
(allow file-read*
    (literal "/private/tmp")
    (literal "{root}")
{read_rules}
)
(deny file-write*
    (subpath "/Users")
    (subpath "/private/tmp")
    (subpath "{Path.home()}")
    (subpath "{distribution}")
)
(allow file-write*
{write_rules}
)
"""


def preflight(profile, worker, evaluator, distribution, decoy):
    env = os.environ.copy()
    env["GIT_CONFIG_GLOBAL"] = "/dev/null"
    command = (
        'cat "$1/task.txt" >/dev/null || exit 10; '
        'for path in "$2/evaluation.json" "$3/evals/flow-answers/rubric.json" '
        '"$4/.codex/auth.json" "$5"; do '
        'if cat "$path" >/dev/null 2>&1; then exit 11; fi; '
        'done'
    )
    result = subprocess.run(
        ["sandbox-exec", "-f", str(profile), "/bin/sh", "-c", command,
         "preflight", str(worker), str(evaluator), str(distribution),
         str(Path.home()), str(decoy)],
        cwd=worker, env=env, capture_output=True, text=True,
    )
    if result.returncode:
        raise RuntimeError(f"OS read-boundary preflight failed ({result.returncode})")
    binding = subprocess.run(
        ["sandbox-exec", "-f", str(profile), "python3", "-B",
         str(worker / "vault/90-Meta/workspace-config.py"),
         "--vault-root", str(worker / "vault"), "locate-repository",
         "https://example.invalid/boreal/exit-adapter.git"],
        cwd=worker, env=env, capture_output=True, text=True,
    )
    if binding.returncode or json.loads(binding.stdout).get("status") != "ok":
        raise RuntimeError("Configured source binding failed inside the OS boundary")


def run_turn(args, prompt, env, profile, worker, evaluator, name):
    result = subprocess.run(
        ["sandbox-exec", "-f", str(profile), *args],
        cwd=worker, env=env, input=prompt + "\n", capture_output=True, text=True,
    )
    (evaluator / f"{name}.jsonl").write_text(result.stdout)
    (evaluator / f"{name}.stderr").write_text(result.stderr)
    events = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
    if (result.returncode or
            any(e.get("type") == "item.completed" and
                (e.get("item") or {}).get("type") == "error" for e in events) or
            not any(e.get("type") == "turn.completed" for e in events)):
        raise RuntimeError(f"{name} failed; inspect its evaluator-side event and error logs")
    return events[0]["thread_id"]


def copy_traces(codex_home, evaluator):
    traces = evaluator / "traces"
    traces.mkdir(exist_ok=True)
    for source in (codex_home / "sessions").rglob("*.jsonl"):
        shutil.copy2(source, traces / source.name)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True, type=Path, help="Fresh, private output directory")
    parser.add_argument("--codex", type=Path, default=Path.home() / ".local/bin/codex")
    parser.add_argument("--model", help="Explicit model; omitted uses the CLI default")
    parser.add_argument("--preflight-only", action="store_true")
    options = parser.parse_args()

    root = options.root.resolve()
    if Path("/private/tmp") not in root.parents:
        parser.error("--root must be a fresh directory under /private/tmp")
    if root.exists():
        parser.error("--root must not exist")
    root.mkdir(mode=0o700, parents=True)
    worker, evaluator = root / "worker", root / "evaluator"
    distribution = HERE.parents[1]
    evaluation = prepare(distribution, worker, evaluator)
    decoy = root / "decoy-key.txt"
    decoy.write_text("unreadable probe\n")
    profile = root / "profile.sb"
    profile.write_text(sandbox_profile(root, distribution))
    preflight(profile, worker, evaluator, distribution, decoy)
    decoy.unlink()
    if options.preflight_only:
        print(root)
        return

    codex_home = root / "codex-home"
    codex_home.mkdir(mode=0o700)
    staged_bin = root / "bin"
    staged_bin.mkdir()
    staged_binary = staged_bin / "codex"
    source_binary = options.codex.resolve()
    shutil.copy2(source_binary, staged_binary)
    shutil.copy2(source_binary.with_name("codex-code-mode-host"),
                 staged_bin / "codex-code-mode-host")
    auth = codex_home / "auth.json"
    shutil.copyfile(Path.home() / ".codex/auth.json", auth)
    auth.chmod(0o600)
    env = os.environ.copy()
    env["CODEX_HOME"] = str(codex_home)
    env["GIT_CONFIG_GLOBAL"] = "/dev/null"
    common = [str(staged_binary), "exec", "--ignore-user-config", "--ignore-rules",
              "--json", "--dangerously-bypass-approvals-and-sandbox"]
    try:
        initial = common + ["--skip-git-repo-check", "-C", str(worker)]
        if options.model:
            initial += ["-m", options.model]
        thread_id = run_turn(initial + ["-"], evaluation["turns"][0]["prompt"],
                             env, profile, worker, evaluator, "initial")
        for turn in evaluation["turns"][1:]:
            resumed = [str(staged_binary), "exec", "resume",
                       "--ignore-user-config", "--ignore-rules", "--json",
                       "--dangerously-bypass-approvals-and-sandbox", thread_id, "-"]
            run_turn(resumed, turn["prompt"], env, profile, worker, evaluator, turn["id"])
    finally:
        auth.unlink(missing_ok=True)
        copy_traces(codex_home, evaluator)
        shutil.rmtree(codex_home)
        shutil.rmtree(staged_bin)

    for name, expected in evaluation["source_sha256"].items():
        actual = hashlib.sha256((worker / name).read_bytes()).hexdigest()
        if actual != expected:
            raise RuntimeError(f"Worker modified a source: {name}")
    repo_status = subprocess.run(
        ["git", "status", "--porcelain"], cwd=worker / "repos/exit-adapter",
        check=True, capture_output=True, text=True,
    ).stdout
    if repo_status:
        raise RuntimeError("Worker modified the synthetic source checkout")
    trace_files = list((evaluator / "traces").glob("*.jsonl"))
    observed_models = set()
    for path in trace_files:
        for line in path.open():
            event = json.loads(line)
            if event.get("type") == "turn_context":
                model = (event.get("payload") or {}).get("model")
                if model:
                    observed_models.add(model)
    (evaluator / "run.json").write_text(json.dumps({
        "thread_id": thread_id,
        "model_requested": options.model,
        "models_observed": sorted(observed_models),
        "session_trace_count": len(trace_files),
        "profile_sha256": hashlib.sha256(profile.read_bytes()).hexdigest(),
        "preflight": "worker and bound Git source readable; host home, evaluator and other temporary runs denied",
        "source_hashes_preserved": True,
        "repository_clean": True,
        "assessment": "pending independent inspection of answers and full traces",
    }, indent=2) + "\n")
    print(root)


if __name__ == "__main__":
    main()
