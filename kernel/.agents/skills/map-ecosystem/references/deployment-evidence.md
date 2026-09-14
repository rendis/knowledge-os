# Resolve deployment by environment

Load this reference for an explicit deployment audit, or the specific part needed to resolve a main-flow connector/trigger. Ordinary repository mapping uses repository-map.md and does not require a full deployment chain. For targeted resolution, stop at the concrete target or unresolved indirection. The full sequence and matrix below apply to the explicit deployment audit. Scanner output is a lead, not proof of deployment.

## Evidence sequence

1. Inspect `.github/workflows/`, including called reusable workflows, event filters, branch or tag conditions, matrices, environments, inputs, and job dependencies.
2. Follow the referenced build and deployment definitions, container files, workload manifests and infrastructure-as-code modules. Cloud Build, Helm/Kustomize, Cloud Run/Functions, AWS CloudFormation/ECS/Lambda and Azure deployment definitions are examples, not an exhaustive platform list. Include other cloud and on-premises targets when observed.
3. Resolve versioned variables that identify project, platform, resource, region or zone, namespace, workload, artifact, and environment. Record secret names only as unresolved indirection; never infer their values.
4. Inspect an accessible pinned reusable action or cross-repository deployment definition when the local workflow delegates target selection to it. Otherwise record the external definition as an exact unresolved indirection, answer dependent fields as “no observado en fuentes estáticas revisadas”, and withhold only claims that require the unavailable body. The unresolved indirection does not make the repository analysis or synchronization incomplete.
5. Follow the access gate in connection-reconciliation.md before a bounded read-only metadata query against the concrete static target. Use the configured provider or infrastructure executor; platform examples confer no authorization. Keep missing access pending only when it blocks an in-scope material question under [evidence-sufficiency.md](evidence-sufficiency.md); ordinary maps do not require operational success tests.

## Per-environment chain

For every environment observed in versioned evidence, trace:

```text
event or manual input
→ workflow and job
→ environment condition
→ build artifact
→ deploy action or command
→ provider account/project/subscription or on-premises authority
→ platform and resource
→ location
→ namespace or workload, when applicable
→ manifest, overlay, or values source
```

Apply the deployment table contract from **Infraestructura y scheduling** in `90-Meta/Convenciones.md`. Keep multiple deployables in separate rows. Use `no observado en fuentes estáticas revisadas` for a missing field after following every versioned indirection, and `#por-confirmar` only when the gap affects understanding of the runtime or production baseline.

## Interpretation rules

- A workflow filename, environment label, branch name, or manifest template alone does not prove a deployed destination.
- A configured target in a workflow proves deployment intent at the analyzed commit; current runtime existence requires reconciled control-plane metadata.
- A secret or organization variable name proves only that indirection exists.
- A reusable workflow pinned by SHA or tag remains external evidence until its relevant versioned body is inspected.
- Contradictory projects, branches, overlays, or runtime names remain separate evidence; do not choose one by naming convention.
- The matrix records stable deployment topology. Logs, pod health, rollout state, and current metrics remain ephemeral operational evidence.

## Completion criterion

Deployment analysis is complete when every environment and deployable observed in versioned sources has one row, every row traces the trigger through its deployment artifact to the target or an exact unresolved indirection, contradictions are visible, and the repository note preserves the resulting matrix for future inspections.
