---
name: report-to-distribution
description: "Trigger: kos fails or gives a wrong result, a skill or gate behaves in a way the user dislikes or is frustrated by, or the user wants to improve or change the kernel (router, skills, gates). Turns it into a sanitized issue or proposal for the knowledge-os distribution, with the user's approval."
---

# Report to the distribution

The kernel and `kos` come from the knowledge-os distribution (`rendis/knowledge-os`); the vault only carries a copy that `kos kernel update` replaces. A failure, an unwanted behavior or an improvement is fixed there, so every cell benefits. Offer this path as soon as you notice one of the triggers; the user decides.

## 1. Name it

| Kind | When |
|---|---|
| **failure** | `kos` errors, crashes or returns something wrong; a skill step cannot be followed as written |
| **behavior** | it works as designed, but the design does not fit how the team works |
| **proposal** | a capability or change the kernel does not have yet |

A preference only this person has belongs in `AGENTS.personal.md` (router, *Personal instructions*); a team rule that fits the kernel as it is belongs in cell-owned notes. Kernel files are never edited in the vault: `kos kernel update` would refuse or restore them.

## 2. Draft it, sanitized

Write the draft to `.scratch/report/issue.md` with these sections: **Summary** (one sentence), **Versions** (from `kos version`: kos and kernel; OS and architecture), **What happened**, **Expected**, **Reproduce** (the shortest steps, with synthetic names), **Evidence** (the minimal command and output that show it), **Proposed change** (for behavior and proposal: which kernel file or command, and the change).

The distribution is shared beyond this cell. Keep only what reproduces or explains the problem and replace everything that identifies people, the company, the client, the project or its systems:
- names, emails and handles of people; company, client, product, system, repository and team names;
- hosts, URLs, cloud projects or accounts, ticket keys, IDs, paths, branch names;
- credentials, tokens, data values and query results.

Use neutral placeholders (`<repo-a>`, `service-x`, `project-1`). If the problem cannot be shown without identifying data, describe its shape instead of pasting it.

## 3. Show it and ask

Show the user the full draft and what will be sent where. Nothing leaves the machine without their explicit approval of that text.

## 4. Send it

With approval, and a GitHub login that can read the repository:

```text
gh issue create --repo rendis/knowledge-os --title "<kind>: <summary>" --label <failure|behavior|proposal> --body-file .scratch/report/issue.md
```

When labels are missing or the account cannot create issues, give the user the draft to file themselves. A change the user wants to make to the kernel goes as a pull request to the distribution, with the same sanitizing, after the issue. Report the issue or pull request link.

Done when the user has decided, and a sent report has its link.
