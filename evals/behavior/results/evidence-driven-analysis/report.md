# Evidence-driven analysis acceptance report

## Scope

Candidate kernel: **0.7.24**, local branch `issue/evidence-driven-analysis`. Implementation started at `089bce9`; the branch was subsequently rebased onto concurrent main revision `6cb92ac` without modifying main. The final delivery commit is the tip of this branch. Existing investigation formats, lifecycle states, helper interfaces, and writer ownership are unchanged.

The method is separate from case persistence. Conversational questions do not create records; explicit case requests retain the existing transactional writer. Mapping, operations, handoff, reconciliation, learning, and publication retain their specific access and completion gates.

## Acceptance matrix

`execution-evidence.json` retains sanitized final responses, actual commands and exit codes, original trace hashes, prompt hashes, file-change observations, and suite receipts. Full raw traces were inspected independently before scratch cleanup. The evidence is a bounded offline evaluation, not production validation or a statistical performance claim.

| Criterion | Final evidence | Result |
|---|---|---|
| Simple query | `c2-simple`: correct missing-versus-zero distinction, brief answer, no writes | OK |
| Distributed evidence | `c5-cross-source`: historical vault note → configured identity resolution → inspected repository → lab observation; rejects unsupported rounding | OK |
| Diagnosis | `c4-diagnosis`: fresh sample without a preanswered cause note; source explains 9.5/0, rejects rounding and limits claim to supplied implementation | OK |
| Audit / incomplete evidence | `c2-cloud`: recognizes pagination, unexecuted receipt query and acceptance versus persistence; no live call or record | OK |
| User interaction | `c2-missing`: inspects accessible evidence before requesting sanitized production inputs, outputs, time and revision | OK |
| Variants | `c2-variants`: confirms recount manifestation and rejects preview look-alike through its different input-unit contract | OK |
| Continuity / authorization | `c8-continuity`: one actual resumed session; zero writes on advice, zero after refusal, one case after explicit request | OK |
| Existing case privacy | `c2-overlay`, `c2-overlay-absent`, corrected `c4-routes`: helper discovers overlay by ID; restricted facts stay in local response, shareable summary omits them; absence is acknowledged; no writes | OK |
| Documentary update | `c6-document`: one supplied E-003 limitation, timestamp and attributed History; previous IDs and private overlay unchanged; no redundant diagnosis | OK |
| Concision | `c8-continuity`: three evidence entries, two substantive questions, no artificial opening/pending decisions; criteria reference existing IDs and inapplicable sections remain empty | OK |
| Linked flows | `c4-routes` verifies owners and no recursive/persistent response to a query; `c2-operational` actually writes only the requested offline Audit ledger with unknown coverage/persistence retained; bootstrap and transaction suites exercise handoff/binding/reconciliation gates | OK |
| Isolation | Candidate installed only into two consumer worktrees; original case/config fingerprints unchanged and candidate absent. Concurrent original changes were explicitly confirmed by the user and retained | OK |

The continuity trace captures successful function invocation but no numeric stdout. A separate coordinator check against that unchanged synthetic checkout observed and asserted `available(18,2250) == 15.75` and `batch(18,2250) == 0`. This supplements the trace rather than attributing unobserved stdout to the executor.

## Corrections and independent review

- Replaced final-message replay with actual session resume. Four runner tests verify session identity, prompt isolation, fixtures, and side-effect recording on failure.
- Removed the variant-only confirmed-cause note from diagnostic runs and reran a new numerical sample. Earlier contaminated diagnostic runs are not accepted evidence.
- Required helper-based overlay discovery before using case context; a fresh routing/continuity run verified the correction.
- Required configured repository binding before reading vault-linked code; a fresh sample verified resolver and locator ordering. An offline Obsidian stub prevents registration calls to the host application.
- A long case remained repetitive after an initial wording correction. The final pre-persistence editing gate in the record contract removes artificial decisions and duplicated detail; a fresh unseen numerical variant passed independent semantic review.
- Fixed a hidden-agent-link violation exposed by bootstrap and aligned both distribution version files.
- Removed sibling task prompts from executor directories in the runner. Requests now enter only at their actual turn. In accepted historical traces, reviewers observed no reads of sibling task contents; the runner tests cover the stronger isolation.

