---
tags: [meta]
---

# Auditoría documental del vault

Este framework es la fuente de verdad para evidencia, scripts y gates del mapa de la célula, del dominio documental operacional y de los aprendizajes de ingeniería. El esquema y las relaciones viven en [[Convenciones]]; `map-ecosystem` mantiene el mapa, `manage-operational-workflow` ejecuta procedimientos con efectos externos, `generate-reports` genera reportes registrados, `manage-development-handoff` prepara tareas atómicas por repositorio y reconcilia su avance en la investigación, y `manage-investigation-derived-learning` mantiene enseñanzas acumulativas.

## Principio Obsidian-native

El vault es el grafo durable. The CLI derives relationships from Markdown/frontmatter and inspected external evidence; its local SQLite search index is reconstructible and is not a parallel authority. Una relación confirmada se escribe en la nota Obsidian correspondiente.

`.sync-acknowledgements.json` is the only versioned synchronization process record and stays outside the graph. It records an exact branch and commit only as `no-durable-node` (accepted new repository without a node) or `no-documentation-change` (accepted existing-repository delta requiring no documentation update). A rejected or limited attempt creates no acknowledgement and advances no successful cursor. Acknowledgements preserve the note baseline and contain no technical facts, relationships, findings, free-form reasons, or secrets.

## Jerarquía de evidencia

Usar este orden hasta sostener la afirmación o delimitar la ausencia:

1. Repo analizado: código, tests, migraciones, schemas y configs. README y otra documentación aportan descripción cuando es coherente con la implementación; los títulos o plantillas vacías no prueban comportamiento.
2. Versioned deployment definitions: build, release, provisioning and runtime configuration. Inspect the definitions needed to resolve connectors, triggers and configuration that changes the mapped flow. A complete per-environment deployment chain belongs to an explicitly requested deployment audit; unresolved indirection does not block independent local-map claims.
3. Búsqueda cross-repo en las `SOURCE_ROOTS` ordenadas del `source_context` derivado de la configuración local: módulo, repo, topic, subscription, env var, endpoint, datos, scheduler y service name. Las raíces no administradas son de solo lectura.
4. Read-only control-plane metadata through the configured provider access procedure, limited to the exact target and metadata needed for the claim.
5. Vault: notas auditadas, backlinks y Bases como vistas derivadas.

Evidence categories: `verificado-codigo`, `verificado-cross-repo`, `verificado-runtime`, `verificado-vault`, `inferido` and `no-verificado`. Runtime evidence records the observed provider, target, executor and time. Existing evidence labels retain their original meaning; changing terminology does not establish a new observation.

Quedan fuera del sync técnico: logs, pruebas runtime/manuales, datos productivos, trackers, documentación externa y conversaciones con owners. Un procedimiento runtime configurado puede consultar estado y logs acotados como evidencia efímera de una investigación operacional; no los convierte en hechos durables ni actualiza automáticamente el grafo. Una investigación puede consultar datos productivos únicamente por la excepción explícita y read-only de `inspect-database`; ese resultado es evidencia de la investigación, no una fuente automática del sync. Al alcanzar el alcance acotado o un límite de acceso, redactar la ausencia como “no observado en fuentes estáticas revisadas”; reservar `#por-confirmar` para límites que afectan el entendimiento de negocio.

## Gate de realidad productiva

Applicability: this section governs `production-gate` and the technical half of `mixed`. For `documented-source`, use [[evidence-policy]]: inspected versioned sources suffice for source-level assertions; actual deployment claims still require deployment evidence.

Source repository maps follow the source-map rule in [[evidence-policy]]: code and configuration at the exact repository reference-branch commit can describe purpose, main flows and connectors without asserting deployment. This exception does not promote future proposals or prove productive execution. For production assertions and investigation promotion under the production profiles, establish both conditions:

1. **Implementación observada**: código, schema/migración, configuración, contrato o recurso que materializa el comportamiento.
2. **Aplicabilidad productiva**: evidencia que vincula esa implementación con producción según su naturaleza, como la cadena de deploy por ambiente, configuración productiva versionada, metadata del control-plane, runtime identificado o estado de esquema validado mediante la autoridad de base de datos correspondiente.

