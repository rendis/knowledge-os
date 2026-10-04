"""Delivered claim verification must be separate from expected-fact coverage."""
import json
import os
import contextlib
import io
import pathlib
import tempfile
import types
import unittest
from unittest.mock import patch

import bench


class EvidenceTests(unittest.TestCase):
    question = {"question": "What selects the country?", "expected": ["configuration"], "must_not": [],
                "evidence": [{"id": "S1", "source": "config.go@abc:12", "content": "12: country = config.DefaultCountry"}]}

    def result(self, **changes):
        return {"points": [1], "losses": [None], "extra_points": [], "violations": [], "unsupported_claims": [],
                "verdict": "correct", "claims": [{"claim": "Configuration selects the country.", "kind": "fact", "status": "supported",
                                                   "sources": ["S1"], "cited": True}], **changes}

    def grade(self, result, question=None):
        response = {"answer": json.dumps(result), "returncode": 0, "seconds": 1}
        with patch.object(bench.judge, "execute", return_value=response) as execute:
            grade = bench.judge.grade(question or self.question, {"answer": "Configuration selects the country [config.go:12]."})
        return grade, execute

    def test_source_evidence_reaches_the_judge(self):
        grade, execute = self.grade(self.result())
        prompt = execute.call_args.args[1]
        self.assertIn("country = config.DefaultCountry", prompt)
        self.assertTrue(grade["evidence_verified"])

    def test_source_packet_schema_requires_claims_without_changing_legacy_grades(self):
        _, execute = self.grade(self.result())
        self.assertIn('"claims": [', execute.call_args.args[1].split("SOURCE PACKET")[0])
        question = {k: v for k, v in self.question.items() if k != "evidence"}
        _, execute = self.grade(self.result(), question)
        self.assertNotIn('"claims": [', execute.call_args.args[1])

    def test_full_coverage_does_not_override_an_unsupported_claim(self):
        result = self.result()
        result["claims"][0]["status"] = "unverified"
        grade, _ = self.grade(result)
        self.assertEqual(grade["score"], 1)
        self.assertFalse(grade["evidence_verified"])
        self.assertTrue(grade["unsupported_claims"])

    def test_true_but_uncited_claim_does_not_pass_delivery(self):
        result = self.result()
        result["claims"][0]["cited"] = False
        grade, _ = self.grade(result)
        self.assertFalse(grade["evidence_verified"])
        self.assertEqual(grade["uncited_claims"], [result["claims"][0]["claim"]])

    def test_empty_audit_unknown_source_or_missing_audit_cannot_pass(self):
        for claims in ([], [{"claim": "fact", "status": "supported", "sources": ["invented"], "cited": True}], None):
            with self.subTest(claims=claims):
                grade, _ = self.grade(self.result(claims=claims))
                self.assertEqual(grade["verdict"], "error")
                self.assertFalse(grade["evidence_verified"])

    def test_no_source_packet_means_verification_unavailable(self):
        question = {k: v for k, v in self.question.items() if k != "evidence"}
        grade, _ = self.grade(self.result(), question)
        self.assertIsNone(grade["evidence_verified"])
        summary = bench.judge.summary([{"id": "Q", "vault": "probe"}], {"Q": grade})["by_vault"]["probe"]
        self.assertEqual(summary["verified_answers"], 0)
        self.assertEqual(summary["evidence_unavailable"], 1)

    def test_malformed_or_oversized_packet_fails_without_a_judge_request(self):
        for packet in ([], [{"id": "S1", "source": "file", "content": "x" * (1024 * 1024)}],
                       [{"id": "S1", "source": "file"}], [self.question["evidence"][0]] * 2):
            question = {**self.question, "evidence": packet}
            with self.subTest(packet_type=type(packet)), patch.object(bench.judge, "execute") as execute:
                grade = bench.judge.grade(question, {"answer": "fact"})
            execute.assert_not_called()
            self.assertEqual(grade["verdict"], "error")
            self.assertFalse(grade["evidence_verified"])

    def test_scope_overclaim_does_not_pass_even_if_expected_facts_are_complete(self):
        result = self.result()
        result["claims"][0].update(claim="All production stores are healthy.", status="unverified", sources=[])
        grade, _ = self.grade(result)
        self.assertFalse(grade["evidence_verified"])

    def test_grounded_scope_boundary_does_not_need_an_extra_citation(self):
        result = self.result()
        result["claims"].append({"claim": "This code leaves current runtime unverified.", "kind": "boundary",
                                 "status": "supported", "sources": ["S1"], "cited": False})
        grade, _ = self.grade(result)
        self.assertTrue(grade["evidence_verified"])

    def test_mislabeled_boundary_cannot_override_missing_support(self):
        result = self.result()
        result["claims"].append({"claim": "All production stores are healthy.", "kind": "boundary",
                                 "status": "unverified", "sources": [], "cited": False})
        grade, _ = self.grade(result)
        self.assertFalse(grade["evidence_verified"])

    def test_reporting_keeps_coverage_and_evidence_separate(self):
        grade, _ = self.grade(self.result())
        grade["score"] = 0.5
        summary = bench.judge.summary([{"id": "Q", "vault": "probe"}], {"Q": grade})["by_vault"]["probe"]
        self.assertEqual(summary["verified_answers"], 1)
        self.assertEqual(summary["clean_answers"], 0)

    def test_pinned_tools_do_not_depend_on_login_shell_path_reset(self):
        with patch.dict(os.environ, {"BENCH_TOOL_PATH": "/toolchain/bin:/usr/bin"}):
            args = bench.runner.command("codex", "prompt", "answer", "model", "medium")
            env = bench.runner.isolated_env("claude")
        self.assertIn("allow_login_shell=false", args)
        self.assertIn('shell_environment_policy.set.PATH="/toolchain/bin:/usr/bin"', args)
        self.assertEqual(env["PATH"], "/toolchain/bin:/usr/bin")
        self.assertIn("read-only", args)

    def test_flow_report_shows_unavailable_output_usage(self):
        with tempfile.TemporaryDirectory() as scratch:
            work = pathlib.Path(scratch)
            folder = work / "flows" / "setting" / "probe"
            folder.mkdir(parents=True)
            record = {"rounds": [{"author": {"seconds": 1}, "review": {"verdict": "accept"}, "stale_neighbours": []}],
                      "first_pass_accepted": True, "accepted": True, "repairs": 0, "first_pass_material": 0,
                      "sources_unchanged": True, "author_seconds": 1}
            (folder / "run1.json").write_text(json.dumps(record))
            with contextlib.redirect_stdout(io.StringIO()):
                bench.report(types.SimpleNamespace(work=work))
            output = (work / "report.md").read_text()
            self.assertIn("Total output tokens", output)
            self.assertIsNone(json.loads((work / "report.json").read_text())["flows"][0]["output_tokens"])


if __name__ == "__main__":
    unittest.main()
