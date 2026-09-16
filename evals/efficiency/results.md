# Routing efficiency comparison — 2026-09-16

## Decision

Retain the 0.10.2 kernel. This controlled experiment does not justify a universal routing change, removing evidence/review rules, or claiming optimal token consumption. Known-file direct access and selective delegation remain sensible defaults. No kernel or consumer vault was modified; all new material is distribution-only evaluation code and evidence. The separate user guide is outside this task.

The user subsequently accepted retaining 0.10.2. The [session closure](../session-closure.md) records that agreement and separates actual remaining work from optional follow-up.

## Design and evidence

Eight synthetic Norte Logistics cases, two strategies and three repetitions: 36 main outcomes and 12 predeclared confirmation outcomes. Every task phase explicitly requested Sol/medium; effective configuration is unexposed. The experiment compares the installed routing with strategy hints, under a common fixed orchestrator. It is not an unrestricted native-agent baseline. The two-repository alternative uses two independent CLI workers, synthesis and review; it does not test autonomous or cheaper-model delegation.

Both arms include independent evidence review, plus one repair and recheck when rejected. This makes simple tasks more expensive than their permitted author-only review path, equally in both arms. Fixture setup is outside task time. Each independent phase's reported usage is counted once. Cached input is a subset of total input, not additional usage. Failed attempts remain visible.

Sources, prompts, runner and fixture hashes are retained in [observed main results](results/observed-main.json). [Registered criteria](results/criteria-registered.md) predate the accepted campaign. [Decisions before confirmation](decision-before-confirmation.md) were frozen before the reserved cases ran. The [paired main comparison](results/comparison-main.json) keeps every repetition, phase cost, rejection, cost per fully accepted outcome and missing-evidence exclusion.

## Main outcomes

Independent answer review accepts 34/36: baseline 18/18, alternatives 16/18. The two semantic rejections are delegated syntheses whose domain conclusions are correct but whose unnecessary references to workers and an unverified extra finding impair standalone clarity. No critical false operational or closure claim was identified. Runtime reviewers accepted these two responses, so runtime acceptance alone is not proof of quality.

One otherwise semantically accepted alternative has an unobservable reviewer preflight: resolver exit 0 is captured, but its output is empty. It remains unknown, not a pass or an inferred access violation. Fully verified main outcomes are therefore baseline 18/18 and alternatives 15/18. The delegation case has no fully accepted pair and is excluded from quality-equivalent paired estimates.

The table reports the median of the **three paired percentage changes**, alternative relative to baseline, for fully accepted pairs. These are not percentages formed by dividing unrelated arm medians. Positive means more consumption/time.

| Main case and alternative | Input tokens, including cache | Uncached input | Output tokens | Task wall time |
| --- | ---: | ---: | ---: | ---: |
| Known file: forced graph lookup | +18.5% | -0.3% | +24.1% | +19.0% |
| Dependencies: directed text search instead of graph | +2.1% | -9.8% | -6.9% | -10.9% |
| Prior investigation: explicit reuse hint | +13.1% | -2.5% | +2.3% | -4.9% |
| Ambiguous question: focused resolution hint | -28.6% | -6.8% | -32.0% | -41.2% |
| False premise: explicit premise-checking hint | -7.1% | +15.7% | -18.6% | -17.1% |

Most directions are not uniform across repetitions. In particular, dependency text search increases input in two pairs and reduces it in one; there is no stable overall input saving. Both reuse arms follow the same official load/read route: this tests the additional hint, not reuse versus repeating an investigation. The false-premise hint lowers total input in all repetitions but raises uncached input in two, so lower total tokens does not establish lower monetary cost.

Delegation's three observed attempts use **79.8–108.2% more input**, **36.4–120.4% more uncached input**, **86.4–115.5% more output**, and **24.4–38.0% more task time**. These are descriptive costs of the attempted strategy, including the failed/unknown outcomes; they are not a quality-equivalent accepted-pair estimate. The result applies to this tiny two-note task, not larger independent investigations or lighter models.

