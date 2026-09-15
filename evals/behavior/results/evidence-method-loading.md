# Explicit evidence-method loading — 0.8.5

Date: 2026-09-15. Scope: replace implicit method application with an explicit full read before relevant analysis in operational audits, investigation updates and vault interrogation. Already-loaded context, valid checks, navigation-only questions and already-supported documentary updates retain their fast paths.

## Observed operational scenario

Executed `run_analysis.py --task operational --model gpt-6-astra --effort low` against the candidate distribution in fresh offline scratch. The runner supplied the natural request and sanitized fixture; evaluator material remained outside executor input. No real consumer or cloud source was used.

The completed command events establish this order:

1. Read the operational workflow and registered offline procedure.
2. Read the complete `evidence-driven-analysis/SKILL.md` (event index 8, exit 0).
3. Read its audit reference and the simulated cloud evidence (event index 11, exit 0).
4. Resolve the registered procedure and write/read back one `.operations/` ledger.

The final answer distinguishes accepted submission from unproven persistence and incomplete pagination/time coverage. The process exited 0; the only changed vault path was the audit ledger. No investigation was created. This is one observed execution, not a guarantee that every future agent follows the prompt.

Raw event SHA-256: `8450cf09114d5068ff853e1f2be848b19e7a1b2c7b902f4b0617d0821755bc3b`.

## Verification

- Bootstrap: 35 passed; instance: 10 passed; analysis runner: 4 passed.
- Both modified skill entrypoints passed `quick_validate.py`.
- Independent read-only review found no actionable issue in scope, ownership, context reuse or fast-path preservation.
- The live user audit session was not modified or messaged.
