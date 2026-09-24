# Kernel routing thinning — session closure

Historical evaluation record. File paths and tool names below identify the
version tested at the time. The current evidence contract lives in
`kernel/AGENTS.md`; consumer operations use `use-vault-cli`. Retired Python
commands and `response-quality.md` references below are not current setup
or execution instructions.

Recorded 2026-09-17. Distribution-only; not a cell instruction. Local commits only; no push.

## Accepted kernel state

Keep the 0.10.4 always-on thinning and the 0.10.5 census rule.

- Catalog and cross-vault stay off the router. Load `execution-profiles.md` only when dispatching or reconsidering a subagent.
- Query/document stays `map-ecosystem`. Inventory, sync, resume, and `SYNC_PACKAGE_WORKER_V1` stay `synchronize-ecosystem`.
- Investigation read-only preflight does not load `record-contract.md`.
- A filename glob, prefix, or neighbor sample is not a census of a type. Owner and type come from declared note fields. That bar lives in `90-Meta/response-quality.md` (cell-wide, before material conclusions) and in `map-ecosystem` interrogation output. It is not a Cell A special case.

## Local release record

Three live cells: Cell A+IoT, Cell B, Cell C. The combined Cell B+Cell C checkout is legacy and was not updated.

| Version | Distribution | Cell A+IoT | Cell B | Cell C |
| --- | --- | --- | --- | --- |
| 0.10.4: thin router, split sync/investigation | `d14b3ce` | `a286c94` | `c4fdaea` | `73aae51` |
| 0.10.5: glob/sample is not a type census | `7fa2038` | `b742a77` | `64b8e02` | `d4b12c4` |

Release checks for both: 36 bootstrap tests, 14 instance tests. Each consumer `update` plus `doctor --strict` reported `managed_matches_dist: true` and `reproducible_distribution: true`. Cell `instance.yaml`, `00-Home.md`, and notes under `10/`–`70/` were not rewritten. Pre-existing Cell A domain dirt was left uncommitted.

## Evaluation limits

Paired Cell A worktree evals (0.10.3 vs thinning) used instruction-file bytes as a load proxy, not billing tokens. Domain notes were byte-identical (243 files). Quality on the eight shared questions: Grok high 8/8 vs 7/8; Grok medium 8/8 vs 8/8; composer-2.5 8/8 vs 7/8. The candidate fails were the same extra vault-wide IoT-topic census after a correct Pub/Sub rejection. Medium 0.10.4 did not make that claim. 0.10.5 was not re-run on those models.

Instruction-byte savings were largest on investigation and operational paths. Known-file remaining load is the kernel floor (resolve, skill, quality), not a further thinning target. Operational classification remaining load is dominated by domain notes and is accepted for `production-gate`.

Traces lived in ephemeral local directories and are not part of this repository.

## Remaining work

- Remote push of distribution or consumers: not part of this session.
- Optional: one false-premise smoke on high/composer after 0.10.5. Not a release blocker.

The 2026-09-16 [0.10.2 session closure](session-closure.md) remains the record of the efficiency/source-access decision. This file does not reopen it.
