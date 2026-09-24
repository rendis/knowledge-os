# Project evidence specialists

The router owns when independent review is required. [[evidence-investigator]] resolves independent bounded questions; [[evidence-reviewer]] verifies candidate answers against actual sources. Neither replaces the primary workflow or its publication gates. Pass source identity, scope, access restrictions and provenance explicitly; subagents need not inherit the full conversation.

## Installed definitions

| Harness | Project definitions | Execution boundary |
| --- | --- | --- |
| Codex | `.codex/agents/*.toml` | Native read-only sandbox; model and effort inherit the parent. |
| Claude Code | `.claude/agents/*.md` | Editing and recursive Agent tools excluded; shell/external read-only scope remains an instruction and configured permission boundary. |
| Cursor | `.cursor/agents/*.md` | Native `readonly: true`; native project definitions take precedence over compatible Claude/Codex paths with the same names. |

These are project-local roles, not global agents or hooks. Discovery depends on the installed harness version and user settings. Verify the role is available in the current session before dispatch. If disabled or unsupported, use an allowed generic subagent with the same contract and role instructions, or follow the router's unavailable-review rule. Do not alter user permissions or enable tools implicitly. A read-only filesystem setting does not establish that an external service call is read-only.

The shared evidence contract and role body are embedded in each native definition at distribution build time, so execution does not depend on the specialist deciding to load the core method. The canonical sources are `AGENTS.md` and `90-Meta/specialists/`; distribution maintainers render with `python3 -B scripts/render_specialists.py` and verify with `--check`. The renderer stays in the distribution and is not installed in cells. Generated definitions are checked for drift before distribution acceptance.

Installing the definitions proves neither invocation nor answer correctness. Validate actual discovery, dispatch and source-backed results in each target harness. Preserve consumer-owned definitions and reject collisions or local modifications through the installer's normal ownership checks.
