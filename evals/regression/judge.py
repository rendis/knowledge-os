"""Grade regression answers against the expected facts with an independent headless judge.

python3 -B evals/regression/judge.py --questions FILE --answers DIR [--answers DIR2 ...]
Writes <DIR>/grades.json and prints accuracy, violations, time and cost per run.
"""
import argparse, concurrent.futures as cf, json, pathlib, re, subprocess, sys

sys.path.insert(0, str(pathlib.Path(__file__).parent))
from run import isolated_env  # noqa: E402

HERE = pathlib.Path(__file__).parent
# Fixed judge for every graded run, so settings are compared under the same grader. The judge sees only
# the question, the expected facts and the answer text: never the harness, model or effort that wrote it.
JUDGE = {"harness": "codex", "model": "gpt-5.5", "effort": "medium"}
PROMPT = """You are a strict grader. Compare the ANSWER with the EXPECTED FACTS and the FORBIDDEN CLAIMS. The question, facts and answer may be in any language.
Question: {question}
Expected facts: what the question asks for (each is worth 1 point; 0.5 when partial or imprecise; 0 when missing or wrong):
{expected}
Additional context: useful but not asked for; it neither adds to nor subtracts from the score, only record which ones appear (1 if it appears correctly, 0 if not):
{extra}
Forbidden claims (asserting one without reservation is a violation):
{must_not}
ANSWER:
<<<
{answer}
>>>
A concise answer that covers what was asked is correct: do not penalize it for leaving out the additional context.
For every expected fact scored below 1, name why the point was lost with exactly one kind:
- "omitted": the answer does not address it;
- "abstained": the answer declares it unknown or unverified;
- "wrong": the answer states something incompatible with it;
- "direction": the answer inverts a direction (producer and consumer, caller and callee, source and target, cause and effect);
- "path": the answer describes another path, branch, case or environment than the one the fact requires, or only one of several;
- "imprecise": the answer is close but incomplete or vague.
Reply ONLY with a JSON object: {{"points": [<score per expected fact, in order>], "losses": [<null for a full point, otherwise the kind, one per expected fact, in order>], "extra_points": [<1 or 0 per additional context item, in order>], "violations": [<text of each violation>], "unsupported_claims": [<relevant technical claims that do not follow from cited evidence or look invented>], "verdict": "correct|partial|incorrect"}}"""
LOSS_KINDS = ("omitted", "abstained", "wrong", "direction", "path", "imprecise")


def grade(q, rec):
    prompt = PROMPT.format(question=q["question"], expected="\n".join(f"- {e}" for e in q["expected"]), extra="\n".join(f"- {e}" for e in q.get("extra", [])) or "- (none)",
                           must_not="\n".join(f"- {m}" for m in q["must_not"]) or "- (none)", answer=rec.get("answer", "")[:12000])
    text = ""
    for _ in range(3):  # a hung judge session is retried, never allowed to abort the whole grading
        try:
            text = subprocess.run(["codex", "exec", "--skip-git-repo-check", "-m", JUDGE["model"], "-c", f'model_reasoning_effort="{JUDGE["effort"]}"', "-s", "read-only", prompt], stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=300, cwd="/tmp", env=isolated_env("codex")).stdout
        except subprocess.TimeoutExpired:
            continue
        if re.search(r"\{.*\}", text, re.S):
            break
    m = re.search(r"\{.*\}", text, re.S)
    g = json.loads(m.group(0)) if m else {"verdict": "error", "points": [], "violations": [], "unsupported_claims": []}
    g["score"] = round(sum(g.get("points", [])) / max(len(q["expected"]), 1), 3)
    g["losses"] = [k if k in LOSS_KINDS else None for k in (g.get("losses") or [])]
    if q.get("extra"):
        g["extra_score"] = round(sum(g.get("extra_points", [])) / len(q["extra"]), 3)
    return g


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--answers", action="append", required=True)
    p.add_argument("--questions", required=True)
    a = p.parse_args()
    qs = {q["id"]: q for q in json.load(open(a.questions))}
    for d in map(pathlib.Path, a.answers):
        recs = [json.loads(f.read_text()) for f in sorted(d.glob("*.json")) if f.name != "grades.json"]
        with cf.ThreadPoolExecutor(6) as ex:
            grades = dict(zip([r["id"] for r in recs], ex.map(lambda r: grade(qs[r["id"]], r), recs)))
        (d / "grades.json").write_text(json.dumps(grades, ensure_ascii=False, indent=1))
        n = len(recs)
        score = sum(g["score"] for g in grades.values()) / max(n, 1)
        viol = sum(len(g.get("violations", [])) for g in grades.values())
        uns = sum(len(g.get("unsupported_claims", [])) for g in grades.values())
        cost = sum((r.get("cost_usd") or 0) for r in recs)
        tokens = sum((r.get("usage") or {}).get("input_total", (r.get("usage") or {}).get("input_tokens", 0)) for r in recs)
        out_tokens = sum((r.get("usage") or {}).get("output", (r.get("usage") or {}).get("output_tokens", 0)) for r in recs)
        secs = sum(r["seconds"] for r in recs)
        print(f"{d.name}: n={n} mean_score={score:.3f} verdicts={[grades[r['id']]['verdict'] for r in recs]} violations={viol} unsupported={uns} seconds={secs:.0f} input_tokens={tokens} output_tokens={out_tokens} cost_usd={cost:.2f}")
        s = summary(recs, grades)
        print(f"  by vault: {s['by_vault']}  lost points by kind: {s['losses']}")


def summary(recs, grades):
    """Score and violations per vault, and the lost points by kind: where the answers fail, not only how much."""
    by_vault, losses = {}, {}
    for r in recs:
        g = grades.get(r["id"]) or {}
        v = by_vault.setdefault(r.get("vault", "?"), {"n": 0, "score": 0.0, "violations": 0})
        v["n"] += 1
        v["score"] += g.get("score", 0)
        v["violations"] += len(g.get("violations", []))
        for kind, pts in zip(g.get("losses") or [], g.get("points") or []):
            if kind:
                losses[kind] = round(losses.get(kind, 0) + 1 - pts, 2)
    for v in by_vault.values():
        v["score"] = round(v["score"] / v["n"], 3)
    return {"by_vault": by_vault, "losses": dict(sorted(losses.items(), key=lambda x: -x[1]))}


if __name__ == "__main__":
    main()
