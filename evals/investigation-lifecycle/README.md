# Investigation lifecycle behavior fixtures

Prepare one isolated installed consumer for each executor and scenario:

```sh
python3 -B evals/investigation-lifecycle/prepare.py \
  --distribution /absolute/candidate-distribution \
  --scenario components \
  --output /absolute/fresh/executor \
  --evaluation-output /absolute/fresh/judge
python3 -B -m unittest discover -s evals/investigation-lifecycle -p 'test_*.py' -v
```

The scenario choices cover component/release tracking, observation planning,
independent observation cycles (missing access, successful unchanged coverage,
expired plan, cancelled plan), selective knowledge absorption, public retirement,
and historical lookup. All facts,
identities, source documents and receipt data are synthetic. No consumer data,
network service, scheduler or model runtime is used by preparation or tests.

`observation-scheduling` asks for a separate scheduled-task proposal while its
synthetic execution context explicitly lacks native scheduler/session support.
The expected capability-limited result is a proposal and a precise explanation
of what is missing, not a scheduler installation. Local snapshot evidence remains
available for bounded access/coverage validation.

`absorption` permits only the mandatory independent note reviewer within the
assigned inputs. `absorption-review-unavailable` uses the same request and source
facts while prohibiting delegation. This paired negative scenario requires the
executor to preserve canonical notes and private context, leave any draft staged, and
report the missing independent-review capability without substituting self-review.
The request authorizes recording correspondence: an attributed case-helper update
may record pending candidates and the blocker, but cannot claim completed absorption.

Give a **fresh executor** only the generated `task.txt` and its executor directory.
Its instructions and references come from the installed candidate distribution.
Keep the judge directory, distribution eval sources, previous responses and
rubric outside that executor's accessible inputs. This directory separation is
not a filesystem sandbox: the harness must enforce the stated access boundary.
Each observation cycle starts from its own seeded operational campaign at
`vault/.operations/<run-id>/run.md`, using `in-progress` or `cancelled`, so it does
not inherit another agent's answer. Cycle authority covers evidence collection,
the operational ledger and reporting; it does not authorize investigation writes.
Explicit synthetic current time and run/case identities are in
`context.json`. Successful coverage represents one window, not the whole plan.

`retirement` seeds a closed case and public attachment through the real helper,
then commits its exact snapshot in the synthetic vault. Its prompt authorizes
public retirement and a local commit, preserves the private overlay and excludes
push. `historical-lookup` additionally retires through the helper and commits the
deletion with the ledger, then asks an informational question by exact ID. Both
keep a private synthetic overlay outside Git. The other case and notes provide
an unchanged baseline outside the selected retirement target.

Freeze the executor's first response, changed artifacts and complete tool trace
before review. Give an independent reviewer those results, original inputs and
the separate `criteria.json`; record each criterion with supporting file or
transcript references, model, effort, elapsed time and available token usage.
For positive absorption, retain the separate reviewer invocation and returned
decision bound to the frozen manifest, preceding publication. A receipt or the
executor's claim alone does not prove independence. If the harness omits nested
agent events, coordinate preparation, a fresh reviewer, and publication as separate
observable phases; missing reviewer evidence keeps that gate unobserved.
Reading a forbidden runtime snapshot must be judged from the tool trace; an
unchanged filesystem cannot establish that no read happened. Keep corrections
separate from the first-pass score. `baseline-hashes.json` and `provenance.json`
capture the installed inputs and source revision/dirty state for reproducibility.

Automated tests establish installation, deterministic prompts, input separation,
closed snapshot setup and committed historical retrieval. A behavioral pass
requires fresh executor runs and independent review.
