# Synchronize the vault

## Governing invariant

> **Regla principal de la sync**
>
> La sync no es un quality gate, una auditoría de seguridad ni un mecanismo para mejorar repositorios. Su pregunta es únicamente:
>
> **¿Qué conocimiento durable puede incorporarse al vault a partir de estos cambios?**

Treat “no durable knowledge” as a valid successful answer. A completed synchronization persists the decision against the inspected commit so the same evidence is not processed again. Credential and deployment observations constrain safe reading or claim eligibility; they are not source-repository remediation, external pending work, or prerequisites for completing the synchronization.

Once the exact Git range is accessible and readable, repository content cannot leave the synchronization unanswered. The run persists one of two kinds of result for that commit: an accepted semantic decision, or an operational cursor stating that no semantic package was accepted. An operational cursor closes the run without changing `commit-analizado` and without claiming that the repository contains no durable knowledge. Only failure to resolve or read the exact Git evidence, loss of branch freshness before writing, or failure to persist the vault result is operationally incomplete.

## Input and authority

Confirm whether the user requested a report or an update. Keep report mode read-only. In update mode, load [node-selection.md](node-selection.md), limit writes to the vault, and keep remote repositories and GCP read-only. The explicit update request authorizes the expected vault-note, accepted traceability, and cursor writes after the batch passes; do not pause again merely because those expected documentation changes are material. Pause only if the target, external effect, destructive effect, or user-authorized scope changes.

Source repositories are evidence surfaces, not mutation targets. Report source observations only as evidence boundaries; never turn them into remediation work, quality findings, security findings, Jira work, or external blockers unless the user separately asks for that work.

## Inventory and lifecycle

1. Run `python3 90-Meta/vault-inventory.py --format markdown` from `VAULT_ROOT` and keep the report ephemeral. Let the inventory resolve and report the task-local GitHub identity. If several authenticated accounts can access the organization, repeat with `--github-user <login>`; never switch or persist the global `gh` account.
2. Require one classification for every relevant repository/note: `current`, `changed`, `new`, `acknowledged-no-node`, `acknowledged-review-rejected`, `acknowledged-inspection-limited`, `container`, `archived`, `renamed-or-transferred`, `missing-candidate`, `branch-ambiguous`, or `invalid-note`.
3. Treat absence, rename, transfer, and archival as candidates; verify them before changing or deleting a note.
4. For `changed`, apply the protocol below using the diff between `commit-analizado` and the new HEAD.
5. For `new`, use the same package pipeline after confirming SYS001/SYS002 or explicit cross-app approval and the production branch; inspect its complete current tree and defer note creation until the batch passes every barrier.
6. For any `acknowledged-*` status, stop when the cursor SHA equals the observed production-branch SHA. A changed SHA reopens the repository against its last accepted semantic baseline; an explicit forced audit may also bypass the cursor.
7. For `current`, do not re-analyze source code or alter traceability.
8. For `container`, verify its Meta representation without attempting self-referential SHA tracking.

## Changed and new repository protocol

Create exactly one temporary run root outside `VAULT_ROOT` and every source-repository root. Record its exact path before delegation. Store every manifest, scaffold, analysis, review, helper result, and run-status file only below that root; no synchronization worker may write anywhere else. Capture `git status --short` for every source checkout before processing and require the same bytes at closure.

The coordinator records phase wall times for exactly `preflight`, `package-preparation`, `extraction`, `mechanical-validation`, `review`, and `gate-write-closure`, plus extractor/reviewer duration by repository when the harness exposes it. It also records one repository-sorted safe finalization summary containing only `repository`, `fallback_used`, `input_issues` code/field pairs, and the observed review disposition. Timing and finalization summaries are observational metadata under the temporary run root, not another gate, retry trigger, or vault artifact. They contain no claim text, source bytes, finding reasons, or sensitive values.

### 1. Freeze the exact range

