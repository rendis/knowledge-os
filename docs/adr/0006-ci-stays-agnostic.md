# The organization's CI stays agnostic of kos

**Status**: accepted (v0.22.24); supersedes the remote gate of ADR 0004

**Context**: ADR 0004 had the kernel install `.github/workflows/knowledge-gates.yml`, which downloads `kos` from
this distribution's releases on every push and pull request and runs `sync verify` on the organization's runner.
A cell's repository belongs to its organization: its pipeline cannot depend on a binary fetched from an external
repository, and the vaults' CI is not ours to extend with that dependency.

**Decision**:

- The kernel ships no CI workflow. `kos kernel update` retires `knowledge-gates.yml` from cells that have it (the
  lock records it), and `instance.yaml` `ci.runner` no longer has an effect.
- The gates stay in `kos` and run locally in the sync flow: `sync verify` before `sync finish`, and
  `sync verify --base <previous remote commit> --allow-no-change` before pushing a base branch.
- Anything the kernel adds to a cell's CI in the future must run without `kos` or any other binary from this
  distribution.

**Considered options**: reimplementing the gates in the workflow with standard tools (a second implementation
that drifts from `kos`); keeping the workflow opt-in per cell (it still puts an external binary in the pipeline).

**Consequences**: a knowledge change committed outside a reviewed sync branch is no longer refused by the remote;
it is caught the next time anyone runs `sync verify` on that range. Enforcement depends on the sync flow being
followed, as before ADR 0004.
