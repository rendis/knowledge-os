# Main campaign r01 trace audit

Snapshot of completed outcomes only; no final-answer semantic scoring. Evidence: `${INITIAL_CAMPAIGN}/evidence/<case>/r01-<arm>/<phase>/events.jsonl`. Command counts use `item.completed`, avoiding duplicate started events.

| Outcome | Completed commands / nonzero exits by phase | Graph query calls by phase | Resolved before domain content |
| --- | --- | --- | --- |
| ambiguous/r01-alternative | parent 7/2; review 6/1 | none | parent yes; review yes |
| ambiguous/r01-baseline | parent 13/3; review 6/1 | parent 2 | parent yes; review yes |
| dependencies/r01-alternative | parent 15/0; review 3/0 | parent 4 | parent yes; review yes |
| dependencies/r01-baseline | parent 10/1; review 5/0 | parent 3 | parent yes; review yes |
| false-premise/r01-baseline | parent 8/1; recheck 4/0; repair 7/1; review 7/0 | review 1 | parent no domain content observed; recheck yes; repair yes; review yes |
| known-file/r01-alternative | parent 6/0; review 2/0 | none | parent yes; review yes |
| known-file/r01-baseline | parent 4/1; review 5/1 | none | parent yes; review yes |
| reuse-investigation/r01-alternative | parent 7/2; review 8/2 | none | parent yes; review yes |
| reuse-investigation/r01-baseline | parent 12/3; review 5/0 | none | parent yes; review yes |
| two-repositories/r01-baseline | parent 15/3; review 7/2 | parent 2; review 1 | parent yes; review yes |

## Material observations

- Known-file baseline and alternative both read the supplied note directly and make no graph query. This is an observed same-route comparison; it establishes no graph-versus-direct advantage. Dependencies baseline and alternative both use graph discovery, with 3 versus 4 parent queries in this snapshot; do not attribute their difference to graph-versus-nongraph routing.
- Every audited phase with domain content has prior resolver output `status: resolved`. Some workers load the vault-resolution reference after executing resolution or after a first domain read (notably known-file baseline parent); resolver-before-domain holds, but full prescribed reference ordering is not uniformly followed.
- Reviewers actually read decisive domain evidence: known-file planner; dependencies planner/topic/ingest/UI; false-premise review planner/Dispatch/Maps/topic, recheck additionally routing-api; two-repositories planner/ingest/topic/Framework. Ambiguous baseline reviewer reads planner and lifecycle/helper references; alternative reviewer also reads learning/Dispatch/ingest/API. Neither loads the full case; alternative parent does a content search over investigations. Both reuse reviewers read the learning and helper source, not the investigation case. This audits access only, not answer acceptance.
- Investigation `load` is not feasible under the imposed read-only sandbox: it attempts a transient `.open.lock` directory and returns PermissionError (exit 4). Reuse baseline parent first invokes with the vault root (exit 2), then investigation root (exit 4), inspects helper source and reads the learning; it never reads investigation.md. Ambiguous baseline parent and two-repositories baseline parent/reviewer also hit exit 4. This is fixture execution incompatibility and bounded access failure, not evidence of inefficient graph/reuse. Reuse alternative parent/reviewer and ambiguous alternative parent hit the same exit-4 lock failure. Both reuse arms therefore have the same blocked-case access route, not a demonstrated reuse-versus-reopen contrast. Any future direct-file advantage needs this caveat.
- Other failures are shell quoting/parse errors, nonexistent personal/config paths and runtime executable discovery. Known-file baseline and dependencies/two-repositories baseline recover their initial shell/bootstrap failures. Reuse/ambiguous investigation-load failures do not recover case access. False-premise baseline invokes repair/recheck; this audit does not score the draft or repaired answer.
- macOS `/usr/bin/python3` triggers xcrun cache-write-denial and Xcode diagnostics under the read-only sandbox, frequently despite final command exit 0. Some workers switch to `/opt/homebrew/bin/python3`. These recovery/environment costs are included; large timing differences cannot be attributed solely to routing.
- No nested agent, connector or application calls, shell-launched Codex workers, external content reads or network commands were observed in the audited completed command traces. Resolver internally consults the disconnected Obsidian stub. Platform xcrun cache writes and investigation lock writes were attempted and denied; no successful authored mutation is shown and every audited outcome reports changed=[].
- Effective Sol/medium remains unexposed. Installed skills/plugin context produces some harmless shortened-description notices; concurrency/shared cache limit latency and cache causal interpretation. Independent blind semantic review remains pending.

Completed outcomes captured: 10. Outcomes that complete after this snapshot are not covered.
