# Source-access correction closure — 2026-09-16

**Accepted: 8/8 scoped scenarios with observable evidence.** The final independent Sol/medium review found no unresolved failure in the tested criteria. Changes remain local; no new version or consumer propagation is claimed.

## Final change

Three agent documents were adjusted, preserving authorization and review gates:

- `kernel/AGENTS.md`: resolve the vault before domain-note reads; allow instruction bootstrap; retain the selected owner; explicitly load the source-resolution reference before checkout selection.
- `kernel/90-Meta/vault-resolution.md`: describe configuration/checkout recovery concretely. Configuration prohibitions keep source reads blocked; the next question asks for the required configuration decision/authorization rather than permission to bypass identity.
- `kernel/90-Meta/response-quality.md`: retain the decisive source reference even for a one-sentence answer.

## Evidence by final case

| Case | Accepted trace and decisive events |
| --- | --- |
| known | round6: vault2 → note4 → lookup5 ok → payload6 → citation7 |
| direct | round6: vault2 → lookup6 ok → payload7 → citation8; no graph discovery |
| ambiguous | round6: vault2 → reference3 → note5 → lookup6 ambiguous → configure8 → configuration choice/authorization9; no payload |
| missing-config | round6: reference3 → vault4 unavailable → configure6 → lookup7 not_found → bounded configuration authorization8; no payload |
| outside-root | round6: successful vault resolution before note; not_found within configured roots; no sibling search or payload |
| wrong-remote | round6: resolved vault and expected remote before lookup; mismatch does not authorize payload access |
| two-repositories | round6: both successful lookup outputs4/5 observable before payload access; corresponding citations |
| overlapping-roots | final-overlap: vault2 → reference3 → note5 → lookup6 ambiguous → configure7 → root decision8, with change authorization preserved; no payload |

All nine final-policy worker fixtures matched the same three candidate instruction files byte-for-byte. Their preservation summaries contain `changed: []`. For acceptance, use seven fully observable round6 cases plus the focused overlap rerun: round6 overlap behaved safely but its CLI event omitted locator output, so it was not sufficient evidence alone.

## Iteration history

44 worker executions across eight scenario types, including the original baseline; these are not 44 distinct scenarios. All workers explicitly requested Sol/medium. Effective-model telemetry was not exposed. Reviewers also used Sol/medium.

| Run | Executions | Outcome |
| --- | ---: | --- |
| Baseline | 8 | Source binding passes; domain-note ordering, recovery, citation and owner-continuity deviations remain. |
| Round2 | 8 | Most original deviations fixed; overlap recovery still proposes path confirmation without configuration repair. One two-repository lookup output is not observable. |
| Round3 | 8 | Source binding and note ordering pass; two agents still propose direct-read exceptions. |
| Round4 focused | 3 | Configuration-focused recovery passes all three cases. |
| Round5 | 8 | Original failures closed; ambiguous case skips required reference/owner loading and guesses a helper path before correcting it. |
| Round6 | 8 | Behavior and required reference/recovery loading pass; one overlap locator output missing from capture. |
| Final overlap | 1 | Same policy; missing evidence captured, recovery and preservation accepted. |

Earlier failures remain in the evidence; no failure was relabeled as a pass. Policy outputs in retained event extracts may be abbreviated with hashes; final decisive locator outputs are retained. Full originals remain in the corresponding `/private/tmp/dv-source-access-*` directories.

## Additional verification and limits

Final candidate bootstrap: 35/35. Instance tests: 14/14. Eval runner Ruff and diff whitespace checks pass. No code-tool implementation, consumer vault, real source repository or existing user-guide work was changed by this correction. The candidate excludes that unrelated user-guide work.

This closes the observed source-access, recovery, citation and ownership defects for these scenarios. It does not guarantee every future model response, establish statistical reliability, demonstrate token savings, or validate production/real Obsidian behavior. Author-only answer-review execution remains unobservable in CLI events; it was not inferred from policy reads.
