# Source-access probes

Run from the distribution with an explicit clean distribution checkout and a new temporary output directory:

```sh
python3 -B evals/source-access/run.py --distribution /path/to/clean/distribution --output /path/to/new/temporary/output --case known
```

Inspect the smoke worker's complete trace and preservation summary before launching the other cases. Repeat `--case` to select cases; omit it for all eight. Each run invokes real Codex workers, consumes model usage and requires local Codex authentication/runtime access. Workers use synthetic Git remotes, an offline Obsidian stub and read-only task prompts. The coordinator creates temporary fixtures and stores evidence outside worker directories.

`criteria.md` is evaluator-only. Judge completed tool events and filesystem fingerprints; an answer asserting compliance or a process exit code is insufficient. `results.md` distinguishes source-selection success from routing/recovery/clarity deviations.

See [closure.md](closure.md) for final acceptance and the complete correction history.
