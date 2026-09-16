# Installed-tool regression checks

These checks cover deterministic helpers shipped to consumers. They do not measure model token usage or replace semantic answer review.

Run the full distribution eval directories with `unittest discover` using the locked quality environment described in `kernel/90-Meta/code-quality.md`. Relevant focused commands:

```sh
python -B -m unittest discover -s evals/tooling
python -B -m unittest discover -s evals/helper-integrity
python -B kernel/90-Meta/test_instance.py
python -B -m unittest discover -s evals/sync -p 'test_next_action.py'
```

The quality-gate regressions require a complete Bandit JSON report. A zero process exit code with scanner errors is rejected. Scanner selection tests use temporary Git repositories, including dirty tracked files, untracked/ignored artifacts, symlinks and whitespace in filenames. Instance tests preserve supported scalar syntax independently of optional YAML packages. Synchronization tests preserve next-action ordering and existing integrity gates.