Resolve every checkout by remote identity through the semantic workspace API. Keep existing source checkouts read-only; acquire missing objects only through the configured managed clone authority. For `changed`, resolve the recorded commit and production-branch HEAD to full commit OIDs and require ancestry. For `new`, resolve the production-branch HEAD and compare its complete tree with Git's empty tree.

Build one manifest per repository and keep it ephemeral:

```text
# changed
python3 -B 90-Meta/git-change-manifest.py build --repo <resolved-checkout> --old <recorded-commit> --new <observed-head>

# new
python3 -B 90-Meta/git-change-manifest.py build-new --repo <resolved-checkout> --new <observed-head>
```

The manifest contains only exact OIDs, sorted path/status/`content_kind` records, derived `environment_configs`, and conservative `credential_suspects` with path and one-based line. For a changed range, credential detection inspects text blobs on both sides, including deleted content; `id-token` is exempt only when its value is `read`, `write`, or `none`. `build-new` records the empty-tree OID as `old_oid` and every current tree path as `added`. Neither command emits file bytes or sensitive values. Exit `2` is a contract blocker; exit `1` is an operational failure. Complete when the selected manifest command exits `0` and every repository has one canonical manifest for its exact range or complete tree. The normal flow runs exactly one manifest command and one `init-analysis` command per repository; the tooling tests prove byte stability.

### 2. Extract one package per repository

Before delegation, collect the known vault-context basenames as deterministic seed nodes. Initialize the package once, passing each seed through a repeated `--node` argument. `init-analysis` rejects structural basenames such as `.`, `..`, path separators, and control characters:

```text
python3 -B 90-Meta/git-change-manifest.py init-analysis --manifest <manifest.json> --repository <full-repository-name> [--node <basename>]...
```

Keep its stdout as the immutable `<scaffold.json>` and copy that file to `<analysis.json>`. The scaffold freezes identity, exact paths, checklist shape, and mandatory seed nodes; it does not freeze the final semantic node set. The coordinator resolves repository identity, source checkout, production ref, exact OIDs, known vault context, search roots, and seed nodes once; these are coordinator-owned bindings and workers do not rediscover them. The coordinator also applies the closed sweep and deployment rules from [single-unit-documentation.md](single-unit-documentation.md) and [deployment-evidence.md](deployment-evidence.md) when preparing the card and scaffold.

For every non-empty repository, create one extractor card marked `SYNC_PACKAGE_WORKER_V1` with the exact fields from [synchronization-package-worker.md](synchronization-package-worker.md), set `allowed_write` to that package's `<analysis.json>`, and start it in the temporary run root. Dispatch exactly one normal extractor per repository. Give it the card, manifest, scaffold, exact vault-context paths, and no top-level synchronization request. Run independent cards in parallel when available; otherwise process them sequentially in isolated contexts. The card and compact worker reference are the extractor's complete operating contract; do not ask it to load the full synchronization, single-unit, deployment, resolution, Framework, or node-selection documents.

The extractor candidate contract lives only in [synchronization-package-worker.md](synchronization-package-worker.md). The coordinator supplies that compact contract without restating it in the card. The deterministic finalizer reconstructs canonical analysis version `2` from the immutable scaffold and recognized semantic decisions; an agent never owns identity, record shape, ordering, checklist status derivation, redaction, or result reconciliation. Missing semantic coverage remains invalid instead of being invented.

A credential suspect remains a redaction boundary rather than an automatic blocker. Missing productive applicability or an inaccessible reusable workflow answers the dependent question as `not-observed` and excludes dependent claims. `init-analysis` closes only a `build-new` manifest with zero paths as canonical `no-change`; every non-empty tree proceeds through semantic extraction.

Finalize each extractor output exactly once before `check`:

```text
python3 -B 90-Meta/git-change-manifest.py finalize-analysis --repo <resolved-checkout> --manifest <manifest.json> --scaffold <scaffold.json> --analysis <analysis.json> --output <finalize-result.json>
```

