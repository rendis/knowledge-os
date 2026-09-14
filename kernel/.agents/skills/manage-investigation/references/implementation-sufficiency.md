# Implementation sufficiency

`manage-investigation` owns this check before exporting a development package and when assessing whether an existing package is sufficient to implement. The consumer checks package and repository integrity; it does not reconstruct source coverage.

1. Derive implementation questions from the current work item, applicable active decisions, acceptance criteria, registered evidence, and reconciled dependency contracts **before** evaluating the package. Questions drawn only from the package cannot reveal omitted source requirements. Adapt these prompts to the selected story and repository:
   - What must be delivered, and what is outside scope or must remain unchanged?
   - Which rules, formulas, units, exceptions, and agreed constraints determine behavior?
   - Which inputs and outputs are required, where do they come from, and how are they interpreted?
   - What already exists, what must be reused, and which dependency contracts and compatibility requirements apply?
   - What happens in the relevant boundary and failure cases, with which expected results?
   - How will acceptance be verified, and what evidence or test data is needed?
   - What remains unresolved, and why does it block implementation or not?
2. Answer each applicable question using only `work-item.md`, `context.md`, `scope.md`, and precise evidence references available to the recipient. Identify the document section or referenced path/symbol supporting each substantive answer. Conversation, memory, and facts known only from the source investigation cannot supply a missing answer. A heading or a bare link is not an answer; verify that a referenced source is accessible and supports it.
3. Compare those answers with the source-derived requirements. Missing, ambiguous, contradictory, or unsupported answers are concrete gaps even when structural validation succeeds. Carry agreed behavior and essential exceptions in the package itself; use references for supporting technical detail. Include implementation-relevant physical details when they affect correctness, without copying whole repositories or unrelated history. Justify non-applicability; leave legitimate implementation choices open rather than treating every unspecified design detail as a blocker.
4. During an authorized preparation or refresh, fill recoverable gaps from current registered sources and recheck the affected answers. If an essential decision or source remains unresolved, stop that target before materialization and name the question and missing evidence or decision. An assessment-only request reports gaps without modifying the package, case, or repository.
5. Report integrity and implementation sufficiency separately. Summarize coverage and any gaps in the existing workflow response, with answer locations available for inspection; no separate questionnaire file, schema flag, or lifecycle state is required. Claim sufficiency only when every necessary question has a supported answer and remaining limitations are explicitly non-blocking. Reassess affected questions when package content or applicable source requirements change.
