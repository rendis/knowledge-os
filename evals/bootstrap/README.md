# Bootstrap evals

These checks are **not** installed into a cell vault.

```bash
python3 -B kernel/90-Meta/test_instance.py
python3 -B evals/bootstrap/test_bootstrap.py
```

Adversarial rounds use `criteria.md` plus the persona briefs. Ledgers go in `evals/bootstrap/ledgers/` (gitignored results may also land in `evals/bootstrap/results/`).
