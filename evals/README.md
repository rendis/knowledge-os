# Evals

Checks and measurements of this distribution. Nothing here is installed into a cell, and cell-specific
questions, fixtures and results stay outside this repository.

| Directory | What it checks | How |
|---|---|---|
| `bootstrap/` | Installer: init, update, adopt, doctor, locks, retirement of old files, product-leak scan, onboarding prompts, native packaging | `make test-installer` (after `make release`) |
| `benchmark/` | Homologated comparison of models and efforts on a cell's questions and a sync flow, with a fixed blind judge and reviewer | [benchmark](benchmark/README.md) |
| `regression/` | The runner and judge the benchmark uses; also a before/after check of a kernel change on a cell's questions | [regression](regression/README.md) |
| `platform/` | Platform providers with the clouds' real CLIs against local emulators: Pub/Sub emulator with `gcloud`, moto with the AWS CLI (SNS, SQS, DynamoDB, RDS, S3); referenced scopes, capture per service, wiring, links to configuration and code, and denied accounts. Google data services are checked against real projects by hand; they have no management emulator. Azure has no emulator of its management API and is covered by the Go tests | `make test-platform` (Docker) |
| `discovery/` | `discover run` against installed cells (stored judgments only) | [results](discovery/results.md) |
| `cli/` | Running the native binary on other Linux distributions and Windows | [cli](cli/README.md) |

Go unit tests (`make test`) cover the CLI itself: discovery, gates, sync, cases, handoffs, retrieval and
configuration. Skill behavior is measured with real agents through `benchmark/`; a static check does not show
that an agent follows a procedure.
