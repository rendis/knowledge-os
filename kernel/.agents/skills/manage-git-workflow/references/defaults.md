# Git and GitHub defaults

Use these conventions only where the target repository instructions and applicable specific notes linked from `60-Operacion/Git/Git.md` are silent. The area index itself is navigation, not policy. These are fallback decisions, not a replacement for live repository rules.

## Tool and identity

- Prefer local `git` for files, refs, history, staging, commits, and repository state.
- Use `gh` for GitHub pull requests, checks, reviews, rulesets, tags exposed through releases, and other GitHub objects.
- Match the authenticated GitHub host and account to the target remote before a GitHub read or write. Keep credentials and tokens out of output and files.
- Resolve changing flags and platform behavior from current command help or official documentation.

## Branches and synchronization

- Start from observed status, branch, upstream, remotes, and divergence. Keep the current branch unless the requested outcome requires another one.
- Treat `fetch` as a local-ref mutation: use it when freshness is required and within the requested workflow, then report the refs it updated.
- Never choose merge, rebase, reset, or cherry-pick merely to make divergence disappear. Select the strategy from applicable policy and the user's requested outcome.
- Keep unrelated tracked and untracked work intact. A dirty worktree is a planning input, not an invitation to stash or discard it.

## Commits and pushes

- Make cohesive commits whose staged diff matches the stated intent. Stage explicit paths and inspect the staged diff.
- Use the repository's commit convention. In the absence of one, use Conventional Commits and omit generated attribution or co-author trailers.
- Confirm the destination branch and upstream before pushing. A normal push is the default; a history rewrite requires explicit authorization and uses the safest lease-protected mechanism supported by the situation.
- Verify the remote ref resolves to the intended commit after pushing.

## Pull requests and merges

- Resolve the actual base branch and inspect the branch diff, commits, repository instructions, templates, and live rules before creating or updating a pull request.
- Validate the title and required ticket against the applicable specific Git notes before creation. Do not generalize a rule that applies only to one base branch or workflow.
- Read the pull request back after a write and verify base, head, title, body, state, and required checks.
- Merge only with the policy-approved method after required checks and reviews are observed. Branch deletion is a separate effect unless the request or repository workflow explicitly includes it.

## Releases and hotfixes

- Inspect existing tags, releases, and versioning policy before proposing a version or target. Do not invent a release scheme from repository contents alone.
- A hotfix is an expedited path, not relaxed authorization. Apply the local naming, ticket, review, and merge exceptions exactly as documented.
- Verify tags against their intended commits and read releases back from GitHub after creation or modification.

## Recovery

- Diagnose first from status, reflogs, refs, and remote state. Prefer reversible recovery that preserves evidence.
- Describe the recoverable point and data at risk before reset, clean, force push, branch deletion, or other destructive action; execute only the exact authorized target.
- Stop after one failed or ambiguous remote mutation and reconcile current state before retrying.
