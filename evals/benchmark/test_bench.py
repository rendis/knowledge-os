"""Synthetic benchmark protocol regressions; no model sessions or cell fixtures."""
import json
import concurrent.futures as cf
import contextlib
import io
import os
import pathlib
import subprocess
import tempfile
import types
import unittest
from unittest.mock import patch

import bench


PENDING_REVIEW = "no review recorded; run `sync review` after the reviewer's verdict"


class GateEvaluationTests(unittest.TestCase):
    def evaluate(self, problems=None, returncode=None, **fields):
        problems = [PENDING_REVIEW] if problems is None else problems
        payload = {"ok": not problems, "branch": "sync/probe", "changed": ["investigations/probe/investigation.md"],
                   "note_gates": [], "case_gates": [], "stale_neighbours": [], "new_structural_issues": [],
                   "copied_paragraphs": [], "problems": problems, **fields}
        code = (0 if payload["ok"] else 1) if returncode is None else returncode
        result = subprocess.CompletedProcess([], code, json.dumps(payload), "")
        reviewer = {"answer": '{"verdict":"accept","findings":[]}', "seconds": 0, "usage": {}, "returncode": 0}
        with patch.object(bench, "sh", side_effect=["sync/probe", "investigations/probe/investigation.md", ""]), \
                patch.object(bench, "cli", return_value="synthetic-kos"), \
                patch.object(bench.subprocess, "run", return_value=result), \
                patch.object(bench.runner, "execute", return_value=reviewer):
            return bench.evaluate(pathlib.Path("synthetic-vault"), "main", "synthetic-review", 30)

    def test_pending_review_is_replaced_by_the_fixed_reviewer(self):
        self.assertTrue(self.evaluate()["accepted"])

    def test_a_failed_case_cannot_be_accepted(self):
        result = self.evaluate(["case gate failed: investigation.md, 1 error(s)", PENDING_REVIEW],
                               case_gates=[{"case": "investigation.md", "ok": False}])
        self.assertFalse(result["gates_ok"])
        self.assertFalse(result["accepted"])

    def test_all_other_verification_problems_block_acceptance(self):
        for problem in ("a note copies a repository paragraph", "discovery state: invalid JSON", "unknown gate failure"):
            with self.subTest(problem=problem):
                self.assertFalse(self.evaluate([problem, PENDING_REVIEW])["accepted"])

    def test_an_unexpected_exit_code_cannot_pass_with_valid_json(self):
        self.assertFalse(self.evaluate(returncode=2)["accepted"])

    def test_a_missing_gate_field_cannot_pass(self):
        self.assertFalse(self.evaluate(case_gates=None)["accepted"])

    def test_note_gates_and_stale_neighbours_still_block(self):
        self.assertFalse(self.evaluate(note_gates=[{"note": "a.md", "ok": False}])["accepted"])
        self.assertFalse(self.evaluate(stale_neighbours=[{"note": "a.md"}])["accepted"])

    def test_a_verified_branch_passes(self):
        self.assertTrue(self.evaluate(problems=[])["accepted"])


