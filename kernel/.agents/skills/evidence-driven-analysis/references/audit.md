# Audit

Use to evaluate whether a claim or guarantee holds. The owning workflow defines the audit's scope, required procedure, persistence, and acceptance gate.

- Translate the claim into observable obligations. Locate the relevant contract, implementation, dependencies, and execution evidence; distinguish what is configured, assumed, checked, and actually observed.
- Inspect the path that enforces each material guarantee, including relevant failure paths and consumers. A function name or caller assumption does not establish enforcement by its callee.
- Compare observations against the claim using matching identities and time windows. Technical acceptance, queue admission, successful HTTP status, or a clean merge does not establish the downstream functional result.
- Check coverage before negative conclusions: filters, page tokens, sampling, truncation, environments, revisions, and inaccessible dependencies. Report bounded absence or insufficient evidence when exhaustive absence is unsupported.
- Report actionable discrepancies with decisive evidence, impact, and limits. Distinguish contradicted claims from unverified claims. Do not invent severity, approval, deployment, or successful remediation; return findings to the owner without writing its records yourself.
