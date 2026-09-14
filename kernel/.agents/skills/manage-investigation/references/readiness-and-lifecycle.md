# Readiness and lifecycle

## States

Investigations use exactly three lifecycle states:

```text
investigating ⇄ blocked
      ↓           ↓
             closed
closed ──explicit reopen──> investigating
```

- `investigating`: the case is open. It covers intake, evidence collection, analysis, validation, and preparation of any requested output.
- `blocked`: useful progress on the investigation objective cannot continue because of one concrete inaccessible dependency or unresolved decision. Record it in `blocked-on`.
- `closed`: active investigation work ended. Record `closure-outcome: completed` or `closure-outcome: abandoned`.

Do not encode internal workflow stages as statuses. Export, publication, implementation, pull-request, deployment, documentation, and learning outcomes have their own evidence and fields; none changes the investigation status automatically.

`learning-outcome` remains independent. Material new evidence that could change an earlier assessment resets it to `not-evaluated` while History preserves the prior result.

## Investigating

Open every new case directly as `investigating`. `purpose: undecided` is valid while the requested outcome is still being classified, but resolve it before declaring completion or producing an external or development output.

Continue in `investigating` while any useful in-scope action can advance the objective. A missing item that blocks only one story, repository, or optional evidence path does not block the whole case when other useful investigation work remains.

## Block and unblock

Use `blocked` only when the named dependency prevents useful progress on the global investigation objective. `blocked-on` must be one concise, portable description of that dependency. History records the observable failure, reason, source, recorder, and the user or external action needed when known.

Do not store `resume-to`; unblocking always returns to `investigating`. Use the helper with the exact snapshot returned by `load`:

```text
investigation-case.py --root "$VAULT_ROOT/investigations" transition \
  --id <id> --to blocked --blocked-on "<dependency>" \
  --reason "<formalized reason>" --source "<portable source>" \
  --expected-public-sha256 <sha256>

investigation-case.py --root "$VAULT_ROOT/investigations" transition \
  --id <id> --to investigating \
  --reason "<why progress can resume>" --source "<portable source>" \
  --expected-public-sha256 <sha256>
```

## Close

Closure is an explicit, attributed decision. It is based on the case objective, not on `purpose`, story export, handoff state, or deployment by default.

### Completed

Close with `closure-outcome: completed` only when all of these are true:

- `purpose` is resolved to `knowledge`, `development`, or `mixed`;
- the current state answers the objective and covers the agreed scope;
- facts, inferences, contradictions, limitations, decisions, and open questions are current and traceable;
- applicable acceptance criteria or other objective-specific completion conditions were verified or explicitly accepted with evidence;
- `vault-outcome` was evaluated independently;
- affected drafts are current or explicitly stale;
- the closure reason, supporting register IDs, and outstanding limitations are recorded.

Implementation, export, publication, merge, or deployment evidence is required only when the stated objective or an applicable acceptance criterion requires it. A knowledge case can complete without stories. A development or mixed case can complete after producing an agreed, verified specification even when implementation remains outside scope. Conversely, a case whose objective includes productive deployment cannot complete from source, a handoff, or a clean Git merge alone.

Invoke:

```text
investigation-case.py --root "$VAULT_ROOT/investigations" close \
  --id <id> --decision complete --reason "<formalized reason>" \
  --limitations "<formalized limitations or none>" \
  --source "<portable closure source>" \
  --evidence E-001 --evidence AC-001 \
  --expected-public-sha256 <sha256>
```

### Abandoned

Use `closure-outcome: abandoned` when the requester explicitly ends the work without satisfying the objective, or when the work is deliberately discontinued. Record the reason and unresolved limitations. Closure evidence IDs are optional. `purpose` may remain `undecided`; an unevaluated vault outcome becomes `none`, while an already evaluated outcome is preserved.

### Reopen

Reopen only for an explicit request or new material evidence that requires active investigation. Transition from `closed` to `investigating`, remove active `closure-outcome`, and preserve the earlier closure event in History:

```text
investigation-case.py --root "$VAULT_ROOT/investigations" transition \
  --id <id> --to investigating \
  --reason "<reopen reason>" --source "<portable source>" \
  --expected-public-sha256 <sha256>
```

## Output sufficiency is separate

Evaluate publication or development readiness for the selected `S-NNN`, work item, and repository package. Only questions, dependencies, decisions, and criteria applicable to that exact output can block it. An unrelated pending story or optional investigation branch does not.

A case status never proves output sufficiency:

- an `investigating` case may produce a sufficient output for one bounded target while other work continues;
- a `blocked` case may still expose an unaffected, already sufficient output, provided the global blocker cannot change it;
- a `closed` case may supply an unchanged output from its recorded snapshot; material new evidence requires reopening first.

Apply the export contract and the development input-bundle check independently. Keep unresolved output-specific gaps in the relevant story or package; keep global blockers in `blocked-on`.

## Completion criterion

The lifecycle is valid when the case uses only the three states, transitions use current-snapshot compare-and-swap, every transition is attributed in History, blocked metadata exists only while blocked, closure metadata exists only while closed, and the semantic gate above supports the recorded outcome. Structural validation is necessary but does not replace the agent's objective-based review.
