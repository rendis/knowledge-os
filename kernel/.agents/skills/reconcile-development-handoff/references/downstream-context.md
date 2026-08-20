# Direct-dependent story context

Load this reference when the source Jira story may block other stories.

## Select direct dependents

Use exactly one Jira link hop from the source story. Include a story only when current Jira metadata and the exact observed link type and direction establish that the selected story depends on, or is blocked by, the source story. Link labels are configuration-dependent: resolve both inward and outward descriptions from Jira before interpreting the edge.

Do not include descendants, parents, siblings, shared-epic members, text mentions, shared labels, or transitive dependents unless they also have their own qualifying direct typed link. Do not assume that `blocks`, `is blocked by`, or any localized label has a universal direction.

## Build one source card per story

Record:

- dependent key, URL, summary, status, updated timestamp, and intended outcome;
- source key plus exact link type, direction, and evidence that makes the dependency direct;
- the blocking condition as stated by the dependent story;
- contracts, APIs, events, data, configuration, compatibility, rollout/rollback, and verification information the dependent needs;
- exact names, schemas, payload shapes, endpoints, flags, ordering/idempotency/error behavior, migrations, and version constraints when observed and relevant;
- what the dependent can start now without another repository investigation;
- remaining gaps, owner/source for each gap, and the observable condition that releases it;
- source repository normalized remote and local worktree path;
- local branch and `HEAD`;
- observed remote branch name, remote ref and SHA, and observation method;
- pull request URL, pull request head, base, and state, checks, and observation time when one exists;
- merge and deployment as separate observations with their own evidence;
- relevant tests, contract fixtures, documentation, and stable code paths.

Do not call a branch `published` unless a current remote query observes its remote ref and SHA. A local branch, tracking configuration, push intent, pull request, merge, and deployment are six distinct states.

## Classify readiness

Set readiness to exactly one of `ready`, `partial`, `still-blocked`, or `not-applicable`:

- `ready`: all source information required by the dependent is observed, stable enough to consume, and accessible; no source-story blocker remains.
- `partial`: useful work can start from the supplied context, but named gaps prevent full completion or verification.
- `still-blocked`: a required contract, implementation, remote artifact, decision, or verification remains unavailable.
- `not-applicable`: the typed link exists, but this implementation produced no repository-specific context needed by that dependent; explain why.

Readiness describes only this dependency edge. It does not change Jira status, resolve the link, prove merge/deployment, or authorize the dependent implementation.

## Propagation

Store each card in the source investigation's current affected surfaces and traceable registers through `manage-investigation`. When a later development package is exported for the dependent story, compile the relevant card into its repository-specific `context.md` and `scope.md`. The dependent implementation should not need to inspect the source repository merely to rediscover an already observed contract.

## Completion criterion

Downstream context is complete when every direct typed dependent has exactly one evidence-backed readiness value, actionable consumer context, exact repository/delivery state, and explicit remaining gaps, while transitive and inferred relationships remain out of scope.
