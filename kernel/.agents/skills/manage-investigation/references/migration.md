# Legacy investigation migration

Use this runbook only when the user asks to migrate one or more cases from `.investigations/`. The installer never migrates cases and this is not a synchronization mechanism.

## Transform one selected case

1. Read the complete legacy case and its referenced local artifacts. Do not modify it.
2. Preserve its investigation ID, creation time, decision meanings, stable register identifiers, useful chronology, and durable references.
3. Create the corresponding public case in `investigations/`. Replace **Original request** with a professional **Request summary**; omit transcripts, hidden reasoning, irrelevant process chatter, and absolute local paths.
4. Translate each development location to normalized repository remote plus exact branch. Retain an observed commit only when it identifies evidence. If repository or branch cannot be verified, preserve the reference as an explicit unresolved limitation rather than inventing availability.
5. Apply the normal persistence classification. Generalize sensitive but shareable meaning into public. Create a private overlay only for necessary sensitive context. Never persist credential values.
6. Validate the new public case and, when present, the private overlay. Compare old and new registers explicitly: every decision and reference must be preserved, intentionally superseded, or documented as omitted with reason.

Migration is complete when the new case is understandable without private context, its ID and decision semantics are preserved, structural validation passes, and the legacy bytes remain unchanged. Do not mark, delete, or rewrite the legacy case automatically.
