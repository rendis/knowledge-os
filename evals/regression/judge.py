"""Grade regression answers against the expected facts with an independent headless judge.

python3 -B evals/regression/judge.py --questions FILE --answers DIR [--answers DIR2 ...]
Writes <DIR>/grades.json and prints accuracy, violations, time and cost per run.
"""
import argparse, concurrent.futures as cf, json, pathlib, re, subprocess

HERE = pathlib.Path(__file__).parent
PROMPT = """Eres un evaluador estricto. Compara la RESPUESTA con los HECHOS ESPERADOS y las AFIRMACIONES PROHIBIDAS.
Pregunta: {question}
Hechos esperados (cada uno vale 1 punto; 0.5 si está parcial o impreciso; 0 si falta o es incorrecto):
{expected}
Afirmaciones prohibidas (si la respuesta afirma alguna sin reserva, es una violación):
{must_not}
RESPUESTA:
<<<
{answer}
>>>
Responde SOLO un objeto JSON: {{"points": [<puntaje por hecho en orden>], "violations": [<texto de cada violación>], "unsupported_claims": [<afirmaciones técnicas relevantes que no se desprenden de evidencia citada o parecen inventadas>], "verdict": "correct|partial|incorrect"}}"""


def grade(q, rec):
    prompt = PROMPT.format(question=q["question"], expected="\n".join(f"- {e}" for e in q["expected"]), must_not="\n".join(f"- {m}" for m in q["must_not"]) or "- (ninguna)", answer=rec.get("answer", "")[:12000])
    # Independent judge from another model family than the Claude answers (Codex gpt-5.5).
    text = ""
    for _ in range(3):  # a hung judge session is retried, never allowed to abort the whole grading
        try:
            text = subprocess.run(["codex", "exec", "--skip-git-repo-check", "-m", "gpt-5.5", "-c", 'model_reasoning_effort="medium"', "-s", "read-only", prompt], capture_output=True, text=True, timeout=300, cwd="/tmp").stdout
        except subprocess.TimeoutExpired:
            continue
        if re.search(r"\{.*\}", text, re.S):
            break
    m = re.search(r"\{.*\}", text, re.S)
    g = json.loads(m.group(0)) if m else {"verdict": "error", "points": [], "violations": [], "unsupported_claims": []}
    g["score"] = round(sum(g.get("points", [])) / max(len(q["expected"]), 1), 3)
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
        tokens = sum((r.get("usage") or {}).get("input_tokens", 0) for r in recs)
        out_tokens = sum((r.get("usage") or {}).get("output_tokens", 0) for r in recs)
        secs = sum(r["seconds"] for r in recs)
        print(f"{d.name}: n={n} mean_score={score:.3f} verdicts={[grades[r['id']]['verdict'] for r in recs]} violations={viol} unsupported={uns} seconds={secs:.0f} input_tokens={tokens} output_tokens={out_tokens} cost_usd={cost:.2f}")


if __name__ == "__main__":
    main()
