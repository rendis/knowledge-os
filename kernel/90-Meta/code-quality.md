# Kernel validation and development quality

## Installed vaults

Select the executable through `use-vault-cli` (`.agents/skills/use-vault-cli/SKILL.md`). The structural closure commands and
when to run them live in [[Auditoria - Framework#Gates]]. Consumer vaults use the
compiled kernel; they do not install Python lint tools or a compiler to validate
notes. Browser-executed resources remain owned by their skills and are verified
in that environment.

## Distribution development

Run these checks from the distribution checkout when changing native code:

```text
go vet ./...
go test ./...
go test -race ./...
```

Build and execute the relevant binaries on supported OS/architecture targets.
Cross-compilation establishes build compatibility; execution tests establish
runtime compatibility. Exercise persistence recovery, authorization boundaries
and migration compatibility when those contracts change. Measure startup time,
index refresh/search time and memory use on the actual benchmark target.

Python installer/evaluation sources retained in the distribution still use
`kernel/90-Meta/check-code-quality.py --root .`, with the tools pinned in
`kernel/90-Meta/requirements-ci.txt`. Prepare that development environment only
when requested or authorized. This is a distribution development gate, not an
installed-vault runtime dependency. Ruff checks correctness and core lint;
Bandit checks maintained Python code and rejects medium/high findings and
runtime assertions. Tests remain outside Bandit. Fix findings in their source
context; do not suppress a rule merely to obtain a passing run.

After installer, kernel or evaluation changes, run the repository-mandated
bootstrap and instance regression suites from the distribution checkout.
Behavioral skill tests still require a fresh agent context with realistic
fixtures; static checks do not demonstrate that an agent follows a procedure.