`finalize-analysis` is the sole deterministic writer after extraction. It atomically projects recognized semantic decisions onto the immutable scaffold, derives canonical identity, ordering, checklist statuses and result, drops unsafe claims, redacts remaining occurrences, and persists `finalize-result.json` as the package receipt. The command refuses to run when that receipt already exists; after any timeout or lost stdout, inspect the receipt instead of invoking the finalizer again. Missing semantic coverage, identity mismatch, malformed JSON, or a candidate that remains invalid produces the exact no-claim fallback without invoking an agent. `gate-batch` recognizes that fallback and permits only an `inspection-limited` cursor. A fallback package skips semantic review and advances directly to the single gate with its review path absent. Complete when every accessible repository has one finalized package and the finalizer disposition is recorded.

### 3. Apply the mechanical gate

Validate every package before review:

```text
python3 -B 90-Meta/git-change-manifest.py check --repo <resolved-checkout> --manifest <manifest.json> --scaffold <scaffold.json> --analysis <analysis.json> --current-ref <explicit-production-branch-ref>
```

`check` rejects duplicate JSON keys, binds the declared repository to the supplied checkout's Git remote name, reconstructs the canonical manifest from the exact checkout and OIDs, requires every scaffold seed while permitting nodes discovered during extraction, verifies every evidence path against the old or new Git object, enforces redaction and path-disposition/claim/write-node consistency, resolves the explicit local or remote branch ref itself, and returns canonical manifest/scaffold/analysis SHA-256 digests. A raw OID is not a freshness ref. Run it only on the finalized package and send only exit `0` forward. `check` remains read-only and never repairs output. After successful finalization, exit `2` identifies an invalid manifest/scaffold/source binding or a helper defect, not repository content or work for a source owner. Record `fallback_used` from finalization: fallback packages go directly to the gate and every other valid package advances to review. Complete when every package has one check result and one deterministic next stage.

### 4. Review in a fresh context

Create one reviewer card marked `SYNC_PACKAGE_WORKER_V1` per checked non-fallback package, reuse the same frozen coordinator bindings, add the successful check result, set `allowed_write` to that package's `<review.json>`, and start it in the temporary run root. Dispatch exactly one fresh reviewer per eligible package. Give it no expected answer, prior review, author diagnosis, or intended correction. It writes review version `3` with exactly `version`, `repository`, `manifest_digest`, `scaffold_digest`, `analysis_digest`, `verdict`, and `findings`; the three digests must be copied from that package's successful `check` result. `verdict` is `accept`, `revise`, or `blocked`. Each finding has exactly `target`, `category`, `reason`, a non-empty sorted `nodes` list naming the exact affected canonical basenames, and `{path, anchor}` evidence.

The compact card and package-worker reference are the reviewer's complete contract. The coordinator never answers a worker request for schema or semantic clarification during the run. A worker that does not materialize its one artifact has an absent or invalid attempt, which the existing one-pass gate converts to `inspection-limited`; it does not receive another prompt or agent.

`accept` requires an empty finding list, justified path coverage, all applicable questions, production evidence for every claim, and complete node decisions. `revise` identifies one or more concrete inconsistencies; each claim-local inconsistency targets exactly `claims.<claim_id>`. Reviewer `blocked` means no claim from the package is eligible for publication over the inspected evidence. Both require at least one exact finding. A valid `revise` is terminal for review but not necessarily for the whole package: when every finding targets an existing claim and at least one unchallenged claim remains, the batch gate deterministically selects `partial-accept`, excludes the challenged claims, and preserves the rest without another extraction or review. If a finding is not claim-local or every claim is challenged, the gate discards the package claims and selects `review-rejected`. A `blocked`, missing, malformed, or contract-invalid review selects `inspection-limited`. Neither decision asserts the absence of durable knowledge. Complete when every non-fallback package has one observed review attempt; fallback packages never enter this stage.

### 5. Scope package and node barriers

Wait until every expected accessible repository has a finalized package and every non-fallback package has an observed review attempt. Repository content cannot create a missing item: `finalize-analysis` replaces missing or invalid extractor output, and `gate-batch` converts a missing or invalid review into a no-claim fallback. Only an unresolved checkout, unreadable or invalid immutable manifest/scaffold binding, or stale source makes the batch mechanically invalid. For the complete set, run the mechanical gate once:

