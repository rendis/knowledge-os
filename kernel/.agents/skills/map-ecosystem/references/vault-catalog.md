# Maintain the related-vault catalog

`90-Meta/vault-catalog.yaml` belongs to the consumer vault and is versionable. It describes related domains, not a user's access list. The installer preserves it. Absence means an empty catalog; create it only for an authorized first registration. `map-ecosystem` owns maintenance; `configure-workspace` retains ownership of local checkout configuration.

## Register or update

1. Resolve the origin vault and load its catalog with `<cli> config catalog --vault "<origin>"`. Invalid data requires a targeted repair; preserve existing entries and never replace a malformed catalog with an empty one.
2. Determine authority:
   - **Discovery during mapping:** finding a candidate authorizes validation and a proposed entry, not registration. Present the complete proposed entry or update diff and ask for explicit user confirmation before writing. Batch candidates when useful. Silence, a mapping request, or a generic instruction to finish does not confirm registration. Continue independent mapping while awaiting the answer; declined or unanswered proposals leave the catalog unchanged.
   - **Direct request to register or update:** the request authorizes the named entries. Validate and write without a second confirmation unless target identity or scope is ambiguous. Merely mentioning or consulting a vault is not a registration request.
3. Follow [cross-vault-consultation.md](cross-vault-consultation.md) to enter the destination: explicitly read its `AGENTS.md` and optional `AGENTS.personal.md`, verify identity and follow its orientation. Derive domain, scope and summary from that vault's authoritative orientation; derive the relationship from inspected evidence or the user's explicit statement and distinguish those bases. No repository remapping is needed. If access or identity cannot be validated, report the blocker and leave the entry unwritten; a remote URL alone does not validate a domain description.
4. Prepare the complete entry using the contract below. Include only information suitable for the origin repository's audience. Personal instructions, private overlays, credential values, local checkout paths and user-specific access state stay out of the catalog. Validation checks structure, not factual accuracy or disclosure permission; inspect both before writing.
5. Match existing entries by stable ID and normalized repository identity (HTTPS and Git SSH forms identify the same repository). Update that entry instead of appending a duplicate; preserve its ID. If ID and repository match different entries, resolve the ambiguity before writing. Preserve all unrelated entries. For discovered updates, confirm the precise diff; direct update requests already provide authority.
6. Stage the proposed catalog in a disposable copy of the origin metadata, validate with `<cli> config catalog --vault "<candidate-root>"`, then replace only the authorized origin catalog and run `<cli> config catalog --vault "<origin>"`. Keep changes confined to the origin catalog and verify the diff. Registry edits occur outside synchronization publication packages; ordinary mapping write authority alone does not waive step 2. Registration does not authorize a commit, push, destination edit or access acquisition.

List requests remain read-only. Remove an entry only on an explicit removal request and report the exact removed ID; loss of local access alone does not remove a shared relationship. Updates follow the same validation and authorization path as registration.

## Version 1 contract

Use the repository's supported simple YAML syntax, with exactly `version` and `vaults` at the root and exactly the entry fields shown below. `version` is integer 1; `vaults` is a list. All text fields and both text lists are nonempty. IDs use lowercase kebab-case; IDs and normalized repositories are unique. Repository URLs are credential-free HTTPS or Git SSH remotes. Local-only destinations can still be consulted but cannot be registered until they have a portable repository identity.

```yaml
version: 1
vaults:
  - id: payments
    repository: https://example.org/team/payments-vault.git
    domain: Payments
    scope: "Payment authorization and settlement; excludes order fulfillment."
    summary: "Documents payment services, contracts and operational flows."
    tags: [payments, settlement]
    relationship: "The order service requests payment authorization; documented in 40-Integraciones/Payments.md."
    consult_when: ["Investigating authorization failures", "Assessing settlement contract changes"]
```

`domain`, `scope` and `summary` describe the destination; `relationship` explains its relevance to the origin and names supporting portable evidence or user-stated intent. `consult_when` gives routing examples. Use the origin's note locale for descriptions and tags. This example is documentation, not a seeded catalog. Do not register synthetic entries in real vaults.

## Use and completion

Consultations search entries by domain, scope, tags and routing examples, then verify current destination availability and instructions. The catalog does not grant access, prove a live integration, or restrict consultation to registered destinations. Resolve checkout paths through existing workspace bindings by repository identity or a user-supplied path, without persisting those paths here.

Finish with the added/updated/removed IDs, validation result and remaining limitations. If confirmation or destination verification is pending, report that no registration occurred. A read-only list or consultation finishes without catalog writes.
