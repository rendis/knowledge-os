# Onboarding behavior trials

Run each scenario with a fresh agent, the candidate distribution, and a disposable directory. Do not give the agent expected answers or access to previous trial artifacts. Observe questions, actual commands and resulting configuration; a successful `--yes` install alone is not an onboarding test. Never use existing consumer vaults as fixtures.

| Scenario | Supplied choices | Expected behavior |
| --- | --- | --- |
| Missing decisions | Destination, systems, organization and prefixes; sibling repos/worktrees directories | `onboard-cell` proposes the branch order and clouds from the repositories and asks once per block; `onboard-developer` proposes the clone root and worktree root from `config detect` and asks one confirmation; nothing is recorded before it. Reuse supplied repository scope. |
| Read-only continuation | Answer the preceding questions with an exact existing source root, read-only access and deferred worktrees | Initialize with managed clone disabled, preserve source contents, report deferred capabilities and unexecuted sync. |
| Complete explicit request | All portable choices, exact managed source root with scoped clone/fetch authorization, exact worktree root; local onboarding only | Configure exactly once without redundant questions or acquisition. |
| Location only | All portable choices, exact source location and deferred worktrees, but no acquisition choice | Ask read-only versus managed access; do not enable acquisition based on location confirmation. |

Use synthetic repositories and offline execution. Compare actual `instance.yaml` and local workspace configuration with supplied answers. Keep source sentinels unchanged. Distinguish missing remote inventory from an empty successful sync. A scratch distribution without Git cannot pass strict provenance checks; confirm strict doctor separately from a clean committed distribution.

The `kos init` subprocess regression suite is `python3 -B evals/bootstrap/test_interactive_onboarding.py`: EOF at each prompt and SIGINT cancel without writing; intentional Enter accepts defaults; explicit answers (sources, reference branch order, clouds, tracker URLs) persist; reinitialization preserves existing content. These deterministic tests complement, rather than replace, agent trials. A finite set of agent trials does not establish universal behavior.