```text
python3 -B 90-Meta/git-change-manifest.py gate-batch --output <gate.json> --expected-repository <full-repository-name> [...] --item <resolved-checkout> <manifest.json> <scaffold.json> <analysis.json> <review.json> [--item <resolved-checkout> <manifest.json> <scaffold.json> <analysis.json> <review.json>]...
```

Pass every `changed`/`new` repository from the frozen inventory through `--expected-repository`; missing, extra, duplicate, checkout-mismatched, source-mismatched, or digest-mismatched package identities block the command. Treat its output as authoritative:

- A reviewer `revise` package whose findings all target existing `claims.<claim_id>` values keeps every unchallenged claim when at least one remains. It becomes `write-ready` with `review_disposition: partial-accept`, explicit `accepted_claim_ids` and `rejected_claim_ids`, and no retry. A non-claim finding or a review that challenges every claim makes the package `cursor-ready` with decision `review-rejected`; it appears in `fallback_repositories` and does not block another package.
- A reviewer `blocked`, missing, malformed, stale, or otherwise contract-invalid review, and an exact deterministic analysis fallback, become `cursor-ready` with decision `inspection-limited`; none starts another agent pass or blocks another package.
- An accepted `new` package with result `no-change` and no write node becomes `cursor-ready` with decision `no-durable-node`; only this accepted case asserts that no durable node was selected.
- Every accepted or partially accepted package with a durable write is `write-ready`; the command groups only nodes referenced by `accepted_claim_ids` through transitive write-node overlaps in stable `write_groups`.

The command atomically persists its complete authoritative result to `<gate.json>` before also emitting it on stdout. If that file is absent, invalid, or not the result of the current command, no vault write is allowed; lost stdout is not a reason to rerun the gate. It emits `acknowledgements`, sorted by repository, with only `repository`, full `new_oid`, and one closed decision: `no-durable-node`, `review-rejected`, or `inspection-limited`. For each emitted write group, one coordinator reconciles its accepted packages independently, orders them by repository, groups accepted claims by canonical node basename, preserves conflicts, and selects lifecycle actions under [node-selection.md](node-selection.md). Complete when every repository has one deterministic disposition and every emitted write group or cursor has one reconciled decision without turning a contradiction into an inference. The gate runs once: semantic disagreement reduces what can be published, but it does not start another extraction, review, or batch gate and creates no content-level pending repository.

### 6. Revalidate and write once

Re-observe every production branch in a `write_group` immediately before writing and run the same full `git-change-manifest.py check --repo ... --manifest ... --scaffold ... --analysis ... --current-ref <explicit-production-branch-ref>` command again. Any stale head, contract failure, or operational failure leaves that group unwritten and requires a new explicit baseline for the group.

The same coordinator projects each ready group once, in stable group, repository, and node-basename order. Apply only claims listed in the gate's `accepted_claim_ids` that pass the production-evidence gate; `rejected_claim_ids` never contribute new prose, relationships, propagation, or nodes. A repository with no accepted claims may retain an existing mention but must not gain a net-new mention within any patched file; preserve prior durable relationships unless accepted evidence contradicts them. Durable notes describe only the accepted technical knowledge and its evidence limitations: never mention the sync run, review, gate, claim acceptance/rejection, package disposition, or cursor. When a glossary or MOC already states where an identifier is authoritative, distinguish that authority from any accepted operational replica or state store instead of merging both into one unqualified location claim. Update `commit-analizado`, `fecha-analisis`, `rama-analizada`, and `ultima-auditoria` atomically within the group. An accepted traceability-only result has zero claims and blockers, sets the repository node to `update`, sets every other node to `no-change`, and changes only those four fields. A package without accepted claims never changes those fields.

Before the first durable write, persist the complete projection as one unified patch and validate it mechanically:

```bash
python3 -B 90-Meta/git-change-manifest.py validate-write --gate <gate.json> --patch <projected.patch>
```