Independent contexts reviewed implementation and observed behavior separately. The implementation reviewer found no actionable kernel/helper/schema regression. The behavioral reviewer rejected contaminated/bypassed/repetitive executions, then accepted the corrected scenarios after inspecting actual operations, artifacts, and full initial-file hashes. Possible stylistic shortening is not treated as a defect where the agreed semantic-concision criterion is satisfied.

## Controlled comparisons

Same model (`gpt-6-astra`), low effort, sandbox, natural prompts and source fixtures within each pair:

- Simple: baseline `089bce9` and candidate both correct, no writes; commands 5 → 6.
- Informational cloud audit: baseline `089bce9` and candidate both correctly limit the conclusion; commands 5 → 4.
- Blind diagnosis: baseline `089bce9` and candidate both derive the source-supported explanation without the preanswer note. Candidate static reasoning and baseline local execution are both valid for the question; no execution requirement is imposed on every diagnosis.
- Cross-source: baseline `6cb92ac` and candidate both pass with configured locator and equivalent offline access. Main advanced concurrently; the installed lock records this baseline revision rather than silently labeling it `089bce9`.

This demonstrates the required behavior and non-regression in these samples. It does not establish a general quality, token-cost, or speed advantage. Initial failed iterations are described above, not counted as final passes.

## Regression and installation

Final suites: bootstrap **35**, instance **8**, investigation transactions **22**, portability **1**, workspace configuration **8**, graph queries **4**, analysis fixture **3**, runner **4**: **85 passing tests**. Skill metadata validation and `git diff --check` also pass.

Bootstrap covers source/recipient responsibility boundaries, exact handoff identity, CAS binding, multi-provider packages, state transitions, materialization and closure fingerprints. Transactions exercise stale snapshots and rollback. This is execution coverage of the affected mechanisms, not a claim that the read-only routing prompt materialized a handoff or published a learning.

Both isolated consumer installations pass `doctor --strict`, report valid instances, matching managed payloads, reproducible clean distribution and no drift. Their local configurations select only the synthetic test repository; resolver output binds each test worktree, never its original checkout.

## Preservation under concurrent work

Initial fingerprints covered **4,047 Cell A files** and **1,006 Cells B/C files**, plus HEAD, status and a hash of local Git configuration; configuration values were not copied. Every installer/configuration mutation in this execution targeted the named test worktrees. Original access was read-only state/hash inspection.

The user explicitly confirmed that other tasks were active in the originals, so preservation means isolation from this execution, not freezing their work. Cell A advanced `a6ca7e1 → 69c4e66`; Cells B/C advanced `ec501b1 → 97288ef` during the first check. Tracked differences correspond to their concurrent map/knowledge commits; ignored Obsidian/map-plan state also continued evolving. Nothing was restored or incorporated into this task's consumer changes.

Observed invariants: original Git status and local configuration hashes unchanged; no changes to investigations, legacy/private investigations, instance identity or local workspace/personal configuration; both originals still at 0.7.23 without `evidence-driven-analysis`. Main remained clean and received no analysis commit. Rebase affected only the local issue branch to retain the concurrent distribution change.

## Sources and delivery boundary

Pinned references, licenses and local adaptation boundaries are in `kernel/.agents/skills/evidence-driven-analysis/references/sources.md`: Matt Pocock and Obra (MIT), Trail of Bits (CC BY-SA 4.0). The method is independently written; upstream code/templates/instruction blocks are not vendored.

Delivery excludes main integration, real-vault propagation, remote pushes and real investigation migration. Test worktrees/branches and execution fixtures are removed after final review; reproducible tests and this sanitized evidence remain on the implementation branch. Final acceptance includes a post-cleanup check of main cleanliness and absence of the temporary worktree registrations.
