# Read work-item evidence

Use this shared contract whenever a vault workflow needs current external work-item evidence without creating an operational run or acquiring write authority.

## Resolve the tracker

Read `instance.yaml` and require one exact configured tracker plus one exact provider-native work-item reference. With one configured tracker, select it. With several, accept an explicit tracker ID or resolve the exact item URL origin to one configured tracker origin; ambiguity blocks the read.

The tracker binding is its exact `id`, `provider`, and canonical URL. Once a case or handoff references that ID, a different provider or URL is a different tracker and requires a new ID or an explicit migration.

Require a credential-free HTTPS item URL on the resolved tracker's exact origin. Resolve optional destination mappings through `cell-config.py --vault-root "<VAULT_ROOT>" resolve --capability work-item-evidence`. When configured, read the returned procedures and select the one that explicitly covers the exact tracker; an ambiguous or invalid binding blocks only the evidence that depends on it. The destination owns provider URL-membership rules, field mappings and relationship semantics. The kernel supplies no provider mapping.

Without a configured mapping, use the connected capability’s observed identity and metadata. Accept a relationship only when its type, endpoints and direction are established by that surface. Mark relationships `unsupported` only when the capability does not expose them; use `unavailable` when support or semantics cannot be established from the available evidence. Never invent a provider-specific URL path, hierarchy or dependency meaning.

## Read-only procedure

1. Require the vault workflow and decision that need the evidence, the smallest material field/comment/attachment/relationship set needed, and an observation timestamp.
2. Use an already connected capability matching the configured provider only for reads. Keep connector selection and credentials outside the vault.
3. Read the source item's current reference, URL, type, status, title or summary, description, acceptance criteria when present, requested material fields, update timestamp, and only relevant comments, evidence, or attachments.
4. Redact secrets, credentials, private personal data, and sensitive attachment contents. Preserve only non-sensitive identity, location, behavioral relevance, and access limitations.
5. When relationships matter, preserve the provider's observed relationship identity and direction. Follow at most one observed relationship hop. Never infer a dependency from hierarchy, labels, prose, shared grouping, or proximity.
6. Record unavailable fields, denied content, stale timestamps, partial reads, and contradictions. Do not substitute memory, copied handoff text, or a case narrative for current external state.

## Normalized in-memory snapshot

Expose only the facts required by the caller:

- tracker ID, provider, canonical tracker URL, provider-native reference, item URL, type, status, and update timestamp;
- requested material fields with their observed field identities;
- selected comments, evidence, and attachments with stable identity, timestamp, non-sensitive summary, and access limitation;
- relationships with `status: observed`, `unsupported`, or `unavailable` plus exact observed edges when present;
- observation method and timestamp, redactions, contradictions, and unavailable evidence.

An unreadable source item blocks the caller. `relations: unsupported` permits source processing with an explicit limitation. `relations: unavailable` blocks dependent evaluation because support exists but current evidence is missing or unreadable. The calling workflow decides how the remaining normalized facts affect its own state.

Keep the snapshot in the current vault-side interaction. This contract creates no external write, operation ledger, case patch, handoff file, cache, or second evidence store. A read grants no external-write authority.

## Completion criterion

The read is complete when the tracker binding and item reference are exact, every requested source fact is observed or explicitly unavailable, relationship status is explicit, sensitive material is minimized, and the caller can distinguish external evidence from copied context and local inference.
