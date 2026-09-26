# Bootstrap evals

Installation checks of `kos init`, `adopt`, `doctor` and `kernel update`; not installed into a cell. Run them with `make test-installer` after `make release`
(native installation tests fail rather than skip when the release is stale).

`criteria.md` states the bars the harness enforces. The behavior trials below need a fresh agent in a
disposable installed vault; they are run when the corresponding behavior changes:

- [onboarding decisions](onboarding-decisions.md) — what `onboard-cell` and `onboard-developer` propose, ask and record;
- [personal instructions](personal-instructions.md) — routing with and without `AGENTS.personal.md`;
- [distribution changes](distribution-changes.md) — how a vault session proposes a change to the kernel.
