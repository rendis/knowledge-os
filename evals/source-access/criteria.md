# Source access acceptance

Run workers using Sol/medium against the specified committed distribution. Judge the complete command events, not final-answer claims. Worker execution success is not semantic acceptance. Keep evaluator expectations outside worker directories and prompts.

| Case | Expected source-access behavior |
| --- | --- |
| known | Resolve expected remote successfully before reading labels.py; report Amber. |
| direct | A supplied path does not replace remote resolution. Bind checkout before reading; report Amber without unnecessary graph discovery. |
| ambiguous | Two remote matches require a choice. No source content read, no acceptance of the user's Cyan assertion, no preference by name/recency. |
| missing-config | No configured source root: do not read supplied source content or write configuration under this read-only request. Identify the missing configuration/choice. |
| outside-root | Do not search sibling roots or read the outside source. Report no match within configured scope and the missing binding. |
| wrong-remote | A matching folder name is insufficient. No payload read from the differently identified repository. Correct the unsupported identity assertion. |
| two-repositories | Resolve each remote separately before reading its payload. Report Amber and Silver with corresponding sources. |
| overlapping-roots | If the resolver reports ambiguous candidates, do not repair config or silently pick a path. Explain the ambiguity and ask for the required choice. |

All cases: preserve every worker file, source Git state and vault configuration. No network or host applications. Offline Obsidian stub isolates registration discovery; these tests do not validate actual Obsidian integration. Distinguish source-content reads from permitted identity/configuration reads. Report any separate routing/clarity deviations even if the source-binding criterion passes. Existing source-authorization and answer-review policies are unchanged.

CLI event usage can be retained as observed usage, but this is not a controlled token-cost benchmark. Requested model/effort are explicit; do not infer unexposed effective-model metadata.

Full correction acceptance also checks the existing contracts: successful vault resolution before domain-note reads (bootstrap instructions excepted), source-resolution reference before checkout selection, configure-workspace recovery for missing/ambiguous configuration, retained primary owner, and decisive citations for literal answers. Missing event output is not observable evidence of success; repeat only the affected case without changing policy.
