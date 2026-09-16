# Response quality implementation checks — 2026-09-16

## Implemented scope

The kernel now routes evidence-based conversational and retained conclusions through one response-quality contract. It defines author review, independent review for operational decisions or material uncertainty, prior-review reuse, bounded corrections and plain-language delivery. Existing technical publication and required-executor gates remain authoritative. Model examples are open recommendations; personal restrictions remain binding. Direct source reads avoid graph discovery only when the source is already identified and relationships are unnecessary.

The synchronization status adds `next_action` with a selected unit/package, argument array and missing inputs while preserving `next_command`. Note-candidate errors retain their codes and add bounded drift diagnostics. Neither change grants authority or creates another orchestrator.

## Observed checks

- Bootstrap: 35 tests passed after linking the new reference from conventions.
- Instance: 10 tests passed.
- Existing analysis fixture: 3 tests passed.
- Existing analysis runner mechanics: 4 tests passed (fake model process, not behavior proof).
- Response-quality fixture: 4 tests passed; executable source semantics and separation from evaluator material.
- Note-candidate integrity and diagnostics: 17 tests passed, including projection-bound drift.
- Next-action contract: 7 tests passed, including parser-compatible arguments and no mutations.
- Existing synchronization state/recovery suite: 59 tests passed in the final run.
- Total across these suites: 139 passing tests.
- Independent static review accepted the policy after clarifying the repair limit and authorized-scope boundary. Code review found no weakening of synchronization gates; a projection-bound diagnostics coverage gap was corrected with focused regression tests and accepted in the final independent spot review.

## Behavioral evidence still pending

A fresh installed synthetic worker and separate evaluator were prepared locally. The real-model campaign was not started: the local isolation preflight did not establish the required evaluator-content read denial. An initial nested sandbox failed to start; subsequent preflights failed; an attempted configuration-isolation option was unsupported by the sandbox CLI. No response, independent-review behavior, multi-turn success or token-saving result is claimed from those attempts. The fixture is ready for a harness whose effective boundary has been verified.

For a campaign, retain each initial response, tool/reviewer observations, and total coordinator/worker usage. Judge initial failures separately from later repairs. Reuse and changed-scope turns must run in the same session with the review injected only at its specified turn. Compare equal inputs and executor settings across repeated revisions before claiming savings.

## Follow-up boundary

A repository-wide utility/duplication audit and comparative cost campaign are separate remaining work. No script is declared redundant from its line count. No validator/executor wrapper was added: provider adapters belong to their configured installation and the shared router now explicitly requires a passing exact-operation check. Consumer propagation, versioning and remote publication were not performed.

## Subsequent real-consumer worktree test and iteration

A separate detached worktree was subsequently created from a consumer's committed HEAD, excluding its pending changes. The working distribution was installed only there. Fresh subagents used the same read-only question, committed local source copies and a supplied historical database snapshot; they received neither earlier answers nor evaluator criteria. This was an instruction-bounded subagent test, not proof of OS-enforced read isolation or current production behavior.

The first response failed: it assigned activation semantics to a stored flag without demonstrated consumer use, and performed only author review. Later correct answers did not erase this first-response failure.

The policy was narrowed to require independent acceptance for new operational status, eligibility, enablement and cause classifications even in read-only questions. Until effective use is established, flags retain literal values and unknown operational effects. Static review also clarified that prior independent acceptance can be reused and that unavailable review yields supported observations rather than an unreviewed operational classification.

Two subsequent fresh workers passed their initial answers under independent evaluator assessment. Both obtained actual independent review before delivery, limited conclusions to supported selection behavior, and distinguished supplied observations from current execution. One ran the intermediate clarification and the other the final wording. The final-wording run also rejected provider mismatch as an established failure cause in a follow-up. These are bounded observations, not a statistical reliability or token-saving claim.

After the final wording, bootstrap 35/35 and instance 10/10 passed. The local evaluator retained policy hashes, source revisions and checkout fingerprints outside the worker. The original consumer and prepared worker were checked for unchanged files, HEAD, status and local Git configuration. No original-vault update, production query, commit or push was performed.

## Diverse adversarial campaign and targeted iteration

Six fresh workers answered ten turns covering asserted delivery from HTTP success, mobile success versus persistence, historical tests versus deployment, EPC input shapes, an unidentified urgent incident and a correctly stated selection rule. Independent evaluation accepted four cases and rated two partial: the HTTP answer omitted available implementation evidence, and the EPC answer omitted a locally decidable decoding distinction. Safe uncertainty alone did not satisfy completeness. These initial partials remain in the record.

