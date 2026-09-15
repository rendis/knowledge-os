"""Extract non-content runtime evidence; semantic verdicts require review."""
import argparse
import hashlib
import json
from pathlib import Path


def summarize(directory):
    summary = json.loads((directory / "summary.json").read_text())
    for run in summary["runs"]:
        capture = directory / f"{run['vault']}-{run['case']}"
        run.pop("diff", None)
        run["runtime_error_count"] = len(run.pop("runtime_errors", []))
        # Paths and file contents can identify private consumer systems.
        changes = run.pop("changed_files", None)
        run["changed_file_count"] = len(changes) if changes is not None else None
        run["sessions"] = []
        for path in sorted((capture / "rollouts").rglob("*.jsonl")):
            session = {"settings": [], "delegation_calls": []}
            for line in path.read_text().splitlines():
                event = json.loads(line)
                payload = event.get("payload", {})
                if event["type"] == "turn_context":
                    settings = {key: payload.get(key) for key in ("model", "effort")}
                    if settings not in session["settings"]:
                        session["settings"].append(settings)
                if event["type"] == "response_item" and payload.get("namespace") == "collaboration":
                    if payload.get("type") != "function_call":
                        continue
                    args = json.loads(payload["arguments"])
                    call = {"tool": payload["name"]}
                    for key in ("model", "reasoning_effort", "fork_turns"):
                        if key in args:
                            call[key] = args[key]
                    # Correlate follow-ups without retaining task names or messages.
                    if "target" in args:
                        call["target_hash"] = hashlib.sha256(args["target"].encode()).hexdigest()[:12]
                    session["delegation_calls"].append(call)
                if event["type"] == "event_msg" and payload.get("type") == "token_count":
                    info = payload.get("info") or {}
                    if info.get("total_token_usage"):
                        session["reported_total_usage"] = info["total_token_usage"]
            run["sessions"].append(session)
    return summary


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    args.output.write_text(json.dumps(summarize(args.directory), indent=2) + "\n")
