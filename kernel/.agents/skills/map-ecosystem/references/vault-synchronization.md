# Synchronize the vault

## Authority and outcome

Synchronize only on an explicit vault-update request; report mode is read-only. Source repositories are read-only evidence surfaces. A successful repository decision publishes complete reviewed note images or records an accepted no-documentation-change/no-durable-node acknowledgement. A limited or rejected attempt remains unresolved, leaves the successful synchronization cursor unchanged, and is reported with its reusable candidate, evidence and exact finding.

The coordinator owns inventory, exact source binding, baseline capture, candidate location, deterministic commands, publication state and closure. One author analyzes each repository delta and writes the complete resulting note candidate. One fresh reviewer checks those final images under [final-note-review.md](final-note-review.md). There is no separate semantic claims package, finalizer, gate, projection or second note review in a new run.

Before freezing context, use [investigation-context.md](investigation-context.md) to locate relevant case/change evidence. Supply only authorized, sanitized context. Correspondence, deployment observations and other non-Git evidence use the external-only documentation path after source sync; preserve source metadata when the repository was not re-analyzed.

## Scope and batching

Apply [repository-map.md](repository-map.md). The result must answer P1–P5 at main-flow depth, preserve valid existing knowledge and stable `connection.*` anchors, and state evidence limits precisely. For an existing map, start from the frozen source delta and current complete notes. Inspect unchanged source only where it explains changed meaning, a possible main-flow omission or preservation. For an initial map, inspect the bounded main flows directly.

Select small independent repository groups so a slow repository does not hold unrelated results. Follow repository-map.md's pilot time and usage limits. Persist each completed candidate/review/publication promptly. An instruction, rendering, formatting or publication change never justifies another source analysis.

## Coordinator recipe

1. **Inventory and bind exact sources.** Run `<cli> inventory --vault "<vault>" --format markdown`. For each selected `changed` repository bind the recorded commit and production head; for `new`, bind the empty-tree baseline and production head. Resolve the checkout by configured remote identity, production ref, full old/new OIDs and initial `git status --short`. An unresolved identity or unreadable frozen revision blocks that repository only.
2. **Freeze the documentation baseline and questions.** Read every affected complete note, its relevant backlinks and current connection anchors. Record the existing P1–P5 answers and useful facts, the source delta expected to change them, exact evidence anchors and explicit limitations. Use associated investigation context only where it helps interpret the frozen delta.
3. **Author the final-note candidate once.** Give one author the exact source bindings, current complete note images, affected P1–P5 questions, evidence profile and candidate directory. The author inspects the source delta once, follows relevant unchanged dependencies only as needed, and writes complete result images directly. It preserves unaffected supported knowledge and provenance. A no-change result reproduces the baseline bytes and explains from the inspected delta why no durable text changes; a new repository with no eligible durable node can close only through an accepted review. The author writes no parallel claims ledger or large semantic JSON.
4. **Preflight and freeze.** Run available cheap deterministic checks on the candidate and resolve their exact reported mechanical issues. Then freeze the complete images, evidence and source tuples with `sync review freeze` as defined by [final-note-review.md](final-note-review.md). Keep credentials, raw logs and production rows out of the artifact.
5. **Review changed meaning once.** Dispatch one fresh reviewer with the frozen manifest, complete candidates, original notes and bound evidence. It checks affected P1–P5, changed/removed values, connectors, citations, pending close conditions and preservation. On `revise`, correct only the cited candidate text from already available evidence, refreeze, and re-review the materially changed meaning and its preservation boundary. Do not repeat repository extraction. Stop the affected repository when evidence or scope is insufficient.
6. **Check deterministically.** Run `sync review check` with repeatable `--checkout <repository> <checkout>` for every bound source and follow its returned issues and locations. Candidate bytes do not change under an accepted review: after any correction, refreeze and obtain targeted semantic re-review of the changed portion and preservation boundary before checking again. A deterministic failure never triggers source re-analysis.
7. **Publish exact reviewed bytes.** Run resumable `sync review publish` with the exact arguments required by the installed CLI. It applies only candidate bytes bound to the accepted review and stores publication state under the sync state root. Identical accepted baseline bytes record `no-documentation-change`; a later accepted write for that repository/commit retires the matching acknowledgement. Resume an interrupted publication with the same manifest, review and candidate.
8. **Verify closure.** Re-run inventory, compare source statuses with their initial snapshots, run `<cli> audit --vault "<vault>"`, `<cli> check links --vault "<vault>"`, available Obsidian checks and `<cli> check bases --vault "<vault>"` when applicable. Inspect the exact vault diff and publication receipts.

## Failure routing

- **Source changed before publication:** invalidate only that repository candidate and analyze its new delta. Preserve unrelated completed repositories.
- **Destination changed:** keep the observed user bytes, rebuild only affected complete candidates against the new baseline, then review the changed merged meaning.
- **Semantic finding:** edit only affected candidate prose from available evidence and target re-review to changed meaning. A repeated defect, unavailable evidence or required scope expansion stops that repository as unresolved. Preserve its candidate, evidence and finding; do not advance its successful cursor or automatically repeat source analysis merely because later inventory still reports `new` or `changed`.
- **Mechanical finding:** repair the reported candidate location, refreeze, and obtain targeted semantic re-review of the changed bytes before checking again. Do not reopen source analysis.
- **Publication interruption:** resume the recorded publication; reuse accepted artifacts and do no semantic work.
- **Legacy active run:** follow [synchronization-state-machine.md](synchronization-state-machine.md) and its stored next command. Do not convert or rewrite its immutable package, gate or projection artifacts.

## Documentation limits

Publish only evidence-backed content under canonical knowledge roots. Do not put review, receipt or synchronization-process language in technical notes. An acknowledgement contains only its source identity, accepted outcome and integrity metadata; it carries no technical claims or sensitive values. `no-documentation-change` means the reviewed delta does not change the retained note baseline. `no-durable-node` means the reviewed new repository does not qualify for a node. A rejected or limited attempt creates no acknowledgement and never establishes current documentation.

## Completion criterion

A successful run completes when every selected repository has an accepted publication or acknowledgement, exact destination hashes and durable receipts exist, the second inventory contains no repeated uninspected SHA, applicable vault checks pass and source checkouts remain unchanged. If any repository remains limited or rejected, stop the attempt as unresolved and report its exact gap, retained baseline and reusable candidate/evidence location; do not describe the campaign as complete. Resume by correcting that localized finding from the existing artifacts unless source drift makes a new delta analysis necessary. Before declaring a wider campaign complete, apply [mapping-completion.md](mapping-completion.md).
