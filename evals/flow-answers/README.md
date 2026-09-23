# Flow-answer behavior check

This offline synthetic case tests the first process explanation, the later diagram and a production-validation follow-up in one conversation. It separates a component dependency from a conditional call, a rejected request from a store procedure, an intended sequence from an enforced state machine, and a burned trace from physical exit. It contains no production data.

Check the deterministic fixture:

```sh
python3 -B evals/flow-answers/test_fixture.py
```

On macOS, run the agent with an OS-enforced boundary and complete event capture:

```sh
python3 -B evals/flow-answers/run_macos.py --root /private/tmp/flow-answer-<unique-id>
```

The runner prepares a fresh consumer and source checkout, stages the local CLI and its authentication in a private temporary directory, and removes authentication after the run. Its `sandbox-exec` preflight verifies that the worker can read its fixture while the evaluator, distribution checkout, host Codex home and other temporary runs are unreadable. The same profile applies to the agent and its reviewers. It passes the later prompts through standard input only after the preceding turn ends. The evaluator directory retains the per-turn JSON events, the full main and reviewer session traces, the profile and source hashes. A CLI error event fails the run even if the agent produces a final message. Keep this output private; raw traces can include environment paths and full source reads.

Inspect the first answer and first diagram independently against `evaluation.json`, the source revision, and the reviewer traces. A later correction does not erase an initial failure. A self-reported review does not prove independent acceptance; check the reviewer session and its timing. Keep model, effort, permission profile and prompt/fixture versions fixed when comparing kernel revisions. Do not report a behavioral pass from fixture tests, a preflight alone, or a CLI session that failed to read its tools.

The fixture test executes the synthetic code paths and checks packaging separation. It does not run an agent or establish compliance with the kernel instructions. The isolated run verifies access boundaries and records behavior; it still requires an evidence-based human or independent evaluator assessment before a pass is claimed.

The bounded candidate run and its limits are recorded in [results.md](results.md).
