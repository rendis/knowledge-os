# Bootstrap evals

These checks are **not** installed into a cell vault.

```bash
python3 -B kernel/90-Meta/test_instance.py
python3 -B evals/bootstrap/test_bootstrap.py
python3 -B evals/bootstrap/test_interactive_onboarding.py
python3 -B evals/bootstrap/test_sync_tooling.py
python3 -B evals/bootstrap/test_cell_capabilities.py
python3 -B evals/bootstrap/test_investigation_transactions.py
python3 -B evals/bootstrap/test_integrity.py
python3 -B evals/sync/test_integrity.py
python3 -B evals/sync/test_noop_integrity.py
python3 -B evals/sync/test_sync_pipeline.py
python3 -B evals/sync/test_sync_semantics.py
python3 -B evals/sync/test_extraction_contract.py
python3 -B evals/sync/test_sync_correction.py
python3 -B evals/extraction-quality/test_fixture.py
python3 -B evals/sync/test_sync_state.py
python3 -B evals/sync/test_run_eval.py
```

The synchronization checks cover the gate/projection contract, durable run state, independent units, and recovery. Their frozen criteria and ledgers live in `evals/sync/`; bootstrap ledgers remain in `evals/bootstrap/ledgers/` (gitignored results may also land in `evals/bootstrap/results/`).

When changing development-package content requirements, also run the bounded
[handoff sufficiency behavioral regression](handoff-sufficiency.md). It checks
source-derived questions and recipient understanding; the Python integrity
tests do not establish semantic completeness.