The repository baseline follows [[reference-branches]]; its branch name alone does not prove deployment. Tampoco satisfacen el gate una investigación, una decisión, un work item, una conversación, un diseño aprobado, una prueba aislada, un pull request o un commit sin vínculo productivo. Para contratos entre unidades, la evidencia debe sostener los participantes y el enlace productivo que se documenta, no solo uno de sus extremos.

Las investigaciones se clasifican como orientadas a conocimiento, desarrollo o mixtas. Pueden registrar evidencia, decisiones y candidatos documentales, pero `map-ecosystem` debe volver a inspeccionar las fuentes autoritativas después del despliegue. En una investigación mixta, únicamente el estado actual que supere este gate puede ser candidato; el estado futuro permanece fuera del vault.

Si cualquiera de las dos condiciones falla, la decisión documental es **sin cambio**. `#por-confirmar` expresa una limitación sobre algo productivo ya observado; no conserva propuestas, diseños futuros ni implementaciones pendientes.

For a configuration assertion, versioned production configuration satisfies applicability at the analyzed commit: describe what it configures and qualify unresolved deployment separately. Claims about effective runtime, deployed destinations, or executed behavior require evidence for those stronger assertions. An inaccessible deployment definition limits only dependent claims; it does not suppress independently observed configuration changes.

## Gate de aprendizaje durable

Este gate decide si una investigación aporta una enseñanza versionable en `70-Aprendizajes/`. Su evaluación es siempre de solo lectura y no presupone que exista conocimiento. La investigación orienta hacia fuentes, historias, pruebas, implementación y despliegue, pero su narrativa no es prueba autónoma.

Una conclusión es `extractable` solo cuando cumple todas las condiciones aplicables:

