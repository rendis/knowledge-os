---
name: reconcile-development-handoff
description: Pull evidence from a registered development worktree, reconcile one selected handoff with its current work item and direct dependents, and update the case through its sole writer while preserving worktree lifecycle state.
---

# Reconcile development handoffs

Treat case reconciliation as a vault-owned pull: select an exact worktree from the source investigation, then compare the immutable handoff baseline, `implementation-updates.md`, and current repository/work-item evidence. `manage-investigation` remains the sole writer of the source case. `ACTIVE.yaml` remains the worktree-local lifecycle source of truth; the worktree agent owns its transitions, so reconciliation reads it but does not change it. Do not ask the repository to create a callback, status packet, `return/` directory, or parallel reconciliation artifact.

## 1. Bind the exact handoff

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Continue only with one canonical `VAULT_ROOT`.
2. Load [references/reconciliation-contract.md](references/reconciliation-contract.md), [references/downstream-context.md](references/downstream-context.md), and `../../../90-Meta/work-item-evidence.md`.
3. Require one selected source investigation and story. Use `../manage-investigation/scripts/investigation-case.py --root <VAULT_ROOT>/investigations load --id <id>` to read the entire public case and discover its optional private overlay by ID. Keep private provenance restricted and supplementary. Select exactly one current `DH-NNN` entry and bind its tracker identity, repository remote, branch, handoff ID, family, and revision. A local path or completion claim cannot replace this register.
4. Invoke the `manage-development-handoff` **Resolve branch** route with the registered remote and branch. It derives a candidate from local configuration and validates repository and branch through Git. Stop with an explicit unavailable result when the worktree is absent; a derived path alone is not evidence. Then invoke **Validate** on the resolved path and select the exact handoff identity.
5. Read the selected `handoff.yaml`, referenced history event, `START.md`, `work-item.md`, `context.md`, `scope.md`, and `implementation-updates.md`. Require their investigation, story, work-item, repository, family, and revision identities to match both the case and registry entry.

Complete this stage when one valid selected identity, its current state, one local closure fingerprint, and every comparison input are bound; otherwise report the exact missing or conflicting invariant.

## 2. Inspect implementation and delivery evidence

Apply [evidence-driven-analysis](../evidence-driven-analysis/SKILL.md) to disputed claims or contradictory observations, keeping this reconciliation as the owner. Its conclusions feed the comparison contract; they do not bypass changelog coverage, snapshot validation or the case writer.

1. Keep the worktree read-only during reconciliation. Inspect its current branch, `HEAD`, status, changed paths, commits and diff against the observed delivery base, relevant contracts/configuration, and the tests or checks actually run.
2. Distinguish local branch state from an observed remote ref. Obtain the remote branch and SHA through a current read-only Git or repository-host query when available.
3. Inspect the pull request when one exists and record its head, base, state, checks, merge state, and URL. Record deployment separately; neither a branch nor a pull request proves production.
4. Separate implementation evidence from conclusions. Do not infer a test result, remote publication, merge, or deployment from filenames, local refs, or intent.

Complete this stage when each local, remote, pull-request, merge, test, and deployment claim has observed evidence or an explicit `not-observed` limitation.

## 3. Inspect the work item and direct dependents

1. Apply `90-Meta/work-item-evidence.md` directly with the registered tracker and reference. This is a read-only evidence operation, not an operational workflow or `.operations/` run.
2. Re-read the source work item, its material fields, relevant comments/evidence/attachments, current update timestamp, and every available typed relationship.
3. Select directly dependent work items only through the one-hop rule in the downstream contract. Resolve the exact observed relation and direction; never infer dependency from text, hierarchy, shared labels, or proximity.
4. Read each selected dependent story far enough to determine the context and contracts it needs from this implementation.

Complete this stage when the source story is current and every direct dependent is either inspected and classified or names the exact access/evidence blocker.

## 4. Close changelog coverage

Apply the comparison matrix in the reconciliation contract. Match every material definition delta found in repository, delivery, or work-item evidence to one or more `UPD-NNN` entries. If any material delta is absent from the changelog, stop before changing the case and report a deficiency card containing the observed delta, affected baseline, evidence, materiality reason, and missing coverage. Do not author the repository's `UPD-NNN` entry or prescribe exact changelog text.

Complete this stage only when every material delta is logged and consistent, or reconciliation is explicitly blocked with the unmatched evidence.

## 5. Build downstream-ready context

Create one in-memory source card and one card per directly dependent work item using the downstream contract. Include contracts, APIs, events, data, configuration, compatibility, verification, what can start now, remaining gaps, and exact branch/PR evidence. Do not create durable files in the worktree.

Complete this stage when each direct dependent is `ready`, `partial`, `still-blocked`, or `not-applicable` with evidence and actionable context.

## 6. Reconcile the source investigation

Immediately before the case write, invoke **Validate** again and require the same identity and closure fingerprint captured before evidence inspection. If either changed, discard the assembled local comparison and restart from the new snapshot; do not write a stale reconciliation.

Pass the normalized vault-side context and exact closure fingerprint directly to `manage-investigation` through its **Reconcile development** route in the same interaction. Let that workflow update only the affected case snapshot, stable registers, draft synchronization, readiness, learning assessment state, and History, then reevaluate the global investigation objective and closure criteria. Keep branch, pull-request, work-item, and implementation claims in future/undeployed state until a separate post-deployment `map-ecosystem` audit proves production.

When implementation evidence shows that the selected handoff is a production candidate or deployed, compare that evidence with its observed lifecycle state. Record the evidence in the case and preserve `ACTIVE.yaml`. If the state is inconsistent, report the exact mismatch for the worktree agent to resolve; reconciliation never invokes **Set state**.

Do not close, reopen, block, or unblock the investigation automatically. A terminal `ACTIVE.yaml` handoff state, merged branch, or observed deployment may satisfy one objective-specific criterion but is never the closure decision itself. If the reevaluation suggests a lifecycle action, report the evidence and let `manage-investigation` apply a separate explicit current-snapshot transition.

Complete reconciliation when the case validates, every observed material change and dependent card is represented, no unlogged delta remains, global closure criteria were reevaluated without an automatic status change, and any lifecycle mismatch is reported exactly. No tracker, source-code, Git remote, pull-request, deployment, worktree lifecycle, or technical-vault write is performed.

## Authority

Keep handoff content, lifecycle state, repository, Git remote, pull-request, tracker, and deployment inspection read-only. Do not edit source code, append the changelog on behalf of implementation, change the tracker, commit, push, create or update a pull request, deploy, or write the technical vault. Durable case writes belong only to `manage-investigation`; lifecycle transitions belong to the worktree agent.
