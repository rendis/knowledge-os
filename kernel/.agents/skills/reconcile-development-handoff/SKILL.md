---
name: reconcile-development-handoff
description: Pull evidence from a registered development worktree, reconcile one selected handoff with current Jira and direct dependents, update the case through its sole writer, and mark completed implementation ready for production.
---

# Reconcile development handoffs

Treat reconciliation as a vault-owned pull: select an exact worktree from the source investigation, then compare the immutable handoff baseline, `implementation-updates.md`, and current repository/Jira evidence. `manage-investigation` remains the sole writer of the source case. Do not ask the repository to invoke this skill or create a callback, status packet, `return/` directory, or parallel reconciliation artifact.

## 1. Bind the exact handoff

1. Load `../../../90-Meta/vault-resolution.md` and run `../../../90-Meta/resolve-vault.py` relative to this skill directory. Continue only with one canonical `VAULT_ROOT`.
2. Load [references/reconciliation-contract.md](references/reconciliation-contract.md), [references/downstream-context.md](references/downstream-context.md), and `../../../90-Meta/jira-evidence.md`.
3. Require one selected source investigation and story. Read the entire case and select exactly one current `DH-NNN` entry from **Development handoffs**. Bind its Jira site/key, repository remote, exact absolute worktree path, handoff ID, family, and revision. A repository message, path, branch, or completion claim may help select the case but is not authoritative and cannot replace this register.
4. Invoke the `manage-development-handoff` **Validate** route with the registered remote and worktree path. Select exactly the entry matching the case handoff ID, family, and revision and retain its state and `closure_fingerprint`; other entries may share the worktree but cannot substitute for it.
5. Read the selected `handoff.yaml`, referenced history event, `START.md`, `jira.md`, `context.md`, `scope.md`, and `implementation-updates.md`. Require their investigation, story, Jira, repository, family, and revision identities to match both the case and registry entry.

Complete this stage when one valid active identity, one local closure fingerprint, and every comparison input are bound; otherwise report the exact missing or conflicting invariant.

## 2. Inspect implementation and delivery evidence

1. Keep the worktree read-only during reconciliation. Inspect its current branch, `HEAD`, status, changed paths, commits and diff against the observed delivery base, relevant contracts/configuration, and the tests or checks actually run.
2. Distinguish local branch state from an observed remote ref. Obtain the remote branch and SHA through a current read-only Git or repository-host query when available.
3. Inspect the pull request when one exists and record its head, base, state, checks, merge state, and URL. Record deployment separately; neither a branch nor a pull request proves production.
4. Separate implementation evidence from conclusions. Do not infer a test result, remote publication, merge, or deployment from filenames, local refs, or intent.

Complete this stage when each local, remote, pull-request, merge, test, and deployment claim has observed evidence or an explicit `not-observed` limitation.

## 3. Inspect Jira and direct dependents

1. Apply `90-Meta/jira-evidence.md` directly with the registered Jira site/key. This is a read-only evidence operation, not an operational workflow or `.operations/` run.
2. Re-read the source story, its material fields, relevant comments/evidence/attachments, current update timestamp, and every typed issue link.
3. Select directly dependent Jira stories only through the one-hop rule in the downstream contract. Resolve the exact observed link type and direction; never infer dependency from text, hierarchy, shared labels, or proximity.
4. Read each selected dependent story far enough to determine the context and contracts it needs from this implementation.

Complete this stage when the source story is current and every direct dependent is either inspected and classified or names the exact access/evidence blocker.

## 4. Close changelog coverage

Apply the comparison matrix in the reconciliation contract. Match every material definition delta found in repository, delivery, or Jira evidence to one or more `UPD-NNN` entries. If any material delta is absent from the changelog, stop before changing the case and report a deficiency card containing the observed delta, affected baseline, evidence, materiality reason, and missing coverage. Do not author the repository's `UPD-NNN` entry or prescribe exact changelog text.

Complete this stage only when every material delta is logged and consistent, or reconciliation is explicitly blocked with the unmatched evidence.

## 5. Build downstream-ready context

Create one in-memory source card and one card per directly dependent Jira story using the downstream contract. Include contracts, APIs, events, data, configuration, compatibility, verification, what can start now, remaining gaps, and exact branch/PR evidence. Do not create durable files in the worktree.

Complete this stage when each direct dependent is `ready`, `partial`, `still-blocked`, or `not-applicable` with evidence and actionable context.

## 6. Reconcile the source investigation

Immediately before the case write, invoke **Validate** again and require the same identity and closure fingerprint captured before evidence inspection. If either changed, discard the assembled local comparison and restart from the new snapshot; do not write a stale reconciliation.

Pass the normalized vault-side context and exact closure fingerprint directly to `manage-investigation` through its **Reconcile development** route in the same interaction. Let that workflow update the case snapshot, stable registers, draft synchronization, readiness, learning assessment state, and History. Keep branch, pull-request, Jira, and implementation claims in future/undeployed state until a separate post-deployment `map-ecosystem` audit proves production.

When implementation is complete and approved as the production candidate, continue only after the case write validates. Invoke **Validate** once more, require the same identity and closure fingerprint, then invoke `manage-development-handoff` **Set state** for the exact handoff ID with `ready-for-production` and that fingerprint. Preview its effect, obtain separate authorization, and apply the unchanged token. Any mutation fails closed and preserves every state.

Complete reconciliation when the case validates, every observed material change and dependent card is represented, no unlogged delta remains, and any requested completed implementation is either marked `ready-for-production` or left unchanged with an explicit blocker. No Jira, source-code, Git remote, pull-request, deployment, or technical-vault write is performed.

## Authority

Keep handoff content, repository, Git remote, pull-request, Jira, and deployment inspection read-only. Do not edit source code, append the changelog on behalf of implementation, change Jira, commit, push, create or update a pull request, deploy, or write the technical vault. Durable case writes belong only to `manage-investigation`; state changes belong only to an explicitly authorized `manage-development-handoff` operation after that case write succeeds.
