# Portable code quality baseline

This vault ships one provider-independent Python gate. Ruff catches syntax
errors, undefined or unused names, malformed imports and other core lint
failures. Bandit rejects every medium/high security finding and any runtime
`assert` in maintained code. Tests receive the Ruff profile and remain outside
Bandit. Distribution evals retain Ruff correctness checks while their deliberate
compact fixture style remains outside the lint baseline.

Ruff discovers Python files and loads `ruff.toml`; Bandit loads `.bandit`. The
small `check-code-quality.py` entrypoint only invokes those tools so the same
command works in the distribution and in an installed vault. Bandit reports
scanner errors separately from findings; the gate rejects incomplete or invalid
reports even when the scanner exits successfully. The configuration
ignores local/generated state and virtual environments. It does not download
remote rules, apply automatic fixes, compare against a baseline, or encode a
company-specific threshold. A reported issue must be fixed or reviewed in its
actual source/control/sink context; do not suppress a rule only to make the gate
green.

`requirements-ci.in` declares the direct tools. `requirements-ci.txt` is a
universal, wheel-only lock with hashes for every transitive dependency. Update
the short input and regenerate the lock with the `uv pip compile` command stored
in its header; running the gate requires only Python and the locked environment.

Create an isolated environment and run it from an installed vault:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install -r 90-Meta/requirements-ci.txt
.venv/bin/python -B 90-Meta/check-code-quality.py --root .
```

In the distribution checkout, use the same command with the managed source
paths:

```bash
python3 -m venv .venv
.venv/bin/python -m pip install -r kernel/90-Meta/requirements-ci.txt
.venv/bin/python -B kernel/90-Meta/check-code-quality.py --root .
```

Run the repository's functional tests after this static gate. Parser, boundary,
authorization and persistence behavior require executable tests; static
analysis cannot prove those contracts by itself.
