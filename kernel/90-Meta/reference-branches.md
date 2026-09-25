# Repository reference branches

`instance.yaml` owns the portable policy under `sources`, chosen during onboarding:

```yaml
sources:
  reference_branch_order: [main, master]   # tried in order; the first that exists is the reference
  reference_branches:                      # exceptions, exact per repository
    payments-service: trunk
    settlement-worker: release/stable
```

`reference_branch_order` applies to every repository without an explicit entry: the first branch of the list that exists in the repository is its reference. It defaults to `main`, then `master`; a team whose reference code lives elsewhere lists it first (for example `[develop, main, master]`), and a cell whose repositories mix `main` and `master` keeps both. Keys of `reference_branches` are exact repository basenames, matching inventory identity and repository-note aliases; values are Git branch names, without `refs/heads/` or remote prefixes. An explicit entry selects only that branch: a missing ref is a blocker, never permission to fall back. The remote's default branch (`origin/HEAD`) is not the reference unless the policy selects it. The setting does not authorize access to another repository.

`configure-workspace` owns changes to this cell configuration under existing authorization. Merge only this portable declaration into the existing `sources` section, preserving unrelated fields; remove an entry to restore the legacy fallback. Validate with `<VAULTCTL> config status --vault "<VAULT_ROOT>"` (selected through `use-vault-cli`), which loads the complete instance contract. This does not authorize manual edits to local workspace configuration. `update` never overwrites `instance.yaml`.

Inventory and `discover` apply the same selection. Inventory resolves the branch to its observed SHA on GitHub; `discover run` and `discover check` scan the local remote-tracking ref (`origin/<branch>`, then the local branch), so fetch before a sync when the checkout may lag. A repository where no branch of the order exists and without an entry is scanned at its remote default or `HEAD` and reported as `reference-branch` pending until the branch is declared. Repository notes record the observed branch and the 12-character commit. Historical note metadata remains valid after a policy change; inventory marks a branch or SHA mismatch as `changed`, allowing reanalysis rather than rejecting the note. Existing acknowledgement artifacts remain readable after a policy change; inventory treats them as current only when both branch and SHA match the newly selected reference.

A branch name does not establish deployment or productive execution. Keep code/configuration observations separate from claims requiring runtime evidence.
