# Work-item operations

Load this reference when advising on, drafting, estimating, creating, changing or validating external work items.

## Resolve semantics

Use the selected operational runbook for the configured tracker. It defines work types, granularity, estimation units, templates, field mappings and allowed relationships. A provider name supplies none of those team decisions. Reuse supported source packages; the producing workflow owns their content and sufficiency.

For a local draft or advice, inspect only metadata needed to resolve a material uncertainty. External writes require an exact tracker, destination, action and current destination metadata. Creation checks the destination’s types, required fields and relationship capabilities without requiring an item that does not yet exist. Changes to an existing item also read that item through `90-Meta/work-item-evidence.md`. Missing provider access blocks those writes, not an otherwise supported local draft.

Complete when the intended outcome, source, destination and applicable rules are exact or the missing decision is named.

## Prepare the effect

1. Resolve existing items and related references by observed identity. Search for an equivalent existing outcome before creating a duplicate.
2. Check required fields, available types, hierarchy and relationship semantics against the current destination. Preserve observed dependency direction; a parent-child link is not inherently a dependency.
3. Use the runbook’s granularity and estimation method. Do not invent a conversion between time and points or split work only by implementation layer.
4. For a type or parent change, inspect the current parent, children, relationships and populated fields. Account for retained, transformed or removed values and prevent hierarchy cycles. A conversion preserves the item’s stable identity and history. Do not delete or replace it to simulate an in-place conversion; when the destination cannot preserve identity, report the limitation before any separately authorized migration. Additional changes require matching authorization.
5. Build the operational effect plan with the exact artifact, field changes, relationships and verification. Keep local drafts separate from saving drafts in an external system.

Complete when every proposed change has supported semantics and the operation’s effect plan contains its complete scope.

## Verify and return

Use the parent workflow’s authorization and execution stages. Read the resulting item back, verify preserved identity and history for conversions, fields, types and directed relationships, and return the observed identifiers and limitations to the producer. Preserve identifiers from partial success for reconciliation before retrying.

This reference writes no source investigation or handoff state and introduces no second execution ledger.
