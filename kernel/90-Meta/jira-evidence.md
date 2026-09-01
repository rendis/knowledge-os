# Normalize Jira evidence

Load `work-item-evidence.md` first and apply its read-only procedure, snapshot,
limitations, and completion criterion unchanged. This reference adds only the
Jira-specific identity and relationship mapping.

## Identity mapping

Resolve exactly one configured tracker whose provider is `jira`. Accept either:

- its tracker ID plus an exact Jira issue key; or
- a canonical issue URL that resolves below that tracker's canonical URL.

Map the observed issue to the shared snapshot with that tracker ID,
`provider: jira`, its canonical tracker URL, the exact observed issue key as the
provider-native reference, and the canonical issue URL. Textual similarity does
not resolve a tracker, project, or issue.

## Relationship mapping

When relationships matter, read Jira's live link metadata and issue links.
Preserve the exact link type name, inward description, outward description,
endpoint keys, and issue-scoped direction. A related issue is eligible for the
shared contract's one-hop read only through that exact observed typed edge and
direction.

Set relationships to `observed` only when both live link metadata and the
issue-scoped direction were read successfully. Use `unavailable` when Jira
supports the surface but metadata, endpoints, or direction could not be read
unambiguously. Jira typed relationships never map to `unsupported`.

## Jira-specific snapshot fields

For each observed Jira relationship, add its live type name, inward and outward
descriptions, endpoint keys, and direction relative to the source issue to the
shared snapshot. All other fields and limitations come from
`work-item-evidence.md`.
