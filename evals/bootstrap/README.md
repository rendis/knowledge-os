# Bootstrap evals

These checks are **not** installed into a cell vault.

```bash
python3 -B kernel/90-Meta/test_instance.py
python3 -B evals/bootstrap/test_bootstrap.py
python3 -B evals/bootstrap/test_interactive_onboarding.py
python3 -B evals/bootstrap/test_cell_capabilities.py
python3 -B evals/bootstrap/test_investigation_transactions.py
python3 -B evals/bootstrap/test_integrity.py
```


When changing development-package content requirements, also run the bounded
[handoff sufficiency behavioral regression](handoff-sufficiency.md). It checks
source-derived questions and recipient understanding; the Python integrity
tests do not establish semantic completeness.

When changing personal-instruction routing, run the blind
[personal instructions behavior trials](personal-instructions.md). The bootstrap
suite checks installation and Git boundaries; it does not prove that an agent
chooses correctly when the optional file is absent or present.