1. **Identidad reutilizable**: formula una pregunta o problema estable y un contexto material de aplicabilidad; no es una bitácora, resumen de sesión, obviedad ni detalle exclusivo de un caso.
2. **Evidencia durable y reconsultable**: cada afirmación decisiva traza a fuentes inspeccionadas que otro agente autorizado puede revisitar, o a un método cuyo procedimiento, entradas y criterios están conservados en una fuente durable/versionada o pueden reejecutarse desde tooling versionado. Un expediente versionado bajo `investigations/` conserva procedencia y decisiones, pero su narrativa no prueba comportamiento productivo. Memoria y artefactos locales transitorios sirven solo como pistas; la misma limitación aplica a `.investigations/`, `.investigations-private/`, `.operations/`, worktree `.handoff/` stores y `.plan/`.
3. **Comparación verificable**: identifica baseline, alternativas, entorno, escala o muestra/ventana, controles, métricas o criterios observables y exclusiones capaces de cambiar el resultado. Conserva resultados positivos, negativos y neutrales.
4. **Conclusión sostenida**: la decisión y su justificación se derivan de los resultados sin ocultar contradicciones ni generalizar más allá de lo medido.
5. **Límites explícitos**: declara cuándo aplica, cuándo no aplica, limitaciones aceptadas y eventos que obligan a revalidar.
6. **Dedupe y acumulación**: busca conocimiento existente por pregunta y contexto, y selecciona exactamente una acción: `create`, `enrich`, `challenge` o `supersede`. Una investigación o fecha nueva no justifica otra nota.
7. **Implementación cuando corresponde**: `cambio-adoptado` identifica la implementación exacta y supera además el [[#Gate de realidad productiva|gate de realidad productiva]]. `baseline-conservado`, `alternativa-descartada` y `hallazgo-metodologico` no exigen un despliegue nuevo, pero sí el baseline autoritativo y la comparación durable.
8. **Publicación segura**: no persiste secretos, datos personales directos, filas productivas, logs crudos ni adjuntos sensibles; resume únicamente lo necesario y enlaza referencias seguras.

Select the assessment outcome and lifecycle action through the `manage-investigation-derived-learning/references/assessment-contract.md`, **Assessment outcomes**, under `.agents/skills/`. That contract owns the effect of unresolved case questions, valid no-write outcomes, deduplication and challenge eligibility. Apply the evidence conditions above to the selected conclusion; publication remains subject to the user's authorization.

## Evidencia operacional

Las notas de `60-Operacion/` se verifican contra la autoridad pertinente: estado read-only de la plataforma configurada, documentación corporativa vigente o confirmación explícita del owner responsable. Registrar un owner real y actualizar `ultima-verificacion` solo después de esa comprobación. Mientras falten proyectos, dashboards, recipients, canales, permisos o reglas corporativas, mantener la nota como `borrador` y describir la limitación sin inventar destinos.

For `clase: reporte`, use the `manage-operational-workflow/references/procedure-contract.md`, **Report**, under `.agents/skills/`. It defines mode-specific completeness without imposing an executor or output format. Before executing any procedure, apply the same contract's **Execution eligibility** section.

Los tickets, correos y mensajes prueban el resultado de una ejecución, pero no sustituyen la norma versionada. `.operations/` es estado local reanudable, queda fuera del grafo y de los gates documentales, y no debe contener secretos ni cuerpos sensibles.

## Evidencia de reconciliación de desarrollo

`.handoff/deltas.md` conserva únicamente cómo cambió o se complementó la definición de la tarea: tipo, detalle y evidencia. El repositorio registra allí esas deltas y el avance vive en la rama (commits, pruebas, pull request); no escribe en el vault. `manage-development-handoff` lee el worktree con `handoff status` (estado, commits por trailer `Handoff:` y deltas de cada tarea) y `handoff reconcile` importa a la investigación, con un mapeo fijo, los commits y deltas posteriores a la última marca de reconciliación del registro `DH-NNN`; una delta sin evidencia entra como pregunta. Un cambio material sin delta bloquea la reconciliación; un archivo de deltas vacío nunca demuestra por sí solo que el alcance no cambió.

La reconciliación consulta como máximo un salto de relaciones tipadas cuando el tracker las soporta para identificar work items directamente dependientes y entrega el contrato que cada uno necesita, su readiness y el estado exacto de repo/rama/PR. Rama local, ref remota, pull request, merge y despliegue son estados separados. Todo este material sigue siendo contexto futuro o no desplegado: actualizar el expediente no satisface el gate de realidad productiva ni autoriza una escritura técnica en el vault.

## Vistas operativas

- [[Repos.base]]: índice y frescura de repos.
- [[Arquitectura.base]]: servicios, componentes excepcionales y runtime.
- [[Auditoria.base]]: cobertura y relaciones de repos.
- [[Operacion.base]]: operaciones por área, clase, estado y reportes.
- [[Aprendizajes]]: índice curado de conclusiones reutilizables y sus estados.

## Native kernel operations

The shared contracts [[vault-resolution]], [[node-selection]] and [[work-item-evidence]] define vault identity, node selection and bounded work-item evidence. Provider-specific access remains configured through procedures in the destination vault.

Select `<cli>` through `use-vault-cli` (`.agents/skills/use-vault-cli/SKILL.md`); it defines the executable path and shell invocation. Resolve the vault before using relative paths. Command help supplies argument details; the owning skill supplies the investigation and authorization workflow.

| Operation | When and output | Boundary |
|---|---|---|
| `<cli> config resolve --vault "<vault_root>"` | Resolve canonical identity and configured sources before vault-dependent work. | Read-only; configuration changes belong to `onboard-developer`. |
| `<cli> config status --vault "<vault_root>"` | Validate instance identity and inspect configured capabilities. | Missing capabilities remain explicit; no procedure is executed. |
| `<cli> config workspace --vault "<vault_root>"` | Read local repository, worktree and proxy configuration. | Local settings preserve consumer namespaces; credentials and remote target identities stay outside this file. |
| `<cli> config areas --vault "<vault_root>"`, `config reports`, `config operation` | Discover areas/reports and resolve a procedure by basename or report ID. | Derived catalog; executing a procedure requires its skill and authority. |
| `<cli> search --vault "<vault_root>" --query "<terms>"` | Find bounded source pointers; the CLI refreshes its local index before searching. | Open the source before using it as evidence. Markdown remains authoritative. |
| `<cli> links --vault "<vault_root>" --node "<basename>"` | Inspect graph neighbors for propagation. | Relations derive from notes, not a parallel ledger. |
| `<cli> audit --vault "<vault_root>"` | Check closed note schemas, sections, learning evidence structure, operational topology and process leakage. | Structure does not prove business truth, experimental quality or external state. |
| `<cli> check links --vault "<vault_root>"` | Find broken links, alias targets, hidden-agent links and unexpected orphans. | Filesystem fallback does not replicate all Obsidian parsing. |
| `<cli> check obsidian-binding --vault "<vault_root>" --vault-name "<obsidian_vault>"` | Verify explicit Obsidian name against the canonical filesystem root before native queries. | Requires Obsidian CLI; a zero exit code alone does not prove the binding. |
| `<cli> check bases --vault "<vault_root>"` | Validate Bases after changing a Base, schema or property. | Does not execute expressions or plugin-defined functions. |
| `<cli> inventory --vault "<vault_root>" [--github-user <login>]` | Compare repository freshness and synchronization cursors before and after sync. | Requires authenticated `gh` and network; cursors are process state, not technical evidence. |
| `<cli> discover run`, `discover check --note <note>` | Extract connection facts at an exact commit; gate a note: anchors resolve, identifiers are in the cited lines, every discovered connector and resource is addressed. | Facts are evidence pointers; the checks prove anchors and coverage, not the correctness of the prose. |
| `<cli> sync start`, `review`, `verify`, `finish` | Publish knowledge on a `sync/` branch: review bound to the exact content by digest, gates versus the base, fast-forward merge. | A recorded review is the reviewer's verdict, not proof; remote publication follows the Git policy. |
| `<cli> investigation new\|list\|check\|add\|state\|absorb\|close\|reopen --vault "<vault_root>"` | Every case change: open from the formalized request; record evidence (source and level, files attached to the record), conclusions (level and basis, marked for the vault), questions, decisions, requirements and handoffs with assigned IDs and a log. Each write is refused when it would introduce a gate error (unsourced evidence, undefined reference, broken link, copied vault text, credential, local path). | `sync verify` runs `check` on changed cases (introduced errors block). The CLI does not judge whether evidence is sufficient or the language neutral; the independent review does. |
| `<cli> handoff start\|status\|refresh\|reconcile --vault "<vault_root>"` | Prepare a worktree for task packages (tasks of one branch share it, dependencies start first), read per-task state, commits and deltas, refresh a changed task, and import commits and deltas since the last mark into the case with a fixed mapping (preview, then `--apply`). | Never commits, pushes or fetches; the managed segment in `AGENTS.md` is identical in every repository. |
| `<cli> check visual`, `check visual-context` | Check visual structure and required context. | Browser rendering remains a separate skill responsibility; browser resources stay in the skill. |

For deployment/configuration evidence, inspect the exact source files and relevant cross-repository references using the source context. Search hits identify inspection targets; they do not establish a deployment chain or verified dependency.

## Resolución de identidad GitHub

`vaultctl inventory` no asume que la cuenta global activa de `gh` corresponde al repositorio actual. Resuelve una identidad por ejecución, valida acceso a la organización y usa el mismo token efímero para inventario, ramas y resolución de repos ausentes. Nunca ejecuta `gh auth switch`, persiste usuarios/tokens ni imprime secretos.

La precedencia es:

1. `--github-user <login>`: selección explícita de una cuenta ya almacenada por `gh`; prevalece sobre tokens de ambiente.
2. `GH_TOKEN` o `GITHUB_TOKEN`: identidad no interactiva para CI o automatización. Si no accede a la organización, el comando falla sin probar cuentas personales.
3. Cuenta activa de `gh`, solo cuando la verificación de acceso resulta positiva.
4. Única cuenta inactiva autenticada que sí accede a la organización.

Si varias cuentas inactivas tienen acceso, el comando termina con exit code 2 y lista únicamente sus logins para que el usuario repita con `--github-user`. Si ninguna funciona, indica autenticar `gh` y revisar permisos o autorización SSO. Si `gh` no puede validar las cuentas por conectividad u otro error operativo, lo reporta separadamente y no lo presenta como falta de autenticación. Reference branches follow [[reference-branches]]. Their commit SHAs are read through paginated GraphQL with that same identity, independently of the Git credential helper.

## Gates

Run the installed binary against the resolved absolute vault root. Consumer closure uses installed operations; distribution unit tests and Python evaluation tooling run only in the distribution checkout. A normal sync runs the final inventory, structural audit, link check and available Obsidian checks. Run Bases validation when a Base, schema or property changed.

```text
<cli> config status --vault "<vault_root>"
<cli> config workspace --vault "<vault_root>"
<cli> audit --vault "<vault_root>"
<cli> check links --vault "<vault_root>"
<cli> check bases --vault "<vault_root>"
```

Before any Obsidian-native query, run `<cli> check obsidian-binding --vault "<vault_root>" --vault-name "<obsidian_vault>"` and require success. It verifies that the explicit Obsidian name resolves to the same filesystem vault. A successful exit code alone is insufficient: Obsidian can return `Vault not found` with exit code zero. Once the binding is verified, run `obsidian "vault=<obsidian_vault>" unresolved` and `obsidian "vault=<obsidian_vault>" orphans`. If Obsidian is unavailable or its binding cannot be verified, record the limitation and use the native link check; never query another vault as fallback.

Después de crear o revisar materialmente una skill compleja, ejecutar además una prueba ciega en un contexto de agente fresco: entregar solo la ruta de la skill y una solicitud realista contra fixtures temporales, sin la respuesta esperada, diagnóstico previo ni conclusiones del autor. Una skill que escribe (casos, handoffs, notas) se prueba sobre copias temporales, con hashes o estado antes y después, sin tocar vaults ni repositorios reales. Si el entorno no ofrece aislamiento de agente, registrar esta validación como no observada; no simularla en el mismo contexto.

Consumer validation requires the installed native binary. Development dependencies, compiler checks and regression suites belong to the distribution checkout, never to the cell. Review changed diagrams and notes in Obsidian or the authorized browser when available, and report any unobserved rendering check.

La allowlist común de huérfanos esperados es `00-Home.md` y `README.md`: son entradas del vault, no fallas. `AGENTS.md` y cualquier `CLAUDE.md` propio de la celda quedan fuera del grafo documental. Scripts, plantillas y metadocumentos deben quedar enlazados desde este framework o desde otro índice.

Los expedientes `investigations/`, los no publicados `.investigations/`, su directorio privado en `.investigations-private/`, las ejecuciones `.operations/` y los handoffs (`.handoff/` en cada worktree) quedan fuera del grafo técnico y sus gates. El versionamiento del expediente no lo convierte en una fuente paralela de verdad técnica.

## Criterio de cierre

Un levantamiento o sincronización termina cuando:

- cada unidad y elemento de inventario tiene una decisión respaldada;
- cada afirmación técnica creada o modificada supera el gate de realidad productiva y ningún estado futuro quedó publicado;
- cada aprendizaje creado o modificado supera su gate independiente, conserva evidencia acumulativa y aplicabilidad, y toda evaluación no extractable terminó sin escritura;
- las notas cumplen [[Convenciones]] y separan hechos de límites;
- topics, integraciones, arquitectura, flujos y MOCs afectados fueron propagados;
- cuando cambia el dominio operacional, su índice, MOCs, Base, políticas, guías, catálogos, estándares, procedimientos, reportes, resolvedor y routing de skills fueron propagados;
- cuando cambia el dominio de aprendizajes, su índice, contrato, routing, notas relacionadas y validator fueron propagados;
- each published documentation update records its analyzed production branch and commit; accepted no-change acknowledgements and unaccepted reviews preserve the note baseline;
- each accepted repository without a published documentation change has an operational cursor matching its branch and SHA; `no-durable-node` follows an accepted review and cannot coexist with a note; `no-documentation-change` follows an accepted existing-repository review and requires a note; rejected or limited attempts leave the prior successful cursor unchanged;
- each updated deployable repository note preserves one row per environment/deployable with the deployment chain resolved or explicitly limited;
- los gates pertinentes a los archivos y contratos modificados pasan; una sync no convierte la suite completa de CI en etapas de análisis;
- a sync uses one source analysis to produce complete final-note candidates and one independent semantic review per repository/OID; candidate repairs remain local, materially changed meaning receives targeted re-review, and mechanical checks never reopen extraction; accepted identical baselines receive the appropriate acknowledgement while rejected or limited attempts remain unresolved without advancing a cursor; publication is resumable and writes only exact reviewed bytes;
- el reporte final distingue revisado, cambiado, sin aporte durable, límites de evidencia y pendientes reales de la ejecución, sin asignar remediaciones a los repositorios fuente.