## Confirmation and final totals

Both confirmation cases pass all 12 semantic reviews and all 12 trace audits. Questions, hints and runtime orchestration were unchanged after the frozen decision. [Confirmation comparisons](results/comparison-confirmation.json), [answers and measured phases](results/observed-confirmation.json), [blind review](results/blind-review-confirmation.json) and [trace audit](results/trace-audit-confirmation.md) retain the evidence.

| Confirmation alternative | Paired median input change | Uncached input | Output | Task wall time |
| --- | ---: | ---: | ---: | ---: |
| Known file: forced graph lookup | +13.5% | +12.8% | -4.0% | +7.6% |
| Topic producer/consumer: text search instead of graph | -5.9% | +2.0% | +2.4% | -7.9% |

Known-file graph input grows in two of three confirmation pairs and falls in the third, when the baseline takes an additional inventory detour. This supports retaining direct access as the default, not a claim that it always wins. Text search again trades metrics rather than establishing a universal improvement: two pairs use more uncached input/output, while one saves substantially. Do not replace graph discovery generally.

Across 48 outcomes, semantic acceptance is **46/48**: baseline **24/24**, alternatives **22/24**. Including the main trace gap, fully verified acceptance is **45/48**: baseline **24/24**, alternatives **21/24**. The two clarity rejections and the separate missing-evidence result remain recorded. There is no selected alternative awaiting propagation.

All **104 measured CLI phases** report usage; every phase-to-outcome total reconciles. Accepted campaigns consume **16,563,154 input tokens**, of which **14,326,144 cached** and **2,237,010 uncached**, plus **85,036 output tokens**. These totals include runtime reviewers, child workers and the one repair/recheck cycle, not just final answers. Nominal input tokens are not a monetary bill.

Separate setup/smoke attempts report at least **7,539,477 input**, **6,623,872 cached input**, and **39,719 output tokens**; the interrupted attempt may have unreported partial usage. These costs are not hidden in favorable strategy comparisons. Native design/audit/blind-review agent usage is unmeasured and is not included in these task-runner totals.

Repository checks: 35 bootstrap tests, 14 instance tests and 5 accounting/review-gate tests pass; Ruff and Bandit pass. Original consumer vaults and the unrelated user-guide work were not modified. The evaluation and accepted decision are recorded in a local eval-only commit. No release, consumer propagation or push was performed for this evaluation.

## Instrument corrections and limitations

The initial read-only sandbox denied the investigation loader's transient lock. System Python also emitted cache-write diagnostics. Those attempts were stopped and excluded, not counted as an efficiency win. The corrected fixture permits official temporary locks, uses the verified interpreter and checks unchanged final file/symlink fingerprints. Initial attempts and corrected smokes are separately accounted in [setup attempts](results/setup-attempts.json); interrupted phases may have additional unreported usage.

The early pilot also exposed identical routes in some arms. Hints were revised before the accepted campaign to create actual graph-versus-direct and graph-versus-text contrasts. Main trace audits confirm those contrasts in all repetitions, successful investigation loads, no persistent changes and no nested workers. See [trace audit](results/trace-audit-main.md).

Blind reviewers initially lacked several cited policy documents and a cited deterministic result. Supplying the actual evidence reversed unsupported rejections without changing answers or the rubric; initial scores and reasons remain in [review history](results/blind-review-main.json). The two remaining clarity failures are qualitative judgments, not fabricated domain facts. Answers that mention workers inherently reveal some execution context despite opaque IDs and withheld arm/cost labels.

Three repetitions and a small synthetic fixture do not establish statistical equivalence, production reliability or an absolute optimum. Shared cache, host scheduling and concurrent pairs limit causal timing claims. Main concurrency is six pairs; confirmation concurrency is two, so compare arms within each case, not absolute times between campaigns. Installed host skills/plugins contribute context; totals cannot all be attributed to this framework. Pricing and actual billing are unavailable, and independent experiment-design/blind-review session usage is outside measured task totals.
