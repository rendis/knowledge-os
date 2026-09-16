# Conversational response quality

A synthetic multi-turn inventory case tests whether an agent limits conclusions to the evidence before a user has to correct it. All names, stores, code and events are fictional. This is an evaluation fixture, not a production incident reconstruction or a measured quality result.

Prepare a fresh installed consumer using the normal installer:

```sh
python3 -B evals/response-quality/prepare.py --worker /tmp/response-worker --evaluator /tmp/response-evaluator
python3 -B -m unittest discover -s evals/response-quality -p 'test_*.py'
```

Run the worker from its assigned directory with `task.txt`. Restrict its filesystem to that directory; do not give it this repository, memory, the evaluator directory or future prompts. A prompt boundary alone is not filesystem isolation. Allow installed-policy delegation only with the same evidence boundary. Preserve complete tool and subagent transcripts for evaluation. No live service access or source modification is needed.

The evaluator owns `evaluation.json`. Deliver its turns one at a time in the same conversation, without corrective hints or requirements. Immediately before `review-reuse`, write the supplied `prior_review_injection` object as `sources/prior-review.json` in the worker. This is an explicitly synthetic previously accepted review, not a claim that this campaign already ran an independent reviewer. Its hashes bind the exact supplied evidence. Do not inject it earlier or show the evaluator rubric to any worker/reviewer under test.

The case distinguishes a defined but uncalled flag query from registered consumers; provider-specific selection from an unrelated provider; configuration/source selection from runtime delivery; and incidental report labels from verified incidents. Follow-ups check continuity, legitimate reuse of an unchanged reviewed conclusion, and rejection of that review for a different environment and time. The first answer must stand on its own: later correction does not erase an initial critical failure.

Evaluate meaning and source support using the separate rubric, not phrases or word counts. Include a concise assessment with response/source references, critical failures, review observations and available total usage. Compare repeated runs with identical sources and prompts if measuring policy changes. Fixture tests prove source behavior, evidence binding and package separation; they do not prove agent compliance, review independence, token savings or production safety. No model campaign is run by the prepare script or unit tests.
