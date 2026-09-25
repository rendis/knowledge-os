# Glossary

Terms used by the kernel, the CLI and this repository. Prefer them over synonyms.

**Cell** — a team and the systems it owns; it has one vault, created from this distribution.
_Avoid_: tenant, project.

**Evidence contract** — the rules in the vault's `AGENTS.md` that every answer follows: bind the scope,
inspect the source, grade each conclusion (demonstrated, observed within limits, inferred, unresolved),
check the draft, review new conclusions.

**Reference branch** — the branch a repository is read at: its `sources.reference_branches` entry, else the
first existing branch of `sources.reference_branch_order`. Never the remote's default branch by itself.

**Fact** — a connection `discover run` extracted from a repository at an exact commit (topic, event,
endpoint, database, library), with its file and line.

**Platform provider** — one cloud `discover platform` can read (`gcp`, `aws`, `azure`), each with its own
CLI and the developer's login. A cell lists its clouds in `platform.providers`; the core only knows topics,
subscriptions and queues.

**Scope** — the unit a provider captures: a Google Cloud project, an AWS `<account>/<region>`, an Azure
subscription. Facts and pending items name it as `<provider>:<scope>`.

**Platform snapshot** — the read-only listing of one scope (`90-Meta/discovery/platform/`), versioned; a scope
that could not be read is kept with its status and the command that confirms it.

**Gate** — a deterministic check that blocks: note gates (`discover check`), the case gate
(`investigation check`), `sync verify`. A reviewer's verdict is separate from a gate.

**Sync branch** — `sync/<slug>`, the only way versioned knowledge changes; it carries the review bound to
the exact content.

**Case** — one investigation: the formalized request, its records and a log, in one Markdown file written
through the CLI. Types: *understanding* and *development*.
_Avoid_: dossier, ticket, chat log.

**Record** — an entry of a case with an immutable ID: evidence `E-`, finding `F-`, question `Q-`,
decision `D-`, requirement `R-`, handoff `DH-`.

**Unpublished / published case** — `.investigations/<id>/` (ignored by Git) until published to
`investigations/<id>/` on a sync branch; there is no unpublish.

**Private directory** — `.investigations-private/<id>/`: sensitive notes and scratch a case needs but never
shares; never authoritative.

**Handoff package** — `<case>/handoffs/DH-NNN.md`: one atomic task in one repository (task, changes,
acceptance criteria, required context), self-sufficient without the vault.

**Worktree** — the repository checkout on a task branch; tasks of one milestone (same repository and
branch) share it. Its `.handoff/` holds the task copies and `deltas.md`.

**Delta** — what the implementing agent records for the cell: a definition change, a finding, a decision
or deviation, a question, a verification; each with its evidence.

**Reconciliation** — `handoff reconcile`: commits and deltas since the last mark become case records
through a fixed mapping.

**Absorption** — moving a finding marked for the vault into its note on publication.
