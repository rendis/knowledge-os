# Repository reference branches

`instance.yaml` owns the portable mapping policy under `sources.reference_branches`:

```yaml
sources:
  reference_branches:
    payments-service: trunk
    settlement-worker: release/stable
```

Keys are exact repository basenames, matching inventory identity and repository-note aliases. Values are Git branch names, without `refs/heads/` or remote prefixes. An explicit entry selects only that branch: a missing ref is a blocker, never permission to fall back. Unlisted repositories retain the existing ordered selection: `main`, then `master` only when `main` is absent. The setting does not authorize access to another repository.

`configure-workspace` owns changes to this cell configuration under existing authorization. Merge only this portable declaration into the existing `sources` section, preserving unrelated fields; remove an entry to restore the legacy fallback. Validate with `python3 -B 90-Meta/cell-config.py --vault-root <vault> status`, which loads the complete instance contract. This does not authorize manual edits to local workspace configuration. `update` never overwrites `instance.yaml`.

Inventory resolves the selected branch to its observed SHA. Repository notes record a syntactically valid observed branch and the observed 12-character commit. Historical note metadata remains valid after a policy change; inventory marks a branch or SHA mismatch as `changed`, allowing reanalysis rather than rejecting the note. Synchronization freezes an exact source OID and checks that the selected ref still resolves to it before closing a package. `close-package --production-ref` retains its existing flag name for compatibility; it accepts `refs/heads/<branch>` or `refs/remotes/origin/<branch>`. It loads policy from its installed vault; use `--root <vault>` when running the distribution helper for a specific cell.

A branch name does not establish deployment or productive execution. Keep code/configuration observations separate from claims requiring runtime evidence. Existing acknowledgement artifacts remain readable after a policy change; inventory only treats them as current when both branch and SHA match the newly selected reference. A newly accepted `no-documentation-change` acknowledgement can retain the historical note baseline. Syntax validation alone never proves that the current reference was analyzed.
