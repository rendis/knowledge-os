# Measured routing comparison

This evaluation compares explicit strategy hints with routing from the frozen 0.10.2 distribution. It does not modify the kernel or install anything into real consumer vaults. Read [criteria](criteria.md) before interpreting measurements. The completed campaign and decision are in [results](results.md); the accepted cross-session decision and remaining work are in [session closure](../session-closure.md).

```sh
python3 -B evals/efficiency/run.py \
  --cases evals/efficiency/cases.json \
  --distribution /path/to/clean-0.10.2 \
  --output /private/tmp/new-efficiency-campaign \
  --repetitions 3 --concurrency 6 --workers 2 \
  --only known-file dependencies two-repositories reuse-investigation ambiguous false-premise
python3 -B evals/efficiency/summarize.py /private/tmp/new-efficiency-campaign \
  --output /private/tmp/efficiency-ledger.json
```

After scoring the six main cases and freezing conclusions, use a new output directory with `--only holdout-known holdout-impact` and the same repetition count. Do not adjust the hints against the confirmation answers.

Run the harness with a Python executable that works within the sandbox; workers receive that same interpreter through their fixture PATH. The recorded macOS campaign uses Homebrew Python to avoid the system launcher's cache-write diagnostics.

First run a one-repetition `--only two-repositories` smoke in a separate output directory. The runner refuses a nonempty output directory, retains failed attempts, captures each independent Sol/medium CLI phase and includes independent review and bounded repairs in task totals. All task phases use workspace-write sandboxing in disposable fixtures, with content changes forbidden and temporary official read locks permitted; the disconnected Obsidian stub prevents real app discovery. These controls do not prove real Obsidian integration or a host-wide read isolation guarantee.

Raw outputs stay in the selected temporary directory. Published results must normalize fixture paths and exclude machine-specific command paths. Retain the case questions, source revision, fixture/runner hashes, individual phase usage, final answers, blind scoring and aggregation method. `ok` in runner output combines process/preservation checks and runtime reviewer acceptance; final semantic scoring is a separate requirement.

Input includes its cached subset. Never add cached tokens again. Cost in money is unmeasured unless the exact billing basis is available. Effective model/effort are unknown when only requested CLI configuration is exposed. Fixed worker decomposition is not a test of autonomous delegation.

After blind scoring, join opaque answer IDs to outcome IDs privately and add the independent trace audit's `trace_verified` value (`true`, `false` or unknown). `compare.py --ledger <ledger.json> --scores <scores.json> --output <comparison.json>` includes failed expenditure and excludes pairs without semantic acceptance and verified traces. Missing trace verification is unknown, never an implicit pass. Published initial/revised scores explain evaluator-source corrections.
