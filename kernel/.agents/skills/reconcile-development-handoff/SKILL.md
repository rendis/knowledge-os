---
name: reconcile-development-handoff
description: Reconcile an active cell development handoff with its implementation worktree, branch and pull-request evidence, current Jira story, and directly dependent blocked Jira stories, then return the normalized result to the source investigation. Use when development changed or completed, when handoff definition changes must be checked, or before closing or deactivating a completed handoff.
---

# Reconcile development handoffs

Treat reconciliation as a three-way comparison between the immutable handoff baseline, `implementation-updates.md`, and current repository/Jira evidence. Coordinate the comparison; let `manage-investigation` remain the sole writer of the source case. Do not create a `return/` directory or any parallel reconciliation artifact.

## 1. Bind the exact handoff

1. Resolve the canonical vault through `map-ecosystem` and keep its interrogation branch read-only.
2. Load [references/reconciliation-contract.md](references/reconciliation-contract.md) and [references/downstream-context.md](references/downstream-context.md).
3. Require one exact worktree path. Run the `manage-development-handoff` validator for that path and repository remote; stop on inactive or invalid state.
4. Read the entire active `handoff.yaml`, referenced history event, `START.md`, `jira.md`, `context.md`, `scope.md`, and `implementation-updates.md`. Bind the investigation ID, story ID, Jira key/site, normalized repository remote, family, and revision from those files.

Complete this stage when one valid active identity and every comparison input are bound; otherwise return the exact missing or conflicting invariant.

## 2. Inspect implementation and delivery evidence

1. Keep the worktree read-only during reconciliation. Inspect its current branch, `HEAD`, status, changed paths, commits and diff against the observed delivery base, relevant contracts/configuration, and the tests or checks actually run.
2. Distinguish local branch state from an observed remote ref. Obtain the remote branch and SHA through a current read-only Git or repository-host query when available.
3. Inspect the pull request when one exists and record its head, base, state, checks, merge state, and URL. Record deployment separately; neither a branch nor a pull request proves production.
4. Separate implementation evidence from conclusions. Do not infer a test result, remote publication, merge, or deployment from filenames, local refs, or intent.

Complete this stage when each local, remote, pull-request, merge, test, and deployment claim has observed evidence or an explicit `not-observed` limitation.

## 3. Inspect Jira and direct dependents

1. Use `manage-operational-workflow` in its read-only **Advise** branch with the Jira work-item reference and the versioned Jira validation standard.
2. Re-read the source story, its material fields, relevant comments/evidence/attachments, current update timestamp, and every typed issue link.
3. Select directly dependent Jira stories only through the one-hop rule in the downstream contract. Resolve the exact observed link type and direction; never infer dependency from text, hierarchy, shared labels, or proximity.
4. Read each selected dependent story far enough to determine the context and contracts it needs from this implementation.

Complete this stage when the source story is current and every direct dependent is either inspected and classified or names the exact access/evidence blocker.

## 4. Close changelog coverage

Apply the comparison matrix in the reconciliation contract. Match every material definition delta found in repository, delivery, or Jira evidence to one or more `UPD-NNN` entries. If any material delta is absent from the changelog, stop reconciliation and return the exact missing entry content required by the managed handoff rule; do not reconstruct it silently in the investigation.

Complete this stage only when every material delta is logged and consistent, or reconciliation is explicitly blocked with the unmatched evidence.

## 5. Build downstream-ready context

Create one in-memory source card and one card per directly dependent Jira story using the downstream contract. Include contracts, APIs, events, data, configuration, compatibility, verification, what can start now, remaining gaps, and exact branch/PR evidence. Do not create durable files in the worktree.

Complete this stage when each direct dependent is `ready`, `partial`, `still-blocked`, or `not-applicable` with evidence and actionable context.

## 6. Reconcile the source investigation

Hand the normalized reconciliation context to `manage-investigation` through its **Reconcile development** route. Let that workflow update the case snapshot, stable registers, draft synchronization, readiness, learning assessment state, and History. Keep branch, pull-request, Jira, and implementation claims in future/undeployed state until a separate post-deployment `map-ecosystem` audit proves production.

Complete reconciliation when the case validates against its record contract, every observed material change and dependent card is represented, no unlogged delta remains, and no Jira, source-code, Git remote, pull-request, deployment, or technical-vault write was performed.

## Authority

Keep handoff, repository, Git remote, pull-request, Jira, and deployment inspection read-only. Do not edit source code, append the changelog on behalf of implementation, change Jira, commit, push, create or update a pull request, deploy, or write the technical vault. The only durable write belongs to `manage-investigation` inside the already selected local case.
