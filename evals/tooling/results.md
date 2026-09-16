# Installed-tool corrections — verification

Date: 2026-09-16. Base distribution: `61a8fe7` (0.10.0). Corrections are local working-tree changes; this report does not claim a new release or consumer propagation.

## Changes and protected behavior

- Bandit errors or malformed/missing JSON reject acceptance even with a zero scanner exit code. Security findings and process failures remain failures.
- Instance configuration uses one dependency-free parser. Quoted commas, escapes, comments and block plain-text punctuation remain supported independently of optional PyYAML. Duplicate keys and unsupported syntax are rejected; public syntax-error diagnostics remain compatible.
- Graph hygiene includes hidden links and duplicate basenames. Ambiguous basename graphs reject rather than silently overwrite a node.
- Static evidence scanning reuses the shared frontmatter reader and selects tracked working-tree files. Dirty tracked bytes remain eligible and are explicitly distinguished from HEAD; untracked/ignored artifacts and symlink escapes are excluded. NUL-delimited Git filenames preserve whitespace.
- Map closure requires distinct resolved summary paths.
- Synchronization selects the next command and its target once. Unified-diff parsers remain separate because their contracts differ.

No answer-review, authorization or publication gate was removed. All 22 executables remain. Consumer configurations in both Cell A and Cells B/C were successfully loaded read-only with the new parser. Unrelated user-guide work was preserved.

## Executed checks

| Suite | Final passing test count |
| --- | ---: |
| behavior (deterministic fixtures/checkers) | 21 |
| bootstrap | 108 |
| extraction-quality | 18 |
| graph-economy | 4 |
| helper-integrity | 5 |
| investigation-lifecycle | 4 |
| map-closure | 2 |
| response-quality fixtures | 4 |
| response-quality breadth fixtures | 8 |
| sync | 177 |
| tooling quality gate | 4 |
| instance | 14 |
| workspace config | 8 |
| database targets | 5 |
| **Total** | **382** |

Counts include inherited test methods executed in separate classes; they are not 382 independent behavioral scenarios. Broad discovery initially ran 380 tests before two additional regression methods were added. Four sync tests rejected concurrent edits with `run-version-mismatch`; after all implementation edits stopped, all four passed in a focused rerun. The changed parser and helper suites were rerun (14 and 5), and full bootstrap was repeated (108). Final implementation hashes remained unchanged during final verification. No unresolved test failure remained.

Ruff and both Bandit policies passed using the locked Python 3.9.6 environment. A real Python 3.14.6/Bandit 1.8.6 scan that returned internal errors with process exit 0 was rejected by the updated wrapper with exit 2; this is fail-closed verification, not a claim that that Bandit/runtime pairing scans successfully.

The synchronization implementer compared original and revised next-command/action outputs over 110,592 combinations and reported equality. Independent Sol/medium review found and then verified fixes for plain-scalar compatibility, ancestor symlink escapes and leading-whitespace filenames; final review accepted the scoped changes after 30 focused tests (14 instance, 5 helper, 4 quality, 7 next-action).

## Limits

This is deterministic tool validation and independent code review. No new live-model behavioral campaign, token benchmark, production operation or external publication was performed. The scanner does not freeze commit bytes; branch/HEAD identifies its checkout baseline. Broader parser/remote-normalization/transaction refactors and removal of shipped test files were not attempted without a demonstrated equivalent contract.
