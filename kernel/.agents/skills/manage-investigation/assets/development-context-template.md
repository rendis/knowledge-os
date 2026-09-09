# Repository implementation context

## Essential context

<Current understanding, evidence, decisions and dependencies necessary for the selected repository. Self-sufficient even when the source is unavailable.>

## Read-only source reference

<Vault-side exporter: include this section only when the vault's existing Git remote is verified. Fill exact values and named relevant sections/notes; omit the section otherwise.>

- Vault remote: <existing verified vault Git remote>
- Investigation ID: <exact case ID>
- Relevant investigation sections: <exact section names needed for this repository>
- Relevant linked vault notes: <exact relative note paths or none>

Resolve the local vault explicitly and verify its remote before bounded read-only inspection. Record missing context and implementation decisions in the worktree's `implementation-updates.md`; source reconciliation belongs to the vault workflow.
