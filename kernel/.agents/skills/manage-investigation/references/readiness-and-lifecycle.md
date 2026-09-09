# Readiness and lifecycle

## States

```text
intake → investigating → validating → ready-to-export → exported → closed
```

Knowledge work may transition from `validating` directly to `closed` when its objective is answered and the explicit completion decision, closure reason, evidence reviewed, and outstanding limitations are recorded in Readiness and History. No story, acceptance criteria for implementation, or export is required. Assess `vault-outcome` independently; closure never publishes knowledge.

`blocked` may interrupt any active state. Store the interrupted state in `resume-to`; after the user resolves or accepts the dependency, return there and record the decision. New evidence may move `validating`, `ready-to-export`, or `exported` back to `investigating`.

A closed investigation reopens only through an explicit user decision, transitions to `investigating`, and records the reason in History.

`learning-outcome` is an independent assessment result and never advances or blocks this lifecycle. Material new evidence that could change an earlier learning assessment resets it to `not-evaluated` while History preserves the prior result.

## Transition gates

### Intake to investigating

- Original request and source are captured.
- Objective and initial scope are usable.
- Every supplied attachment has a copy, summary, pending action, or explicit exclusion.
- `purpose` is `knowledge`, `development`, or `mixed`; `undecided` cannot leave intake.
- `vault-outcome` contains a valid value. `not-evaluated` may remain while evidence collection is still required; use `deferred-until-production` when the only candidate is undeployed future behavior.
- `learning-outcome` contains a valid value and may remain `not-evaluated`; learning assessment is not required to advance the investigation.
- A development or mixed case keeps **Current productive state** separate from **Future/proposed state** under **Current state**.

### Investigating to validating

- Current productive state and, when applicable, future/proposed state cover the stated objective and scope.
- Facts, inferences, contradictions, and limitations are separated and sourced.
- Current productive observations and future proposals remain separated; no proposal is labeled as a vault fact or production evidence.
- `vault-outcome` is no longer `not-evaluated`. In a mixed case it follows the current-state candidate when one exists; the future portion remains separately deferred and outside the vault.
- Affected surfaces and active decisions are current.
- Remaining questions are explicit.
- Every existing provisional draft is synchronized or explicitly marked stale; no draft has an unlabeled mismatch.

### Validating to ready-to-export

- No open question or unread source blocks the intended output.
- Contradictions that could change acceptance are resolved or explicitly accepted by the user.
- Acceptance criteria are testable and trace to the current understanding.
- Requester role and export intent are sufficient to choose story kind and audience.
- Readiness lists the evidence reviewed and any accepted limitations.
- Every existing draft intended for the output is current, has no missing, reused, or meaning-shifted register reference, and contains no unregistered source, component, dependency, or implementation claim.
- `vault-outcome` matches the evidence: an undeployed change remains `deferred-until-production`; `candidate-for-audit` records only a handoff candidate; `documented` cites a completed `map-ecosystem` audit, the affected canonical notes, and whether the lifecycle result was no change or a verified write.

### Ready-to-export to exported

- At least one release-ready platform-neutral local draft exists in `exports/`; provisional drafts created in earlier states do not satisfy this gate.
- Every release-ready draft is `current`, its `source-updated-at` matches the investigation `updated-at`, and it references the case-file ID and relevant in-scope evidence or acceptance criteria without changed identifier meanings or unregistered implementation context.

### Exported to closed

- The user accepts the outcome, discards it, or declares the investigation complete.
- Closure reason and outstanding limitations are recorded.
- Any deferred or candidate vault outcome remains explicit; closing a case does not promote it.

### Blocked

- `blocked-on` names the inaccessible dependency or decision.
- `resume-to` names the prior active state.
- History records the observable failure and exact user action needed.

## Completion criterion

A status is valid only when its gate is satisfied. Use the earliest valid state; move backward when evidence invalidates a later gate.

After recording the knowledge readiness evidence and explicit decision, run `investigation-case.py --root "$VAULT_ROOT/.investigations" close --id <id> --decision <complete|abandoned> --reason "<reason>" --limitations "<limitations or none>"`. This transactional command validates the case and records closure without requiring stories or exports.

An explicit `abandoned` decision may close knowledge work from any active state, including intake or blocked. Record reason and limitations; an unevaluated vault outcome becomes `none` (no promotion requested), while existing candidate/deferred outcomes are preserved. Completed outcomes still require validation.
