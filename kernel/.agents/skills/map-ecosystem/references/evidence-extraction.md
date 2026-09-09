# Extract knowledge with bounded impact

Use this contract for single-unit, multi-unit and synchronization extraction. The transport and publication gates remain in their respective recipes.

## Select the evidence boundary

For a new repository, map its entrypoints, contracts, behavior, data and deployment/configuration sources. For a known repository, start with the frozen diff and existing canonical notes. Follow changed symbols and contracts into unchanged files at the same exact revision when needed to explain their effect. A diff selects where investigation starts; it is not the complete evidence boundary.

Follow directly affected publishers/consumers, callers, storage and flow participants. Use graph neighbors and targeted identifier searches together: a missing graph edge does not prove a missing dependency. Freeze each additional source's remote and revision before relying on it. Cross-repository observations remain context until their owning package supplies source evidence; select an affected unchanged repository for an explicit audit when its own claims need revision.

Reuse analysis only while its source revisions, evidence profile, selected context and unresolved-evidence state remain valid. A changed dependency, newly accessible evidence or changed profile can justify an explicit same-commit audit. Record the reason and affected scope; do not reset unrelated cursors or repeat a completed scan just because publication failed.

Stop expanding when each changed contract has an explained effect, direct participants have a decision, and no unanswered question can change a proposed claim or node action. A blocked dependency limits only the claims that need it.

## Ask questions appropriate to the cell

The scaffold's six dimensions are the stable core: inputs, outputs, data, business behavior, infrastructure and deployment. Derive concrete probes from observed code and the procedures/capabilities configured in `instance.yaml`; the coordinator supplies relevant procedure notes as `vault_context`. Do not load every adapter or assume a cloud provider, retail vocabulary, environment name or credential mechanism.

Examples: a message publisher needs payload, routing and failure behavior; a batch file job needs file contract, schedule and recovery; a local library needs callers, exported behavior and state effects. Investigate only applicable technology details. Mark unsupported dimensions `not-applicable` with a reason after a bounded source check. An applicable behavior not found is `not-observed`; unreadable required evidence is `blocked`. Keep the frozen scaffold question keys unchanged and put concrete answers in reasons, claims and evidence anchors.

## Reconcile against the vault

Compare the evidence with the existing note and directly affected relationships. For each affected fact decide whether it is unchanged, new, corrected, contradicted or explicitly removed. Keep valid unrelated facts and relationships, including their provenance. Absence from this extraction, a missing search result or an inaccessible source is not removal evidence. A removed file alone does not prove a behavior disappeared: inspect replacements, callers and configuration before retiring its claim or node.

Use existing node actions and atomic claims; do not create another persisted fact register. State material preservation/removal decisions in node reasons and the proposed diff. Keep conflicting environment values separately scoped until evidence resolves them. Follow `90-Meta/evidence-policy.md`: source behavior and effective deployment are distinct assertions.

## Review both assertions and omissions

Before reading the candidate claims in detail, the independent reviewer derives the few material questions raised by the changed contracts and existing notes. Then check the candidate against exact evidence: supported claims, missing rules, affected consumers, absent-versus-zero or units/state semantics when relevant, and unjustified removals. This is a bounded review of changed meaning and directly dependent knowledge, not another full repository scan.

One initial review may trigger at most one targeted correction. For synchronization packages, the coordinator uses `sync-correction.py` before checkpointing. Ordinary single-unit or multi-unit mapping uses one focused revision of its existing draft without creating synchronization artifacts. Repair only findings and connected claims/dependencies, preserve the original attempt, and obtain a fresh review of the repaired artifact. A remaining defect stays explicit as limited/rejected or a safe partial outcome; never acknowledge rejected work as current documentation.

## Verify the published meaning

Inspect the final note diff against accepted claims and preservation decisions. Every new or corrected assertion needs evidence; every removed fact or relationship needs an explicit justified replacement or removal. For each changed business rule or contract, answer a concrete source-derived question from the resulting note and check it against the inspected source. Structural checks and valid links supplement this semantic check; they do not establish completeness.
