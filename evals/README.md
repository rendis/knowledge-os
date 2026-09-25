# Evals

Checks and measurements of this distribution. Nothing here is installed into a cell, and cell-specific
questions, fixtures and results stay outside this repository.

| Directory | What it checks | How |
|---|---|---|
| `bootstrap/` | Installer: init, update, adopt, doctor, locks, retirement of old files, product-leak scan, onboarding prompts, native packaging | `make test-installer` (after `make release`) |
| `benchmark/` | Homologated comparison of models and efforts on a cell's questions and a sync flow, with a fixed blind judge and reviewer | [benchmark](benchmark/README.md) |
| `regression/` | The runner and judge the benchmark uses; also a before/after check of a kernel change on a cell's questions | [regression](regression/README.md) |
| `discovery/` | `discover run` against installed cells (stored judgments only) | [results](discovery/results.md) |
| `behavior/` | `explain-visually` assets: pinned icons and browser-side scripts | [behavior](behavior/README.md) |
| `cli/` | Running the native binary on other Linux distributions and Windows | [cli](cli/README.md) |

Go unit tests (`make test`) cover the CLI itself: discovery, gates, sync, cases, handoffs, retrieval and
configuration. Skill behavior is measured with real agents through `benchmark/`; a static check does not show
that an agent follows a procedure.
