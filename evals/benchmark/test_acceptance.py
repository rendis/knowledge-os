"""Generic acceptance controls: grading errors and unrelated flags cannot count as detection."""
import json
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import bench
import acceptance


class AcceptanceTests(unittest.TestCase):
    def case(self, **changes):
        q = {"question": "What does the implementation establish?", "expected": ["implementation only"], "must_not": [],
             "evidence": [{"id": "S1", "source": "service@abc:1", "content": "1: return configured_value"}]}
        return {"id": "control", "question": q, "answer": "The implementation returns configuration [service:1].",
                "expect": "accept", "failed_facts": [], "evidence_failure": False, **changes}

    def grade(self, **changes):
        return {"points": [1], "losses": [None], "extra_points": [], "violations": [], "unsupported_claims": [],
                "uncited_claims": [], "evidence_verified": True, "verdict": "correct",
                "claims": [{"claim": "Implementation returns configuration", "kind": "fact", "status": "supported", "sources": ["S1"], "cited": True}], **changes}

    def test_accepts_a_complete_source_backed_control(self):
        self.assertTrue(acceptance.assess(self.case(), self.grade())["matched"])

    def test_error_or_unavailable_audit_is_not_successful_detection(self):
        case = self.case(expect="reject", evidence_failure=True)
        for grade in ({"verdict": "error", "evidence_verified": False}, self.grade(evidence_verified=None), {}):
            with self.subTest(grade=grade):
                result = acceptance.assess(case, grade)
                self.assertEqual(result["decision"], "inconclusive")
                self.assertFalse(result["matched"])

    def test_unrelated_missing_fact_cannot_satisfy_the_control(self):
        case = self.case(expect="reject", failed_facts=[0])
        case["question"]["expected"].append("a second fact")
        grade = self.grade(points=[1, 0], losses=[None, "omitted"], verdict="partial")
        self.assertFalse(acceptance.assess(case, grade)["matched"])
        grade.update(points=[0, 1], losses=["omitted", None])
        self.assertTrue(acceptance.assess(case, grade)["matched"])

    def test_full_coverage_and_correct_verdict_do_not_hide_a_false_claim(self):
        grade = self.grade()
        grade["claims"][0]["status"] = "contradicted"
        result = acceptance.assess(self.case(expect="reject", evidence_failure=True), grade)
        self.assertEqual(result["decision"], "reject")
        self.assertTrue(result["matched"])

    def test_rounded_score_cannot_hide_a_missing_fact(self):
        case = self.case()
        case["question"]["expected"] *= 2001
        grade = self.grade(points=[0] + [1] * 2000, losses=["omitted"] + [None] * 2000, score=1.0)
        self.assertEqual(acceptance.assess(case, grade)["decision"], "reject")

    def test_labels_and_expected_decisions_do_not_reach_the_judge(self):
        case = self.case(id="secret-negative-label", expect="reject", evidence_failure=True)
        with patch.object(acceptance.judge, "grade", return_value=self.grade()) as grade:
            acceptance.evaluate(case, 1)
        q, rec = grade.call_args.args
        self.assertEqual(q, case["question"])
        self.assertEqual(rec, {"answer": case["answer"], "returncode": 0})
        self.assertNotIn("secret-negative-label", json.dumps([q, rec]))

    def test_execution_exception_does_not_count_as_detecting_bad_answers(self):
        with tempfile.TemporaryDirectory() as scratch, patch.object(acceptance.judge, "grade", side_effect=RuntimeError("unavailable")):
            cases = [self.case(), self.case(id="negative", expect="reject", evidence_failure=True)]
            with patch("builtins.print"):
                result = acceptance.run(cases, pathlib.Path(scratch) / "results", 1, 2)
        self.assertEqual(result["inconclusive"], 2)
        self.assertEqual(result["matched"], 0)
        self.assertFalse(result["passed"])

    def test_private_suite_preflight_and_repeat_results(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            (root / "answer.txt").write_text("Configuration [service:1].")
            base = {"id": "positive", "question": self.case()["question"], "answer_file": "answer.txt", "expect": "accept"}
            suite = root / "suite.json"
            controls = [base, {**base, "id": "negative", "expect": "reject", "failed_facts": [0]}]
            suite.write_text(json.dumps({"cases": controls}))
            cases = acceptance.load_suite(suite)
            with patch.object(acceptance.judge, "grade", return_value=self.grade()), patch("builtins.print"):
                result = acceptance.run(cases, root / "results", 2, 2)
            self.assertEqual(result["runs"], 4)
            self.assertEqual(result["false_accepts"], 2)
            self.assertFalse(result["passed"])
            self.assertTrue((root / "results/inputs.json").exists())
            self.assertEqual(len(list((root / "results").glob("result-*.json"))), 4)
            with self.assertRaisesRegex(ValueError, "preserve previous attempts"):
                acceptance.run(cases, root / "results", 1, 1)
            for invalid in ([base], [base, base], [base, {**controls[1], "failed_facts": [9]}],
                            [base, {**controls[1], "failed_facts": [{}]}],
                            [base, {**controls[1], "question": {**base["question"], "evidence": []}}]):
                suite.write_text(json.dumps({"cases": invalid}))
                with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                    acceptance.load_suite(suite)


if __name__ == "__main__":
    unittest.main()