The existing analysis method and review rubric were clarified, without another workflow: resolve locally answerable subclaims before reporting external evidence gaps; check decisive implementation and omitted supported conclusions; use declared source identity while retaining deterministic checkout binding.

The two partial cases were repeated with fresh workers, unchanged questions and no evaluator answers. Both initial responses passed independent evaluation and obtained actual independent review before delivery. Two further fresh workers tested explicit single-entity HTTP success and empty-container versus null-element EPC input. Follow-ups pressured the original two workers to turn unknown success into confirmed failure, and possible publication into confirmed stock movement. Full results and source-bound review remain in the local campaign record; no live production behavior or statistical reliability is claimed.

After the final policy edits, bootstrap 35/35, instance 10/10 and response fixture 4/4 passed; diff whitespace checks passed. Earlier code suites were not rerun because the final delta only clarified agent instructions. The original consumer's 4,280 files and the test worktree's 1,073 files, HEAD, status and local configuration were preserved. No token/cost comparison was performed.

The final independent evaluator accepted all four latest scenarios and all six responses, including both follow-ups. Across the diverse campaign there were ten fresh-worker runs and sixteen responses spanning eight distinct scenarios (two original questions were repeated after correction). The earlier two partial ratings remain historical failures of completeness, not retroactive passes.

## Additional diversity and continuity round

The frozen policy was tested with five fresh-worker initial runs (four new questions and one identical blind repeat), one repair and three follow-ups: nine responses. The cases cover conflicting documentation/component identity, incomplete pagination, a partly true premise, unsupported numerical confidence, reuse of a reviewed conclusion and changing environment. Eight responses passed substantive evaluation; one first response delivered an operational classification before independent acceptance. A later real review repaired that response, and a fresh worker passed the identical prompt with prior independent acceptance. The initial gate failure remains recorded. One substantively accepted response added redundant review metadata beyond the requested three sentences.

No new policy rule was added: the failed obligation was already explicit. The successful blind repeat demonstrates observed compliance, not elimination of stochastic failure. Same-scope follow-ups reused review without repeated investigation; the changed-environment follow-up reused only static rules and withheld the unsupported current-state classification. No quantitative token-saving claim follows from these observations.

## Closing round after role clarification

The reviewer-recursion sentence was clarified: the author, including a subagent, requests and waits for acceptance; the reviewer does not delegate another review. The test dispatch also separated review metadata sent to the coordinator from the final user answer, removing a reporting conflict with strict sentence limits. This setup change and the earlier global stopping-rule clarification prevent attributing results solely to the kernel wording.

A frozen-policy round used six fresh workers and five follow-ups: eleven responses. All six cases and eleven responses passed the predefined substantive, prior-review, continuity and format criteria under a separate evaluator. Cases covered selection and alternative paths, same-scope reuse, environment change, HTTP success versus completion or failure, EPC input forms versus inventory effects, conflicting source documentation, empty-page coverage, unsupported numerical confidence and a partly true premise in three sentences. All six initial answers had actual independent acceptance before delivery; five used child reviewers and one waited for a coordinator's source inspection and acceptance. The later evaluator was not counted as that gate.

Two source-navigation sequencing deviations remain recorded: a worker read and another searched authorized source content before completing the repository locator; both verified the exact identity before concluding. These did not fail the frozen answer/review/format criteria, but the round does not demonstrate perfect compliance with every navigation instruction. One worker received source-identity metadata from the coordinator after requesting it, never an answer oracle. Earlier failures and partials remain unchanged.

After the clarification, bootstrap35/35, instance10/10 and response fixture4/4 passed. Original consumer and worker fingerprints, HEAD, status and local configuration remained intact. This completes the bounded behavioral acceptance round, not a claim of statistical reliability, production correctness, OS-enforced isolation or token savings.

## Broader Sol/medium acceptance campaign

See [breadth/results.md](breadth/results.md), the reproducible13-case fixture and [write-workflow report](breadth-write-results.md). After an initial source-inspection failure and excess review on a routine answer, narrow policy clarifications were tested with an entirely fresh13-case matrix. Independent evaluation accepted13/13 cases and14/14 final responses (one repeated routine case). Across initial, adversarial, intermediate and final runs there are33 responses, with earlier defects preserved. Behavioral workers/reviewers were explicitly dispatched Sol/medium. Mechanical regression covered327 executions (59 inherited duplicates) across8 suites, plus8 new breadth-fixture tests. Real disposable investigation and note publication completed with independent review, stale-write rejection and published-byte verification. This is wider evidence for the candidate, not general reliability, production or token-cost proof.
