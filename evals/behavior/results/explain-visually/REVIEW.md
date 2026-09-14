# Explain-visually validation and final review

Date: 2026-09-14. Scope: distribution skill, templates, structural/context checkers, investigation attachment contract, and synthetic behavioral fixtures. No consumer publication, commit or push was performed.

## Outcome

The nine behavioral decisions meet the rubric after one targeted source-fidelity correction. Automated regression checks pass. Independent static review findings have been repaired and rechecked. **The user subsequently approved the spatial template after opening it manually. Agent-driven browser validation remains blocked; other templates have no new manual browser approval in this run.**

## Behavioral execution

Three isolated Luna high sessions each received three fictional requests, with the current skill and relevant references available and without the expected-outcome ledger. Contexts were described as independent within each batch; they were not nine separate model sessions. The primary agent inspected actual responses and generated files. This bounded evaluation is evidence of these cases, not a statistical guarantee of future behavior. Model choice belongs to this evaluation only; the skill remains model-neutral.

| Case | Observed output | Assessment |
|---|---|---|
| 1. One-sentence definition | [One sentence, no widget](case-1/response.txt) | Pass: proportional response. |
| 2. Request-to-storage confusion | [Four source-backed arrows across five components](case-2/diagram.md) | Pass: focused temporary Mermaid and text equivalent; rendering unverified. |
| 3. Ambiguous visual request | [Contextual clarification](case-3/response.txt) | Pass: asks about flow versus alternatives before choosing a visual. |
| 4. Retained architecture comparison | [Embedded comparison and context](case-4/A-004-architecture-comparison.md) | Pass after correction: first attempt asserted error propagation not established by the source. [Original](case-4/first-attempt.md) retained for audit. Revised text marks it unspecified. |
| 5. Illustration inside an open investigation | [Temporary queue explanation](case-5/temporary-queue-illustration.md) | Pass: investigation existence does not imply retention. |
| 6. Promote a useful temporary diagram | [Retained context](case-6/A-006-producer-queue.md), [original](case-6/temporary-source.md) | Pass: Mermaid content preserved byte-for-byte, context added, case registration delegated to owner. |
| 7. Text-only architecture request | [Text-only explanation](case-7/response.txt) | Pass: no visual artifact generated. |
| 8. Update retained simulator | [Four-day simulator](case-8/weekly-completed-units.html), [matching context](case-8/weekly-completed-units.md) | Pass: formula, initial values, scale, reset defaults and explanation updated together. Context hash matches. Independent numeric checks confirm default 880, maximum 1900, reset 880. |
| 9. Production topology without sources | [Source boundary response](case-9/response.txt) | Pass: does not label the synthetic architecture as production. |

The retained cases exercise preparation and handoff to the investigation owner. They do not claim actual registration in a consumer case. No rendered Mermaid or HTML pass is claimed.

## Repairs made during this review

- Scenario template: added reset; the text table includes the custom projection; selected planning reference no longer mislabels the custom projection. Regression checks cover reference independence and reset.
- Spatial template: added reduced-motion handling. Existing handlers were exercised across all ten nodes, all three routes, Enter/Space activation, both clearing controls, and theme state with DOM stand-ins.
- Context checker: accepts reference-style links; requires hashes for local companion links; excludes comments, fenced examples, inline code and indented code from embedded-diagram detection, including longer valid closing fences.
- Independent reviewer confirmed the reported checker defects closed after the regressions. No remaining finding in that bounded static scope.

## Automated evidence

Run from the distribution root:

```sh
python3 -B evals/bootstrap/test_bootstrap.py
python3 -B kernel/90-Meta/test_instance.py
python3 -B -m unittest discover -s evals/behavior -p 'test_visual*.py'
node evals/behavior/test_visual_focus.cjs
node evals/behavior/test_visual_spatial.cjs
node evals/behavior/test_visual_scenario.cjs
node evals/behavior/test_visual_scenario.cjs evals/behavior/results/explain-visually/case-8/weekly-completed-units.html 4
```

Observed results: bootstrap 35 tests; instance 8 tests; visual Python 14 tests; focus helper 4 cases; spatial handlers 18 cases; each simulator 9 boundary combinations plus reference-independence and reset checks. All passed. Structural checks pass for the four bundled visual templates. Context checks pass for retained behavioral cases 4, 6 and 8. These checks establish code/data correspondence, not browser rendering, actual keyboard navigation, computed CSS contrast or clipping.

Reviewed source and fixture hashes are recorded in [reviewed-files.json](reviewed-files.json).

## Unresolved environment gate

The browser tool rejected local-file navigation under its URL security policy and explicitly prohibited workarounds, alternate browser surfaces and indirect execution. No such workaround was attempted. Consequently the required desktop/narrow screenshots, computed contrast and label-size inspection, real control/keyboard interaction, offline browser behavior and reduced-motion rendering could not be verified in this run.

To close this gate, a permitted browser session or human browser inspection must verify the exact recorded files using the skill's verification contract (desktop approximately 1440px, narrow approximately 390px). Static inspection, DOM stand-ins, earlier screenshots of different revisions and user aesthetic approval do not replace that gate. Keep the release verdict conditional until that evidence exists.

## Subsequent manual acceptance

The user opened the current `kernel/.agents/skills/explain-visually/assets/path-explorer.html` and reported that it was perfect after receiving the node/route selection, reset, keyboard, narrow-window and theme checklist. Record this as user-reported overall acceptance of the spatial template, not individually measured checks or agent-observed browser evidence. No screenshots or per-check results were supplied. This acceptance does not extend to the other templates or the four-day simulator fixture.

## Manual review of remaining standalone examples

The user approved example 2 (atomic SVG), example 3 (five-day simulator) and example 4 (four-day simulator). Example 1 (editorial explainer) failed: a supplied screenshot showed desktop SVG text extending beyond its cards. The desktop cards were widened, descriptions split into explicit lines, headings fitted to available width, and connecting arrows repositioned. This repair needs renewed manual render acceptance; source checks alone do not close it. No approval was reported for the four Mermaid examples.

## Reusable containment prevention

The user subsequently approved the repaired editorial explainer. The skill now specifies container sizing, line wrapping and a rendered text-containment gate rather than relying on a template-only repair. `check_text_fit.cjs` checks actual SVG bounds or HTML text ranges in a permitted browser context. The explainer annotates all eight desktop/mobile containers and their labels. Seven synthetic regression cases cover the reported overflow, wrapping, padding, vertical bounds, absent measurements and the measurement adapter. They do not establish browser execution of the helper, which remains unavailable under the existing restriction. The four Mermaid examples still lack reported manual acceptance.

## Final user acceptance

The user confirmed completing the remaining checks personally and requested closing them. The pending manual visual checks, including the Mermaid examples, are therefore closed by user-reported acceptance. Earlier blocked-browser entries remain historical evidence; they do not mean the agent executed those checks or the browser helper. Implementation and bounded static review have no reported outstanding findings. Commit and consumer propagation have not been performed.
