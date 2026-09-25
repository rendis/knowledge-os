---
name: evidence-driven-analysis
description: Method for answers that one decisive source cannot settle — diagnosing a failure, auditing a claim or guarantee, tracing variants of a confirmed cause. The router's evidence contract governs every answer; this skill adds the discriminating checks, and other workflows borrow it as their evidence method.
---

# Evidence-driven analysis

The router's evidence contract applies to every answer: bind, inspect the source, grade each conclusion, check the draft with `discover claims`, and send new conclusions to independent review. This method adds what an uncertain answer needs beyond that: checks that **discriminate** between explanations, so the answer rests on evidence that could have refuted it.

## Method

1. **Frame the competing explanations.** Write the leading explanation and at least one credible alternative, each with the observation that would distinguish them. One decisive source that settles the question ends the method here.
2. **Load the method for the question:**
   - a failure or unexpected behavior → [diagnosis](references/diagnosis.md);
   - a claim, guarantee or control to assess → [audit](references/audit.md);
   - a confirmed cause whose related occurrences matter → [variants](references/variants.md).
3. **Seek the refuting evidence first.** Read the caller, the condition, the consumer, the environment or the revision that would break the leading explanation before collecting more support for it.
4. **Cover absences.** A negative ("nothing publishes to X", "it never fails") needs the query, pagination, environments and revisions that make it exhaustive; otherwise report it as a bounded observation.
5. **Stop** when another read cannot change the graded answer.

Deliver the supported conclusion first, then the decisive sources, the alternatives ruled out and how, and the remaining gaps. Concise prose; no dossier or fixed schema.

## Sources

- **Repositories:** bind the checkout with `<CLI> config locate --vault "<root>" --remote "<remote>"` ([use-vault-cli](../use-vault-cli/SKILL.md)); a similarly named directory is not a binding. Read implementation claims at their actual revision.
- **Existing investigations:** read them through [manage-investigation](../manage-investigation/SKILL.md); published knowledge stays authoritative over private or local notes.
- **Work items, cloud, databases and other systems:** the configured adapter or access procedure and `../../../90-Meta/work-item-evidence.md`, within read-only limits, environments, pagination and permissions. Credentials are never disclosed or persisted.
- **External technical facts:** primary documentation at the relevant version; a documented guarantee is distinct from behavior observed in this system.

## Ownership

When another workflow uses this method, return conclusions, sources and limits to it; its access, write and completion rules apply. A standalone answer creates no record; whether to recommend an investigation case is decided by [manage-investigation](../manage-investigation/SKILL.md#when-a-case-is-worth-it). Analysis never authorizes instrumentation, code changes, operational data processing, deployment or publication.
