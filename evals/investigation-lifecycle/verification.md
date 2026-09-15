# Lifecycle verification — 0.9.0

Result: GO for the scoped implementation and synthetic validation; final combined
independent review has no pending findings.

Scope: isolated distribution branch `issue/investigation-lifecycle`, based on
`6f102eadf1cf162510923f6c6aa6736e1480c375`. All behavioral sources, identities,
receipts and Git repositories were synthetic. Consumer vaults were not modified.
No real schedules, deployments, pushes or investigation retirements were performed.

## Corrections and evidence

The initial absorption executor published using its own review despite a missing
independent reviewer. The investigation-context reference now requires staged-only
candidates when independent review is unavailable. A fresh negative execution
preserved both canonical notes and the private overlay and recorded only pending
correspondence through the case helper. Its request authorized that attributed
case update; the original unchanged-case rubric was therefore corrected rather
than treating this permitted write as a product defect.

The earlier positive execution had a manifest-bound receipt but no observable
reviewer dispatch in its CLI trace. That execution does **not** establish reviewer
independence. The replacement uses separately witnessed phases, without relying
on missing nested CLI events:

1. Fresh producer `absorption_witness_producer` prepared two complete candidates
   from the natural request and installed skill, without receiving judge criteria.
2. Coordinator verified original note and case hashes before publication.
3. Fresh reviewer `absorption_witness_reviewer`, without producer conversation,
   inspected both complete candidates, originals, guidance and five bound evidence
   files. It returned `accept`, no findings, for manifest
   `ae58cbffec277c6703bdfbd7df404f872b2545d0d364d6de26b4cede8234f169`.
4. Reviewer and coordinator each ran the installed checker: `note-candidate-reviewed`,
   `status: pass`. Coordinator again verified unchanged original hashes **after**
   review before authorizing the producer to resume.
5. Producer resumed only after that independent decision. Coordinator repeated
   `verify-published`: two files checked, zero mismatches, `status: pass`; whole-root
   case validation returned one valid case and no warnings. The case remained
   investigating with production criteria open. The private overlay hash stayed
   `0e4093051f82302086058ea13dad8c0651975989bac4682246d6425f64383679`.

This run also exposed a fixture seed dated one minute after its simulated clock.
The generator now seeds at the scenario time, with a chronological regression
assertion. This is a fixture correction, not a change to the production helper.
The witnessed run's preserved seed timestamps do not establish real-world dates.

The two reviewed candidate SHA-256 values are:

| Note | Original | Reviewed candidate |
|---|---|---|
| `30-Flujos/Receipt handling.md` | `611bc260ae83c561f3314db377385b48e5978aa493f089017fd94c739b1d835f` | `72ccda5bd3be784125ee05018f0ee526c18fcfa4449cb4ede51de1043646e249` |
| `40-Integraciones/Receipt contract.md` | `1150a84b0bc45f8338f1860a81b647db24e03083adb4f6f45bb24e2bb873f114` | `7c37600c2d614619cec6ffa8c854da307793a98f58c45b640272fd6073036eae` |

## Scenario matrix

| Scenario | Observed result |
|---|---|
| Component progress | Independent receiver observation and reporter preparation; aggregate remains investigating; release guide does not execute a release. |
| Observation proposal | User-specified cadence retained; ambiguous coverage remains proposed and collection pending; access gap disclosed; case unchanged. |
| Scheduler unavailable | Separate-context proposal stays draft; missing native capabilities disclosed; no scheduler or workaround created. |
| Access unavailable | Exact uncovered interval and unknown result recorded; no invented zero/error count; case unchanged. |
| Successful observation | Fresh access/target/window checked; one of three intervals recorded and reported; campaign remains open; case unchanged. |
| Expired campaign | Runtime snapshot not read; final report uses retained evidence and identifies incomplete coverage. |
| Cancelled campaign | Runtime snapshot not read; cancelled state and case preserved. |
| Selective absorption | Separate witnessed reviewer accepted the frozen candidates before publication; two published hashes matched; attributed case correspondence saved and validated. |
| Reviewer unavailable | Canonical notes/private bytes unchanged; attributed pending-case update allowed; no completed-absorption claim. |
| Retirement | Closed public directory and attachment retired through helper, joint deletion/ledger commit, exact history lookup; private and unrelated bytes preserved. |
| Historical lookup | Exact snapshot returns synthetic receipt marker and limits, without restoring or modifying the case. |

The behavior judge independently inspected operation traces and artifacts. The
final code reviewer reported no actionable findings and independently repeated
the 20 retirement tests and 4 fixture tests. After the corrections and witnessed
publication, its combined re-review independently confirmed the two published
hashes, valid case, unchanged private hash and chronological fixture checks, with
no actionable findings. Unavailable native scheduling is the
expected result of its scenario, not proof of real scheduler integration. These
evaluations do not claim production behavior, cross-harness equivalence or native
Obsidian rendering.

## Automated checks

274 tests passed across bootstrap (35), instance (10), transactions (22),
portability (1), graph query (4), workspace configuration (8), retirement (20),
lifecycle fixtures (4) and sync regressions (170). Fixture tests validate input and
evaluation contracts; behavioral acceptance comes from the separately observed
executions above. Six affected skill entrypoints passed skill validation.
`git diff --check` passed. A fresh installation from clean implementation commit
`97dbce7c6b965e33bdd3a70d304d3ad0583c676f` passed `doctor --strict`: version 0.9.0,
valid instance, reproducible distribution, matching managed files, and no adapter,
distribution or topology drift. Delivery retains the local issue branch; main
integration and real-vault propagation remain outside this execution.

No scheduler engine, new investigation state, component state machine, duplicate
case archive or provider-specific deployment strategy was introduced.