class AccountingTests(unittest.TestCase):
    def test_unborn_source_integrity_keeps_index_worktree_and_head_changes(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            subprocess.run(["git", "init", "-q", "-b", "main", str(root)], check=True)
            original = bench.sh
            def command(args, **kwargs):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                return original(args, **kwargs)
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command):
                empty = bench.source_state("synthetic-vault")
                source = root / "source"
                source.write_text("first")
                untracked = bench.source_state("synthetic-vault")
                self.assertNotEqual(empty, untracked)
                subprocess.run(["git", "-C", str(root), "add", "source"], check=True)
                staged = bench.source_state("synthetic-vault")
                self.assertNotEqual(untracked, staged)
                source.write_text("second")
                unstaged = bench.source_state("synthetic-vault")
                self.assertNotEqual(staged, unstaged)
                subprocess.run(["git", "-C", str(root), "add", "source"], check=True)
                source.write_text("first")
                index_changed = bench.source_state("synthetic-vault")
                self.assertNotEqual(staged, index_changed)
                subprocess.run(["git", "-C", str(root), "symbolic-ref", "HEAD", "refs/heads/other"], check=True)
                self.assertNotEqual(index_changed, bench.source_state("synthetic-vault"))

    def test_invalid_detached_head_is_not_treated_as_an_unborn_branch(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            (root / ".git/HEAD").write_text("0" * 40 + "\n")
            original = bench.sh
            def command(args, **kwargs):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                return original(args, **kwargs)
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command):
                with self.assertRaisesRegex(RuntimeError, "invalid source HEAD"):
                    bench.source_state("synthetic-vault")

    def test_source_reference_changes_invalidate_a_frozen_flow(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            (root / "source").write_text("fixture")
            subprocess.run(["git", "-C", str(root), "add", "source"], check=True)
            identity = ["git", "-C", str(root), "-c", "user.name=test", "-c", "user.email=test@invalid"]
            subprocess.run([*identity, "commit", "-qm", "fixture"], check=True)
            other = subprocess.check_output([*identity, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "other"], text=True).strip()
            original = bench.sh
            def command(args, **kwargs):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                return original(args, **kwargs)
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command):
                before = bench.source_state("synthetic-vault")
                subprocess.run(["git", "-C", str(root), "update-ref", "refs/remotes/origin/main", other], check=True)
                self.assertNotEqual(before, bench.source_state("synthetic-vault"))

    def test_failed_codex_run_cannot_reuse_a_previous_answer(self):
        with tempfile.TemporaryDirectory() as scratch:
            answer = pathlib.Path(scratch) / "answer.txt"
            answer.write_text("old successful answer")
            failure = subprocess.CompletedProcess([], 1, "", "failure")
            with patch.object(bench.runner, "isolated_env", return_value={}), patch.object(bench.runner.subprocess, "run", return_value=failure):
                record = bench.runner.execute("codex", "prompt", scratch, "model", "effort", answer)
            self.assertEqual(record["returncode"], 1)
            self.assertNotEqual(record["answer"], "old successful answer")
            question = {"question": "q", "expected": ["fact"], "must_not": []}
            with patch.object(bench.judge, "execute") as execute:
                grade = bench.judge.grade(question, {"answer": "fact", "returncode": 1})
            execute.assert_not_called()
            self.assertEqual(grade["score"], 0)

    def test_judge_cli_does_not_report_unknown_token_counts_as_zero(self):
        with tempfile.TemporaryDirectory() as scratch:
            work = pathlib.Path(scratch)
            questions = work / "questions.json"
            questions.write_text(json.dumps([{"id": "Q1", "expected": ["fact"]}]))
            answers = work / "answers"
            answers.mkdir()
            record = {"id": "Q1", "seconds": 1, "answer": "fact"}
            (answers / "Q1.json").write_text(json.dumps(record))
            grade = {"score": 1, "verdict": "correct", "points": [1], "losses": [None]}
            output = io.StringIO()
            with patch.object(bench.judge, "grade", return_value=grade), \
                    patch.object(bench.judge.sys, "argv", ["judge.py", "--questions", str(questions), "--answers", str(answers)]), \
                    contextlib.redirect_stdout(output):
                bench.judge.main()
            self.assertIn("input_tokens=unavailable output_tokens=unavailable", output.getvalue())

    def test_tracked_binary_integrity_does_not_capture_the_patch_in_memory(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            artifact = root / "artifact.bin"
            artifact.write_bytes(b"\0" * (2 * 1024 * 1024))
            subprocess.run(["git", "-C", str(root), "add", "artifact.bin"], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test", "-c", "user.email=test@invalid",
                            "commit", "-qm", "fixture"], check=True)
            with artifact.open("r+b") as stream:
                stream.write(b"a")
            original = bench.sh
            def command(args, **kwargs):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                if "diff" in args and "--binary" in args:
                    raise AssertionError("must stream the tracked diff")
                return original(args, **kwargs)
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command):
                before = bench.source_state("synthetic-vault")
                with artifact.open("r+b") as stream:
                    stream.seek(-1, os.SEEK_END)
                    stream.write(b"b")
                self.assertNotEqual(before, bench.source_state("synthetic-vault"))

    def test_source_integrity_includes_ignored_untracked_bytes(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            (root / ".gitignore").write_text(".env\n")
            subprocess.run(["git", "-C", str(root), "add", ".gitignore"], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=test", "-c", "user.email=test@invalid",
                            "commit", "-qm", "fixture"], check=True)
            original = bench.sh
            def command(args, **kwargs):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                return original(args, **kwargs)
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command):
                before = bench.source_state("synthetic-vault")
                ignored = root / ".env"
                ignored.write_text("first fixture value")
                created = bench.source_state("synthetic-vault")
                self.assertNotEqual(before, created)
                ignored.write_text("second fixture value")
                edited = bench.source_state("synthetic-vault")
                self.assertNotEqual(created, edited)
                ignored.unlink()
                ignored.symlink_to("second fixture value")
                self.assertNotEqual(edited, bench.source_state("synthetic-vault"))
                (root / " leading-file").write_text("fixture")
                self.assertNotEqual(edited, bench.source_state("synthetic-vault"))

    def test_missing_claude_and_cursor_usage_is_unavailable(self):
        for harness in ("claude", "cursor"):
            for payload in ({"result": "ok"}, {"result": "ok", "usage": {}}, {"result": "ok", "usage": {"input_tokens": 10, "inputTokens": 10}}):
                with self.subTest(harness=harness, payload=payload), tempfile.TemporaryDirectory() as scratch:
                    result = subprocess.CompletedProcess([], 0, json.dumps(payload), "")
                    with patch.object(bench.runner, "isolated_env", return_value={}), patch.object(bench.runner.subprocess, "run", return_value=result):
                        rec = bench.runner.execute(harness, "prompt", scratch, "model", "effort", pathlib.Path(scratch)/"answer")
                    self.assertIsNone(rec["usage"])

    def test_complete_usage_keeps_explicit_zero_cache_counts(self):
        model_usage = {"inputTokens": 10, "cacheReadInputTokens": 0, "cacheCreationInputTokens": 0, "outputTokens": 1}
        cases = [("claude", {"modelUsage": {"model": model_usage}}),
                 ("claude", {"usage": {"input_tokens": 10, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0, "output_tokens": 1}}),
                 ("cursor", {"usage": {"inputTokens": 10, "cacheReadTokens": 0, "cacheWriteTokens": 0, "outputTokens": 1}})]
        for harness, payload in cases:
            with self.subTest(harness=harness, payload=payload):
                self.assertEqual(bench.runner.normalize_usage(harness, payload), {"input_total": 10, "input_cached": 0, "output": 1})

    def test_incomplete_codex_turn_usage_is_unavailable(self):
        complete = {"type": "turn.completed", "usage": {"input_tokens": 10, "cached_input_tokens": 0, "output_tokens": 1}}
        for events in ([{"type": "turn.completed"}, complete], [complete, {"type": "turn.completed", "usage": None}],
                       [{"type": "turn.completed", "usage": {"input_tokens": 10}}], []):
            with self.subTest(events=events), tempfile.TemporaryDirectory() as scratch:
                stdout = "\n".join(json.dumps(e) for e in events)
                result = subprocess.CompletedProcess([], 0, stdout, "")
                with patch.object(bench.runner, "isolated_env", return_value={}), patch.object(bench.runner.subprocess, "run", return_value=result):
                    record = bench.runner.execute("codex", "prompt", scratch, "model", "effort", pathlib.Path(scratch)/"answer")
                self.assertIsNone(record["usage"])
        with tempfile.TemporaryDirectory() as scratch:
            result = subprocess.CompletedProcess([], 0, "\n".join([json.dumps(complete)] * 2), "")
            with patch.object(bench.runner, "isolated_env", return_value={}), patch.object(bench.runner.subprocess, "run", return_value=result):
                record = bench.runner.execute("codex", "prompt", scratch, "model", "effort", pathlib.Path(scratch)/"answer")
            self.assertEqual(record["usage"]["input_total"], 20)

    def test_oversized_answers_are_not_sent_to_the_judge(self):
        question = {"question": "q", "expected": ["fact"], "must_not": []}
        with patch.object(bench.judge, "execute") as execute:
            grade = bench.judge.grade(question, {"answer": "x" * (1024 * 1024)})
        execute.assert_not_called()
        self.assertEqual(grade["verdict"], "error")
        self.assertEqual(grade["score"], 0)

    def test_source_integrity_streams_files_and_rejects_special_files(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = pathlib.Path(scratch)/"repo"
            (root/".git").mkdir(parents=True)
            artifact = root/"artifact"
            artifact.write_bytes(b"a" * (2 * 1024 * 1024))
            def command(args, **_):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(root)}]}})
                if "rev-parse" in args:
                    return "commit"
                return "artifact\0" if "ls-files" in args else ""
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command), \
                    patch.object(bench, "stream_git_diff"), \
                    patch.object(pathlib.Path, "read_bytes", side_effect=AssertionError("must stream source bytes")):
                before = bench.source_state("synthetic-vault")
                with artifact.open("r+b") as stream:
                    stream.seek(-1, os.SEEK_END)
                    stream.write(b"b")
                self.assertNotEqual(before, bench.source_state("synthetic-vault"))
                if hasattr(os, "mkfifo"):
                    artifact.unlink()
                    os.mkfifo(artifact)
                    with patch.object(pathlib.Path, "open", side_effect=AssertionError("must not open a FIFO")):
                        with self.assertRaisesRegex(RuntimeError, "unsupported source entry"):
                            bench.source_state("synthetic-vault")

    def test_partial_grading_keeps_judge_accounting_unavailable(self):
        with tempfile.TemporaryDirectory() as scratch:
            work = pathlib.Path(scratch)
            run = work / "qa" / "setting" / "run1"
            run.mkdir(parents=True)
            for name in ("Q1", "Q2"):
                (run / (name+".json")).write_text(json.dumps({"id": name, "vault": "probe", "returncode": 0, "seconds": 1}))
            (run / "grades.json").write_text(json.dumps({"Q1": {"score": 1, "verdict": "correct", "judge_seconds": 10, "judge_cost_usd": 0.1}}))
            with contextlib.redirect_stdout(io.StringIO()):
                bench.report(types.SimpleNamespace(work=work))
            row = json.loads((work / "report.json").read_text())["qa"][0]
            self.assertEqual(row["grading_errors_per_run"], 1)
            self.assertIsNone(row["judge_seconds"])
            self.assertIsNone(row["judge_cost_usd"])

    def test_source_changes_block_an_otherwise_accepted_flow(self):
        accepted = {"accepted": True, "gate_problems": []}
        bench.enforce_source_integrity(accepted, {"repo": "before"}, {"repo": "after"})
        self.assertFalse(accepted["accepted"])
        self.assertFalse(accepted["sources_unchanged"])
        self.assertTrue(accepted["gate_problems"])
        with tempfile.TemporaryDirectory() as scratch:
            work = pathlib.Path(scratch)
            flow = work / "flows" / "setting" / "probe"
            flow.mkdir(parents=True)
            # Old artifacts may still carry accepted=true beside sources_unchanged=false.
            rec = {"rounds": [{"author": {"seconds": 1}, "review": {"verdict": "accept"}, "stale_neighbours": []}],
                   "first_pass_accepted": True, "accepted": True, "repairs": 0, "first_pass_material": 0,
                   "sources_unchanged": False, "author_seconds": 1}
            (flow / "run1.json").write_text(json.dumps(rec))
            with contextlib.redirect_stdout(io.StringIO()):
                bench.report(types.SimpleNamespace(work=work))
            result = json.loads((work / "report.json").read_text())["flows"][0]
            self.assertEqual(result["accepted"], 0)
            self.assertEqual(result["first_pass_accepted"], 0)

    def test_unsupported_claims_remain_visible_despite_full_score(self):
        rec = {"id": "Q1", "vault": "probe", "returncode": 0}
        grade = {"score": 1, "verdict": "correct", "points": [1], "losses": [None],
                 "violations": [], "unsupported_claims": ["Invented service"]}
        summary = bench.judge.summary([rec], {"Q1": grade})["by_vault"]["probe"]
        self.assertEqual(summary["score"], 1)
        self.assertEqual(summary["unsupported_claims"], 1)
        self.assertEqual(summary["clean_answers"], 0)

    def test_report_does_not_hide_missing_grades_or_execution_errors(self):
        with tempfile.TemporaryDirectory() as scratch:
            work = pathlib.Path(scratch)
            run = work / "qa" / "setting" / "run1"
            run.mkdir(parents=True)
            (run / "Q1.json").write_text(json.dumps({"id": "Q1", "vault": "probe", "returncode": 1, "seconds": 2}))
            with contextlib.redirect_stdout(io.StringIO()):
                bench.report(types.SimpleNamespace(work=work))
            result = json.loads((work / "report.json").read_text())["qa"][0]
            self.assertEqual(result["grading_errors_per_run"], 1)
            self.assertEqual(result["execution_errors_per_run"], 1)
            self.assertEqual(result["clean_answers_per_run"], 0)
            self.assertEqual(result["score_mean"], 0)
            self.assertIsNone(result["input_tokens"])
            self.assertIsNone(result["cost_usd"])

    def test_all_author_and_reviewer_rounds_are_counted(self):
        author = {"seconds": 10, "cost_usd": 0.5, "usage": {"input_total": 100, "output": 20}}
        reviewer = {"seconds": 7, "cost_usd": 0.2, "usage": {"input_total": 50, "output": 10}}
        total = bench.flow_usage({"rounds": [{"author": author, "review": reviewer}] * 2})
        self.assertEqual(total["total_agent_seconds"], 34)
        self.assertEqual(total["total_input_tokens"], 300)
        self.assertAlmostEqual(total["total_cost_usd"], 1.4)
        reviewer["cost_usd"] = None
        self.assertIsNone(bench.flow_usage({"rounds": [{"author": author, "review": reviewer}]})["total_cost_usd"])
        self.assertIsNone(bench.known_mean([0.5, None]))
        self.assertEqual(bench.judge.known_sum([0, 0]), 0)

    def test_invalid_judge_results_cannot_pass(self):
        question = {"question": "q", "expected": ["fact"], "must_not": []}
        invalid = {"points": [1, 1], "losses": [None, None], "violations": [], "unsupported_claims": [], "verdict": "correct"}
        result = {"answer": json.dumps(invalid), "seconds": 1, "returncode": 0, "usage": None}
        with patch.object(bench.judge, "execute", return_value=result) as execute:
            grade = bench.judge.grade(question, {"answer": "answer"})
        self.assertEqual(execute.call_count, 3)
        self.assertEqual(grade["verdict"], "error")
        self.assertEqual(grade["score"], 0)
        self.assertEqual(grade["judge_seconds"], 3)

    def test_parallel_harness_setup_reuses_the_same_auth_link(self):
        with tempfile.TemporaryDirectory() as scratch:
            real = pathlib.Path(scratch) / "real"
            isolated = pathlib.Path(scratch) / "isolated"
            real.mkdir()
            ((real / "auth.json").resolve()).write_text("{}")
            with patch.dict(os.environ, {"CODEX_HOME": str(real), "BENCH_CODEX_HOME": str(isolated)}):
                with cf.ThreadPoolExecutor(16) as executor:
                    list(executor.map(lambda _: bench.runner.isolated_env("codex"), range(64)))
            self.assertEqual((isolated / "auth.json").resolve(), (real / "auth.json").resolve())

    def test_source_state_includes_direct_roots_with_the_same_basename(self):
        with tempfile.TemporaryDirectory() as scratch:
            roots = [pathlib.Path(scratch) / group / "repo" for group in ("a", "b")]
            for root in roots:
                (root / ".git").mkdir(parents=True)
            def command(args, **_):
                if args[0] == "synthetic-kos":
                    return json.dumps({"source_context": {"roots": [{"path": str(r)} for r in roots]}})
                return "commit" if "rev-parse" in args else ""
            with patch.object(bench, "cli", return_value="synthetic-kos"), patch.object(bench, "sh", side_effect=command), \
                    patch.object(bench, "stream_git_diff"):
                state = bench.source_state("synthetic-vault")
            self.assertEqual(set(state), {str(r.resolve()) for r in roots})


if __name__ == "__main__":
    unittest.main()
