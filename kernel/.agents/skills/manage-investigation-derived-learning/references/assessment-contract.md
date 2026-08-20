# Assessment contract

Use this contract for every assessment, including read-only or blind-test execution.

## Stable identity

A learning is identified by the **problem or question plus its applicability context**. The context includes the conditions that can change the result: workload and scale, data shape, architecture, relevant versions, operational constraints, measurement method, and material exclusions.

An investigation ID, Jira key, implementation branch, author, or date is provenance, not identity. Two investigations with the same question and materially equivalent context normally belong to one cumulative note. Create a context variant only when the differing condition could reasonably change the conclusion, and explain that boundary in both notes.

## Assessment outcomes

Select exactly one:

| Outcome | Use when | Durable effect |
|---|---|---|
| `extractable` | The durable-learning gate passes and the candidate adds a non-obvious, reusable, bounded conclusion. | Select `create`, `enrich`, `challenge`, or `supersede`. |
| `no-learning` | The relevant sources and context are sufficient, but the resolved result is trivial, purely case-specific, only restates a delivery fact, or does not improve a future decision. | `none`. |
| `already-covered` | A canonical note already contains the same teaching, applicability boundary, and materially equivalent evidence or stronger evidence. | `none`; identify the note and why the new material adds nothing. |
| `insufficient-evidence` | A required source is missing or unreadable, the investigation is unfinished, the comparison is not reproducible, context is too incomplete, decisive evidence exists only under `.investigations/` or another transient/local location, or a possible contradiction lacks enough support to qualify as a durable challenge. | `none`; name the exact missing source or context and the retry condition. |

Do not collapse `insufficient-evidence` into `no-learning`: the former is retryable; the latter is a substantive conclusion after sufficient review. Do not collapse `already-covered` into `no-learning`: deduplication is useful evidence that the knowledge system worked.

A negative or neutral result is not automatically `no-learning`. It is `extractable` when a sound comparison shows why an alternative should be avoided, why the baseline remains preferable in the stated context, or which method prevents a future error.

For `no-learning`, `already-covered`, or `insufficient-evidence`, the skill **must not create or update a learning note**.

## Lifecycle actions

Select exactly one:

| Action | Use when |
|---|---|
| `create` | No canonical learning exists for the stable question and context, or a material context difference can change the conclusion. |
| `enrich` | New evidence strengthens, qualifies, extends, or revalidates the current teaching without contradicting it. |
| `challenge` | Credible new evidence conflicts with the current teaching or its stated boundary and must remain visible while the conclusion is reassessed. |
| `supersede` | Stronger evidence establishes replacement guidance for the same context and the prior conclusion is no longer current. |
| `none` | The outcome is not `extractable`. |

Prefer `enrich` or `challenge` over creating a parallel note. Prefer `challenge` over `supersede` while material contradictions remain unresolved.

Use `challenge` only when the conflicting evidence independently passes the durable-learning gate and materially changes how safely the existing teaching can be used. A contradiction that is merely possible, unreadable, transient, or methodologically weak remains `insufficient-evidence` and causes no write.

Valid combinations are:

- `extractable` + `create|enrich|challenge|supersede`
- `no-learning|already-covered|insufficient-evidence` + `none`

Any other combination is invalid.

## Evidence comparison

Evaluate the candidate against:

1. The stated baseline and alternatives.
2. The test or observation method, sample/window, environment, controls, and material exclusions.
3. The measured or observable results, including negative and neutral results.
4. The reasoning that connects results to the conclusion.
5. The conditions under which the conclusion applies and does not apply.
6. Durable source references that another authorized agent can revisit.
7. For an adopted implementation, the exact implementation and productive applicability required by the production-evidence gate.

The investigation may organize those references, but its prose, a story, an approval, or an undeployed commit does not independently satisfy the gate.

A reproducible method qualifies only when its procedure, inputs, and criteria are preserved in a durable/versioned source or can be rerun from versioned tooling. Content under ignored local workspaces — including `.investigations/`, `.operations/`, `.knowledge-os-handoffs/`, and `plan/` — is provenance or working state; it cannot occupy `Fuentes durables` or independently support `extractable`.

## Required response

Present this card in the user's language:

```text
Resultado: <extractable|no-learning|already-covered|insufficient-evidence>
Acción: <create|enrich|challenge|supersede|none>
Identidad evaluada: <stable question + applicability context>
Evidencia revisada: <direct sources and boundary>
Razón decisiva: <why this outcome/action>
Nota canónica: <path or none>
Aplicabilidad: <where it applies and does not apply>
Faltantes o revalidación: <exact retry/revalidation trigger or none>
Efectos realizados: <none during assessment>
```

Assessment is complete only when every field is explicit. Do not imply a learning exists merely because the user invoked extraction.
