"""Replay private positive and negative answer controls against the fixed source judge."""
import argparse
import concurrent.futures as cf
import hashlib
import json
import pathlib
import sys

import judge


def digest(value):
    return hashlib.sha256(json.dumps(value, ensure_ascii=False, sort_keys=True).encode()).hexdigest()


def load_suite(path):
    path = pathlib.Path(path)
    suite = json.loads(path.read_text())
    cases = suite.get("cases") if isinstance(suite, dict) else None
    if not isinstance(cases, list) or not cases:
        raise ValueError("suite requires nonempty cases")
    prepared, ids = [], set()
    for case in cases:
        if not isinstance(case, dict) or not isinstance(case.get("id"), str) or not case["id"].strip() or case["id"] in ids:
            raise ValueError("case IDs must be nonempty and unique")
        ids.add(case["id"])
        q = case.get("question")
        if (not isinstance(q, dict) or not isinstance(q.get("question"), str) or not q["question"].strip()
                or not isinstance(q.get("expected"), list) or not q["expected"]
                or not all(isinstance(s, str) and s.strip() for s in q["expected"])
                or not isinstance(q.get("must_not"), list) or not all(isinstance(s, str) for s in q["must_not"])
                or not judge.valid_evidence(q.get("evidence"))):
            raise ValueError(f"{case['id']}: invalid question or source packet")
        if "extra" in q and (not isinstance(q["extra"], list) or not all(isinstance(s, str) for s in q["extra"])):
            raise ValueError(f"{case['id']}: invalid extra facts")
        expected = case.get("expect")
        failed = case.get("failed_facts", [])
        evidence_failure = case.get("evidence_failure", False)
        if (expected not in ("accept", "reject") or type(evidence_failure) is not bool
                or not isinstance(failed, list)
                or not all(type(i) is int and 0 <= i < len(q["expected"]) for i in failed)
                or len(set(failed)) != len(failed)
                or (expected == "accept" and (failed or evidence_failure))
                or (expected == "reject" and not (failed or evidence_failure))):
            raise ValueError(f"{case['id']}: rejection needs a specific failing fact or source-audit failure")
        filename = case.get("answer_file")
        if not isinstance(filename, str) or not filename:
            raise ValueError(f"{case['id']}: answer_file is required")
        answer = (path.parent / filename).read_bytes()
        if len(answer) > judge.MAX_ANSWER_BYTES:
            raise ValueError(f"{case['id']}: oversized answer")
        answer = answer.decode("utf-8")
        if not answer.strip():
            raise ValueError(f"{case['id']}: empty answer")
        # Only question/source fields reach the judge, never the expected decision or case labels.
        q = {k: q[k] for k in ("question", "expected", "must_not", "extra", "evidence") if k in q}
        prepared.append({"id": case["id"], "question": q, "answer": answer, "expect": expected,
                         "failed_facts": failed, "evidence_failure": evidence_failure})
    if {c["expect"] for c in prepared} != {"accept", "reject"}:
        raise ValueError("suite requires both positive and negative controls")
    return prepared


def assess(case, grade):
    """A grader failure is inconclusive, never a successful rejection of a negative control."""
    q = case["question"]
    if not judge.valid_grade(grade, q) or type(grade.get("evidence_verified")) is not bool:
        return {"decision": "inconclusive", "matched": False, "reason": "invalid or unavailable source grade"}
    source_failure = bool(grade.get("unsupported_claims") or grade.get("uncited_claims") or grade.get("violations"))
    source_failure |= any(c["status"] != "supported" or (not c["cited"] and c["kind"] != "boundary") for c in grade["claims"])
    source_failure |= grade["evidence_verified"] is False
    # Inspect every point; a rounded coverage score of 1 can conceal a missing fact.
    complete = all(point == 1 for point in grade["points"])
    decision = "accept" if complete and not source_failure and grade["verdict"] == "correct" else "reject"
    target_found = (all(grade["points"][i] < 1 for i in case["failed_facts"])
                    and (not case["evidence_failure"] or source_failure))
    return {"decision": decision, "matched": decision == case["expect"] and target_found,
            "source_failure": source_failure, "complete": complete, "target_found": target_found}


def evaluate(case, repetition):
    grade = judge.grade(case["question"], {"answer": case["answer"], "returncode": 0})
    return {"id": case["id"], "repetition": repetition, "expect": case["expect"],
            "question_sha256": digest(case["question"]),
            "answer_sha256": hashlib.sha256(case["answer"].encode()).hexdigest(),
            "assessment": assess(case, grade), "grade": grade}


def run(cases, out, repetitions, parallel):
    if repetitions < 1 or parallel < 1:
        raise ValueError("repetitions and parallel must be positive")
    out = pathlib.Path(out)
    if out.exists() and any(out.iterdir()):
        raise ValueError("output directory must be new or empty; preserve previous attempts")
    out.mkdir(parents=True, exist_ok=True)
    fingerprint = {"judge": judge.JUDGE, "judge_sha256": hashlib.sha256(pathlib.Path(judge.__file__).read_bytes()).hexdigest(),
                   "runner_sha256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
                   "suite_sha256": digest(cases), "repetitions": repetitions, "parallel": parallel}
    (out / "inputs.json").write_text(json.dumps({"fingerprint": fingerprint, "cases": cases}, ensure_ascii=False, indent=2))
    jobs = [(case, repetition) for repetition in range(1, repetitions + 1) for case in cases]
    records = []
    with cf.ThreadPoolExecutor(parallel) as pool:
        pending = {pool.submit(evaluate, case, repetition): (case, repetition) for case, repetition in jobs}
        for future in cf.as_completed(pending):
            case, repetition = pending[future]
            try:
                record = future.result()
            except Exception as error:
                record = {"id": case["id"], "repetition": repetition, "expect": case["expect"],
                          "assessment": {"decision": "inconclusive", "matched": False}, "error": str(error)}
            records.append(record)
            # Numeric file names avoid interpreting private case IDs as output paths.
            (out / f"result-{len(records):03d}.json").write_text(json.dumps(record, ensure_ascii=False, indent=2))
            a = record["assessment"]
            print(f"{case['id']} run={repetition}: {a['decision']} matched={a['matched']}", flush=True)
    records.sort(key=lambda r: (r["id"], r["repetition"]))
    result = {"fingerprint": fingerprint, "runs": len(records),
              "matched": sum(r["assessment"]["matched"] for r in records),
              "false_accepts": sum(r["expect"] == "reject" and r["assessment"]["decision"] == "accept" for r in records),
              "false_rejects": sum(r["expect"] == "accept" and r["assessment"]["decision"] == "reject" for r in records),
              "inconclusive": sum(r["assessment"]["decision"] == "inconclusive" for r in records),
              "passed": all(r["assessment"]["matched"] for r in records), "records": records,
              "limits": "Evaluator calibration on known controls, not held-out answer quality or a delivery/release gate. Adjudicate model findings against the sources."}
    (out / "report.json").write_text(json.dumps(result, ensure_ascii=False, indent=2))
    return result


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--suite", required=True, help="private positive/negative control suite")
    p.add_argument("--out", required=True, help="new or empty private results directory")
    p.add_argument("--repetitions", type=int, default=3)
    p.add_argument("--parallel", type=int, default=3)
    args = p.parse_args()
    try:
        cases = load_suite(args.suite)
        result = run(cases, args.out, args.repetitions, args.parallel)
    except (ValueError, OSError, UnicodeError) as error:
        p.error(str(error))
    print(f"matched={result['matched']}/{result['runs']} inconclusive={result['inconclusive']} passed={result['passed']}")
    return 0 if result["passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