Only a `status: pass` projection may be applied. The validator requires a completed batch gate, restricts Markdown paths to the gate's `write_groups`, permits the acknowledgement file only when the gate emitted cursors, blocks synchronization-process language, and blocks net-new references per patched file to repositories without accepted claims; a rewritten line may preserve an existing mention. Validate the patch once; do not repair and revalidate it inside the same run.

Revalidate every `cursor-ready` package against its unchanged `new_oid`, then merge its record into `90-Meta/.sync-acknowledgements.json`. The file has exactly `version: 1` and a repository-sorted `repositories` array; each item has exactly `repository`, `branch`, `analyzed_sha` (12 lowercase hexadecimal characters), `decision` (`no-durable-node`, `review-rejected`, or `inspection-limited`), and `analysis_date` (`YYYY-MM-DD`). `no-durable-node` cannot coexist with a repository note. The two operational decisions may coexist only with a note whose semantic `commit-analizado` is older; remove the cursor when a later accepted package brings the note to that SHA. A cursor contains no technical claim, relationship, free-form reason, source finding, or secret.

Extractors and reviewers never write the vault. Complete when one writer applied every still-valid accepted group and cursor.

## Execution and closure

For the Framework's isolated blind test only, a newly created Git fixture below the recorded temporary run root may be passed directly to the helper as its checkout. This exception does not resolve an ecosystem repository, supply production evidence, or authorize any vault write; real synchronization always uses the semantic workspace resolution above.

- Resolve repositories by remote identity and keep existing source checkouts read-only. Acquire missing Git objects only through configured managed-clone authority; otherwise stop for the exact missing authority instead of modifying a source checkout.
- Every repository receives exactly one extraction and at most one review. Represent them as one extractor card and, when applicable, one reviewer card. Use native child contexts for independent repository packages when available. If unavailable, execute the same bounded package-worker flow sequentially with fresh isolated contexts. Do not emulate an agent hierarchy with shell processes.
- Reconcile accepted claims directly from the deterministic `write_groups`; do not invoke the single-unit workflow again and do not launch a second multi-unit analysis pass. Node selection and cross-repository synthesis happen once in the coordinator before the single write.
- Use read-only GCP only when needed to establish durable structural metadata, and record its project and verification date. Interpret deployable repositories' versioned Actions and deployment artifacts during their one extraction, not as a later analysis pass.
- Run the inventory again. Every inspected repository must become `current` or the exact `acknowledged-*` status selected by the gate; otherwise its completed cursor was not persisted.
- Compare every source `git status --short` byte-for-byte with its initial snapshot. After reporting, remove only the exact recorded temporary run root; never delete or clean a source checkout.

## Verification and output

Run only the closure checks relevant to the synchronization effects: the second inventory, `audit-vault.py`, `verify-links.py`, Obsidian unresolved/orphan checks when available, and `validate-bases.py` only when a Base, schema, or referenced property changed. Distribution-development suites are outside a cell synchronization; do not run unrelated test suites as synchronization phases.

Report the inspected inventory, changed/new/lifecycle repositories, modified notes, accepted traceability-only updates, operational cursors, no-node acknowledgements, propagated relationships, evidence limitations, and observed checks. Use `pending` only when the exact Git evidence could not be resolved or read, branch freshness was lost before writing, or the vault result could not be persisted. Reviewer `revise` or `blocked`, credentials, deployment limitations, source quality, and remediation observations produce cursor decisions rather than pending work. Never present credential, deployment, source-quality, or remediation observations as work required from a source repository.

## Completion criterion

The run always terminates after its single batch gate. It never starts a correction pass or repeats an extraction or review.

Synchronization is complete when every inventory item is classified, every non-current state has an evidence-backed decision persisted against its inspected commit, each analyzed deployable with an existing or selected repository node has a resolved or explicitly limited deployment row for every observed environment, all documentation effects were propagated, the second inventory contains no repeated `changed` or `new` item from the same inspected SHA, and the relevant closure checks pass. Zero durable knowledge is a successful completed result.
