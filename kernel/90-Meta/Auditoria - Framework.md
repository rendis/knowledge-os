---
tags: [meta]
---

# Auditoría documental del vault

Este framework es la fuente de verdad para evidencia, scripts y gates del mapa de la célula, del dominio documental operacional y de los aprendizajes de ingeniería. El esquema y las relaciones viven en [[Convenciones]]; `map-ecosystem` mantiene el mapa, `manage-operational-workflow` ejecuta procedimientos con efectos externos, `generate-reports` genera reportes registrados, `reconcile-development-handoff` devuelve implementaciones a la investigación y `manage-investigation-derived-learning` mantiene enseñanzas acumulativas.

## Principio Obsidian-native

El vault es el grafo durable. Los scripts solo leen Markdown/frontmatter y evidencia externa verificable para reportar estado; no mantienen relaciones paralelas. Una relación confirmada se escribe en la nota Obsidian correspondiente.

`.sync-acknowledgements.json` es la única excepción procedimental versionada de la sync y no forma parte del grafo: conserva el cierre de una rama y commit exactos como `no-durable-node`, `review-rejected` o `inspection-limited`. Sólo `no-durable-node` expresa una decisión semántica aceptada; los otros dos son cursores operacionales y no alteran la trazabilidad semántica de una nota. Su nombre oculto evita incorporarlo a las vistas de Obsidian. No puede contener hechos técnicos, relaciones, hallazgos, razones libres ni secretos.

## Jerarquía de evidencia

Usar este orden hasta sostener la afirmación o delimitar la ausencia:

1. Repo analizado: código, tests, migraciones, schemas, configs y README solo como pista.
2. Deploy versionado: workflows, Cloud Build, Kubernetes/Kustomize, Cloud Run, Functions, Helm y Dockerfile. Para cada repo desplegable, interpretar Actions y sus referencias hasta resolver por ambiente el trigger, artefacto, proyecto, plataforma/recurso, ubicación y namespace/workload; un inventario de filenames no cierra esta evidencia.
3. Búsqueda cross-repo en las `SOURCE_ROOTS` ordenadas del `source_context` derivado de la configuración local: módulo, repo, topic, subscription, env var, endpoint, datos, scheduler y service name. Las raíces no administradas son de solo lectura.
4. GCP control-plane read-only: `list/describe` con salida limitada a metadata necesaria.
5. Vault: notas auditadas, backlinks y Bases como vistas derivadas.

Categorías: `verificado-codigo`, `verificado-cross-repo`, `verificado-gcp`, `verificado-vault`, `inferido` y `no-verificado`. Citar GCP como `verificado en GCP (gcloud, <project>, <date>)`.

Quedan fuera del sync técnico: logs, pruebas runtime/manuales, datos productivos, Jira, Confluence y conversaciones con owners. `inspect-gcp-runtime` puede consultar estado y logs acotados como evidencia efímera de una investigación operacional; no los convierte en hechos durables ni actualiza automáticamente el grafo. Una investigación puede consultar datos productivos únicamente por la excepción explícita y read-only de `inspect-database`; ese resultado es evidencia de la investigación, no una fuente automática del sync. Tras agotar las fuentes permitidas, redactar la ausencia como “no observado en fuentes estáticas revisadas”; reservar `#por-confirmar` para límites que afectan el entendimiento de negocio.

## Gate de realidad productiva

Antes de crear o modificar una afirmación del grafo técnico, demostrar dos condiciones:

1. **Implementación observada**: código, schema/migración, configuración, contrato o recurso que materializa el comportamiento.
2. **Aplicabilidad productiva**: evidencia que vincula esa implementación con producción según su naturaleza, como la cadena de deploy por ambiente, configuración productiva versionada, metadata del control-plane, runtime identificado o estado de esquema validado mediante la autoridad de base de datos correspondiente.

`main`/`master` es el baseline de análisis, no una prueba autónoma de despliegue. Tampoco satisfacen el gate una investigación, una decisión, una historia Jira, una conversación, un diseño aprobado, una prueba aislada, un pull request o un commit sin vínculo productivo. Para contratos entre unidades, la evidencia debe sostener los participantes y el enlace productivo que se documenta, no solo uno de sus extremos.

Las investigaciones se clasifican como orientadas a conocimiento, desarrollo o mixtas. Pueden registrar evidencia, decisiones y candidatos documentales, pero `map-ecosystem` debe volver a inspeccionar las fuentes autoritativas después del despliegue. En una investigación mixta, únicamente el estado actual que supere este gate puede ser candidato; el estado futuro permanece fuera del vault.

Si cualquiera de las dos condiciones falla, la decisión documental es **sin cambio**. `#por-confirmar` expresa una limitación sobre algo productivo ya observado; no conserva propuestas, diseños futuros ni implementaciones pendientes.

## Gate de aprendizaje durable

Este gate decide si una investigación aporta una enseñanza versionable en `70-Aprendizajes/`. Su evaluación es siempre de solo lectura y no presupone que exista conocimiento. La investigación orienta hacia fuentes, historias, pruebas, implementación y despliegue, pero su narrativa no es prueba autónoma.

Una conclusión es `extractable` solo cuando cumple todas las condiciones aplicables:

1. **Identidad reutilizable**: formula una pregunta o problema estable y un contexto material de aplicabilidad; no es una bitácora, resumen de sesión, obviedad ni detalle exclusivo de un caso.
2. **Evidencia durable y reconsultable**: cada afirmación decisiva traza a fuentes inspeccionadas que otro agente autorizado puede revisitar, o a un método cuyo procedimiento, entradas y criterios están conservados en una fuente durable/versionada o pueden reejecutarse desde tooling versionado. Memoria, aprobaciones, historias y artefactos locales transitorios sirven solo como pistas; ningún archivo bajo `.investigations/` puede ocupar `Fuentes durables`, incluidos manifests, runners, hashes, resultados o adjuntos. La misma prohibición aplica a workspaces locales ignorados como `.operations/`, `.knowledge-os-handoffs/` y `plan/`.
3. **Comparación verificable**: identifica baseline, alternativas, entorno, escala o muestra/ventana, controles, métricas o criterios observables y exclusiones capaces de cambiar el resultado. Conserva resultados positivos, negativos y neutrales.
4. **Conclusión sostenida**: la decisión y su justificación se derivan de los resultados sin ocultar contradicciones ni generalizar más allá de lo medido.
5. **Límites explícitos**: declara cuándo aplica, cuándo no aplica, limitaciones aceptadas y eventos que obligan a revalidar.
6. **Dedupe y acumulación**: busca conocimiento existente por pregunta y contexto, y selecciona exactamente una acción: `create`, `enrich`, `challenge` o `supersede`. Una investigación o fecha nueva no justifica otra nota.
7. **Implementación cuando corresponde**: `cambio-adoptado` identifica la implementación exacta y supera además el [[#Gate de realidad productiva|gate de realidad productiva]]. `baseline-conservado`, `alternativa-descartada` y `hallazgo-metodologico` no exigen un despliegue nuevo, pero sí el baseline autoritativo y la comparación durable.
8. **Publicación segura**: no persiste secretos, datos personales directos, filas productivas, logs crudos ni adjuntos sensibles; resume únicamente lo necesario y enlaza referencias seguras.

La evaluación entrega exactamente uno de estos resultados:

| Resultado | Significado | Acción |
|---|---|---|
| `extractable` | Todas las condiciones aplicables pasan y existe enseñanza nueva o materialmente útil. | `create`, `enrich`, `challenge` o `supersede`; escribir solo si el usuario solicitó/autorizó publicación. |
| `no-learning` | Las fuentes y el contexto bastan, pero no existe una enseñanza no obvia y reutilizable. | `none`; presentar el motivo, sin escritura. |
| `already-covered` | Una nota canónica ya conserva la misma enseñanza, contexto y evidencia equivalente o superior. | `none`; identificar la nota, sin actualización cosmética. |
| `insufficient-evidence` | Falta una fuente/contexto, la investigación no termina, la comparación no es reproducible o una posible contradicción aún no tiene evidencia suficiente para publicarse como `challenge`. | `none`; presentar el faltante exacto y la condición de reintento. |

`no-learning`, `already-covered` e `insufficient-evidence` son cierres válidos de una evaluación. No crean notas, drafts ni placeholders. Una revalidación sin aporte material es `already-covered` y no modifica fechas para simular actividad.

`challenge` solo es válido cuando la evidencia contradictoria supera de forma independiente cada condición aplicable de este gate y cambia materialmente el uso seguro de la enseñanza vigente. Una contradicción posible, ilegible, transitoria o metodológicamente débil permanece `insufficient-evidence` + `none` y no produce escritura.

## Evidencia operacional

Las notas de `60-Operacion/` se verifican contra la autoridad pertinente: estado read-only de Jira u otra plataforma, documentación corporativa vigente o confirmación explícita del owner responsable. Registrar un owner real y actualizar `ultima-verificacion` solo después de esa comprobación. Mientras falten proyectos, dashboards, recipients, canales, permisos o reglas corporativas, mantener la nota como `borrador` y describir la limitación sin inventar destinos.

Un contrato `clase: reporte` se verifica además contra una única receta del mismo `report-id`. Una ejecución live usa la query versionada, período cerrado, dry-run y cap de bytes; el manifest reconcilia query, filas, totales, workbook y validaciones sin ingresar al grafo. Los resultados productivos, CSV, Excel y manifests permanecen fuera del repositorio.

Los tickets, correos y mensajes prueban el resultado de una ejecución, pero no sustituyen la norma versionada. `.operations/` es estado local reanudable, queda fuera del grafo y de los gates documentales, y no debe contener secretos ni cuerpos sensibles.

## Evidencia de reconciliación de desarrollo

`implementation-updates.md` conserva únicamente cómo mutó o se complementó la definición exportada: fuente, cambio, justificación/acuerdo, impacto, evidencia y análisis solo cuando existió. `reconcile-development-handoff` debe contrastarlo con el baseline, la implementación, pruebas, rama remota/PR y Jira actuales. Un cambio material no registrado bloquea el retorno a la investigación; un changelog vacío nunca demuestra por sí solo que el alcance no cambió.

La reconciliación consulta un salto de enlaces Jira tipados para identificar historias directamente dependientes y entrega el contrato que cada una necesita, su readiness y el estado exacto de repo/rama/PR. Rama local, ref remota, pull request, merge y despliegue son estados separados. Todo este material sigue siendo contexto futuro o no desplegado: actualizar el expediente no satisface el gate de realidad productiva ni autoriza una escritura técnica en el vault.

## Vistas operativas

- [[Repos.base]]: índice y frescura de repos.
- [[Arquitectura.base]]: servicios, componentes excepcionales y runtime.
- [[Auditoria.base]]: cobertura y relaciones de repos.
- [[Operacion.base]]: operaciones por área, clase, estado y reportes.
- [[Aprendizajes]]: índice curado de conclusiones reutilizables y sus estados.

## Scripts reutilizables

La tabla usa `python3` como forma breve para el intérprete local; en Windows usar `py -3` o `python`. Los gates de cierre detallan abajo la invocación neutral entre shells.

| Script | Propósito | Cuándo usarlo | Comando | Entradas | Salidas | Modo | Limitaciones |
|---|---|---|---|---|---|---|---|
| [`audit-vault.py`](audit-vault.py) | Validar contratos cerrados de índices, sistemas, repos, arquitectura, topics, integraciones, flujos, glosario, operación y aprendizajes; impedir que estados internos de sync/review/gate entren al grafo durable | Después de cualquier cambio documental y antes del cierre | `python3 90-Meta/audit-vault.py` | Notas Markdown visibles; `--root` permite un fixture alternativo | Conteos, issues y exit code | Solo lectura | Valida estructura y separación entre conocimiento durable y proceso; no verdad de negocio, calidad experimental ni estado externo |
| [`operational-catalog.py`](operational-catalog.py) | Listar áreas/reportes y resolver notas operacionales recursivas por basename o `report-id` | Al seleccionar una operación, migrar un run heredado o verificar la bijección de reportería | `python3 90-Meta/operational-catalog.py <list-areas|list-reports|resolve>` | Markdown/frontmatter bajo `60-Operacion/`; `--root` acepta fixture | JSON/text ordenado o error estable | Solo lectura | Deriva el catálogo; no ejecuta operaciones ni reportes |
| [`verify-links.py`](verify-links.py) | Detectar wikilinks rotos, links dirigidos a aliases, enlaces desde notas visibles hacia `.agents/` y huérfanos inesperados | Después de renombres, migraciones o síntesis | `python3 90-Meta/verify-links.py` | Markdown visible del grafo y Bases; excluye instrucciones raíz, rutas ocultas y el directorio local `plan/`; `--root` permite un fixture alternativo | Broken, aliases, enlaces ocultos, orphans y exit code | Solo lectura | El fallback no replica todo el parser de Obsidian |
| [`validate-bases.py`](validate-bases.py) | Validar YAML y estructura determinista de las Bases | Después de modificar una Base, el esquema o sus properties | `python3 90-Meta/validate-bases.py` | Archivos `.base` raíz y properties del contrato u observadas en frontmatter visible; excluye rutas con segmentos `.`; `--root` permite un fixture alternativo | Conteo de Bases, issues y exit code | Solo lectura | No ejecuta expresiones ni valida funciones o vistas agregadas por plugins |
| [`workspace-config.py`](workspace-config.py) | Administrar la configuración local, proponer puertos loopback y exponer vistas semánticas de repositorios, root de worktrees y puertos por ambiente | Onboarding mediante `configure-workspace`; el root se configura cuando `development-worktree-root` reporta un gap y un puerto solo cuando el ambiente seleccionado devuelve `proxy_port_not_configured`; consumidores usan únicamente vistas semánticas | `<python> -B 90-Meta/workspace-config.py --vault-root <path> <command>` | `.knowledge-os-config.yaml`, raíces, repos Git locales, root opcional de worktrees y preferencias de puerto presentes | JSON, path, puerto estable o error semántico según la vista | Lectura por defecto; escritura local exclusiva de la skill de configuración y posterior a confirmación | Cada raíz representa el repo mismo si es Git o, en caso contrario, solo sus hijos inmediatos; `locate-repository` normaliza SSH/HTTPS, no recurre ni persiste caché y rechaza cero o varias coincidencias; el root de worktrees debe existir, ser dedicado y quedar fuera del vault y de todo repositorio o worktree Git; las capacidades ausentes siguen no configuradas; `suggest-proxy-ports` no reserva puertos; no almacena credenciales ni identidad de targets DB; preserva namespaces extensibles, rechaza secretos e identidades remotas incorrectas y solo reemplaza estado inválido con confirmación explícita |
| [`static-evidence-scan.py`](static-evidence-scan.py) | Barrido de deploy/config y referencias cross-repo | Antes de cerrar un levantamiento de repo | `python3 90-Meta/static-evidence-scan.py --repo <nota> [--source-repo <path>]` | Nota, `source_context` resuelto por la API central y un worktree exacto opcional ligado al clone root administrado | Markdown por stdout | Solo lectura | Heurístico; inventaría workflows/manifiestos pero no resuelve la cadena de deploy por ambiente; no consulta GCP, no acepta raíces alternativas y rechaza clones divergentes del mismo remote |
| `.agents/skills/map-ecosystem/scripts/resolve-vault.py` | Resolver el vault canónico y adjuntar la vista semántica de fuentes y autoridad de clone | Al iniciar la skill, antes de leer rutas relativas o usar Obsidian CLI | `python3 <directorio-de-la-skill>/scripts/resolve-vault.py [--path <vault>]` | Ruta explícita o actual, remotes Git, marcadores, registro opcional de Obsidian y API de configuración del vault | JSON con vault, `source_context`, modo de interacción y exit code | Solo lectura | No interpreta ni corrige el YAML local; deriva onboarding/reparación a `configure-workspace` |
| [`check-obsidian-binding.py`](check-obsidian-binding.py) | Verificar que el nombre explícito de Obsidian resuelve al path exacto del vault | Antes de `unresolved`, `orphans` o cualquier consulta Obsidian-native | `<python> -B 90-Meta/check-obsidian-binding.py --vault-root <path> --vault-name <name>` | `vault_root` y `obsidian_vault` entregados por el resolver | Estado normalizado, diagnóstico y exit code | Solo lectura | Requiere Obsidian CLI; no sustituye la validación de enlaces ni el render visual |
| [`vault-inventory.py`](vault-inventory.py) | Clasificar frescura y lifecycle de repositorios declarados en la instancia y cross-app aprobados, incluido el repositorio contenedor y los cursores de sync vigentes | Al iniciar y cerrar una sincronización | `python3 90-Meta/vault-inventory.py --format markdown [--github-user <login>]` | Notas, `.sync-acknowledgements.json`, GitHub CLI autenticado y remotos visibles de la organización | Reporte Markdown o JSON con identidad GitHub efímera; exit code operativo; el vault se clasifica `container` sin SHA autorreferente y un cursor vigente como el estado `acknowledged-*` correspondiente | Solo lectura | Requiere `gh` y red; la allowlist cross-app es cerrada; un cursor solo aporta estado procedimental y nunca verdad del grafo; no clona, no escribe ni cambia la cuenta global |
| [`git-change-manifest.py`](git-change-manifest.py) | Construir manifests Git deterministas, cerrar paquetes semánticos, derivar gate v2 desde checkpoints y validar proyecciones por unidad | Ejecutar `build` o `build-new`, `init-analysis` y `finalize-analysis` una vez por repo; `check` después de finalizar; review sólo para paquetes sin fallback; `close-package` una vez por resultado final; `gate-batch` una vez por conjunto de paquetes cerrados y `validate-projection` por unidad | `python3 -B 90-Meta/git-change-manifest.py build --repo <path> --old <commit> --new <commit>`; `python3 -B 90-Meta/git-change-manifest.py build-new --repo <path> --new <commit>`; `python3 -B 90-Meta/git-change-manifest.py init-analysis --manifest <manifest.json> --repository <name> [--node <basename>]...`; `python3 -B 90-Meta/git-change-manifest.py finalize-analysis --repo <path> --manifest <manifest.json> --scaffold <scaffold.json> --analysis <analysis.json> --output <finalize-result.json>`; `python3 -B 90-Meta/git-change-manifest.py check --repo <path> --manifest <manifest.json> --scaffold <scaffold.json> --analysis <analysis.json> --current-ref <explicit-branch-ref>`; `python3 -B 90-Meta/git-change-manifest.py close-package --repo <path> --manifest <manifest.json> --scaffold <scaffold.json> --analysis <analysis.json> --review <review.json> --production-ref <refs/heads/main-or-master-or-refs/remotes/origin/main-or-master> --analysis-date <YYYY-MM-DD> --output <package.json>`; `python3 -B 90-Meta/git-change-manifest.py gate-batch --output <gate.json> --expected-repository <name> [...] --package <package.json> [...]`; `python3 -B 90-Meta/git-change-manifest.py validate-projection --gate <gate.json> --projection <unit.json> --patch <unit.patch>` | Checkout Git exacto y commits; manifest, scaffold y candidato semántico v2; review v3 o fallback cerrado; referencia productiva canónica y fecha congeladas; paquete semántico validado; conjunto esperado; gate v2; projection v1 y patch de una unidad | `close-package` conserva el producto final sin output crudo; `gate-batch` deriva grants explícitos `repository + claim_id + node` sólo desde esos paquetes; `validate-projection` liga imágenes completas, cobertura de nodos, cursor, gate, patch, paths y grants y devuelve `projection-invalid` recuperable | Sólo lectura de fuentes salvo artefactos de trabajo autorizados; `finalize-analysis`, `close-package` y `gate-batch` escriben atómicamente sus salidas | Una claim no aceptada nunca autoriza prosa; `traceability-only` sólo autoriza acknowledgement; un fallo de proyección conserva paquetes y gate y se corrige en la misma corrida; acknowledgements y grupos se validan y aplican por separado mediante `sync-run.py` |
| [`sync-run.py`](sync-run.py) | Persistir y reanudar una corrida por paquetes y unidades independientes | Desde `begin` hasta `close`; después de una interrupción consultar `status` o `resume` sobre el mismo `run_id` | `python3 -B 90-Meta/sync-run.py <begin|checkpoint-package|seal-gate|status|validate-unit|apply-unit|resume|close> ...` | Digests de tooling/inventario, pares repositorio/OID, paquetes finalizados/redactados, gate v2, projection v1 y vault destino | JSON estable con estado, siguiente comando, invalidaciones y receipts | Escribe sólo el checkpoint ignorado y las rutas del vault autorizadas por la unidad validada | Un fallo recuperable conserva artefactos válidos; `close` exige todas las unidades aplicadas y elimina sólo el checkpoint activo de esa corrida |
| [`test_workspace_config.py`](test_workspace_config.py) | Probar migrate, merge de puertos y vistas semánticas del config local | Al modificar workspace-config o skills consumidoras | `python3 -B 90-Meta/test_workspace_config.py` | Fixtures YAML temporales | Resultado `unittest` | Escribe solo temporales | No abre proxies ni conecta a una DB real |
| `manage-investigation` — helper de expedientes | Aplicar las invariantes mecánicas de expedientes locales | Al abrir, consolidar o validar un expediente mediante `manage-investigation` | `<python> -B .agents/skills/manage-investigation/scripts/investigation-case.py --root .investigations <open|consolidate|validate>` | Expedientes Markdown locales; `open` recibe identidad, propósito, resultados documental/de aprendizaje y anclas exactas; `consolidate` exige el mapping semántico preparado por el agente | JSON estable y exit code; creación, archivo o validación según el subcomando | Escritura local transaccional para `open` y `consolidate`; lectura para `validate` | No decide equivalencia semántica, no crea índices y acepta con warning expedientes heredados sin `dedupe-key`, `purpose`, `vault-outcome`, `learning-outcome` o la separación estructural actual |
| `manage-development-handoff` — helper de materialización | Planificar y preparar en una sola autorización worktrees persistentes y su handoff inicial; mantener operaciones granulares para worktree-only, refresh, recuperación, validación y desactivación | Al transferir un paquete vigente de investigación/Jira a uno o varios repositorios | `<python> -B .agents/skills/manage-development-handoff/scripts/development-handoff.py --vault-root <path> <plan-handoff|prepare-handoff|plan-worktree|create-worktree|plan|apply|validate|deactivate>` | Paquete one-way validado, remote Git, base confirmada, root semántico de worktrees y estado local del repo/worktree | JSON estable con commits local/remoto, branch/path, efectos Git y de archivos, token completo, identidad, revisión, límite de reanudación y validación | `plan-handoff` inspecciona commits faltantes solo en temporales y no deja estado persistente; `prepare-handoff` revalida un token completo, ejecuta y verifica ambas fases; `plan-worktree`/`plan`/`validate` son de solo lectura persistente; `create-worktree` y `apply` conservan las rutas granulares; `deactivate` elimina `ACTIVE.yaml` y reconcilia la política cuando está stale | No lee el expediente ni Jira, no mueve la base local, no cambia código, commit o remoto, nunca sobrescribe `implementation-updates.md`, preserva el worktree como resume boundary ante fallo de materialización y no hace rollback amplio entre repositorios |
| `inspect-gcp-runtime` — validador de comandos | Rechazar mutaciones, cambios de identidad/configuración, scopes implícitos y lecturas sin límites antes de ejecutar gcloud o kubectl | Antes de todo comando propuesto por `inspect-gcp-runtime` | `<python> -B .agents/skills/inspect-gcp-runtime/scripts/validate-runtime-command.py --command "<candidate>"` | Un único comando candidato ya resuelto contra el target card | JSON `allowed`/`reason` y exit code | Solo lectura; nunca ejecuta el candidato | Valida la envolvente mecánica, no la sintaxis leaf ni la semántica del target |
| `manage-operational-workflow` — preparador de links Jira `Blocks` | Traducir aristas semánticas `BLOCKER → BLOCKED` a payloads Jira sin inversión y producir aserciones de lectura desde ambos extremos | Antes de previsualizar y otra vez antes de crear cualquier link `Blocks` | `<python> -B .agents/skills/manage-operational-workflow/scripts/prepare-jira-blocks-links.py --type-name Blocks --outward-description blocks --inward-description "is blocked by" --edge <BLOCKER> <BLOCKED>` | Metadata viva del tipo de enlace y pares blocker/blocked autorizables | JSON con payloads y aserciones, o estado `blocked`; exit code | Solo lectura | No consulta ni modifica Jira; la skill debe obtener metadata viva y ejecutar el payload autorizado |
| `generate-reports` — runner | Listar, generar o validar un reporte registrado | Al producir un Excel registrado o validar su paquete | `<python> -B .agents/skills/generate-reports/scripts/run_report.py <list|generate|validate>` | `report-id`, meses inclusivos obligatorios, salida y opcional fixture; live usa receta BigQuery fija | XLSX + manifest trazable publicados atómicamente, o JSON de validación | Fixture local o BigQuery read-only acotado | No publica externamente, programa ni envía; requiere XlsxWriter fijado |

Los tres CLIs de frontmatter comparten el lector [`vault_frontmatter.py`](vault_frontmatter.py), que soporta el subconjunto versionado de YAML sin dependencias externas y no expone una CLI propia.

## Resolución de identidad GitHub

`vault-inventory.py` no asume que la cuenta global activa de `gh` corresponde al repositorio actual. Resuelve una identidad por ejecución, valida acceso a la organización y usa el mismo token efímero para inventario, ramas y resolución de repos ausentes. Nunca ejecuta `gh auth switch`, persiste usuarios/tokens ni imprime secretos.

La precedencia es:

1. `--github-user <login>`: selección explícita de una cuenta ya almacenada por `gh`; prevalece sobre tokens de ambiente.
2. `GH_TOKEN` o `GITHUB_TOKEN`: identidad no interactiva para CI o automatización. Si no accede a la organización, el comando falla sin probar cuentas personales.
3. Cuenta activa de `gh`, solo cuando la verificación de acceso resulta positiva.
4. Única cuenta inactiva autenticada que sí accede a la organización.

Si varias cuentas inactivas tienen acceso, el comando termina con exit code 2 y lista únicamente sus logins para que el usuario repita con `--github-user`. Si ninguna funciona, indica autenticar `gh` y revisar permisos o autorización SSO. Si `gh` no puede validar las cuentas por conectividad u otro error operativo, lo reporta separadamente y no lo presenta como falta de autenticación. Los SHA de `main` y, solo en su ausencia, `master` se obtienen mediante GraphQL paginado con esa misma identidad; no dependen del credential helper de Git.

## Gates

Ejecutar desde la raíz con el mismo intérprete usado por el resolver. En los comandos siguientes, `<python>` significa `python3` en macOS/Linux, `py -3` o `python` en Windows y `python` en CI. La forma por argumentos funciona en bash, zsh, fish y PowerShell; reemplazar los placeholders entre ángulos y mantener entre comillas los paths/nombres con espacios.

La suite de cierre de una celda contiene sólo comandos instalados. Las pruebas de distribución y de desarrollo del tooling se ejecutan en el repositorio de la distribución, no desde una celda consumidora. Una sync normal ejecuta segundo inventario, auditoría, links y cierre Obsidian; ejecuta Bases sólo si cambió una Base, esquema o property.

```text
<python> -B 90-Meta/test_instance.py
<python> -B 90-Meta/test_workspace_config.py
<python> -B 90-Meta/audit-vault.py
<python> -B 90-Meta/verify-links.py
<python> -B 90-Meta/validate-bases.py
<python> -B 90-Meta/check-obsidian-binding.py --vault-root "<vault_root>" --vault-name "<obsidian_vault>"
obsidian "vault=<obsidian_vault>" unresolved
obsidian "vault=<obsidian_vault>" orphans
```

El binding debe pasar antes de `unresolved`/`orphans`: Obsidian puede imprimir `Vault not found` con exit code `0`, por lo que el exit code del CLI por sí solo no es evidencia. El helper compara identidad de filesystem y normaliza symlinks, separadores y case según la plataforma.

Después de crear o revisar materialmente una skill compleja, ejecutar además una prueba ciega en un contexto de agente fresco: entregar solo la ruta de la skill y una solicitud realista contra fixtures temporales, sin la respuesta esperada, diagnóstico previo ni conclusiones del autor. Para `manage-development-handoff`, comprobar primero en modo read-only que una solicitud por investigación e historia sin paquete se clasifique `producer-required`, se entregue al productor y conserve la reanudación del consumidor sin abrir el caso; detener esa variante antes de cualquier escritura. Para probar el helper, limitar la ejecución a `plan-handoff`, a `plan-worktree` worktree-only o a `plan` sobre un worktree temporal ya existente, revisar que reconstruya el contrato y todos los efectos desde los artefactos, y no permitir ninguna aplicación ni escritura sobre repositorios reales. Para `reconcile-development-handoff`, usar un handoff/worktree/Jira simulados o copias temporales, exigir que detecte un delta material ausente y que no cree `return/`, modifique Jira ni presente una rama local como publicada; la escritura del expediente se reemplaza por la inspección del contexto normalizado devuelto. Para `manage-investigation-derived-learning`, usar **Assess** sobre una investigación real en modo de solo lectura o una copia temporal, capturar hashes/estado antes y después y prohibir publicación, actualización del expediente o creación de notas; la prueba valida la decisión comunicada, no persiste su resultado. Si el entorno no ofrece aislamiento de agente, registrar esta validación como no observada; no simularla en el mismo contexto.

CI instala la dependencia fijada en [`requirements-ci.txt`](requirements-ci.txt) antes de ejecutar los gates. Localmente, la instalación con `<python> -m pip install --no-deps -r 90-Meta/requirements-ci.txt` es un paso de preparación que modifica el entorno: ejecutarlo solo cuando el usuario pidió preparar o corregir el entorno, o lo autorizó explícitamente. Un preflight read-only comprueba la dependencia y ofrece esta remediación sin ejecutarla. Revisar además en Obsidian los diagramas/notas modificados. Si la app no está disponible, registrar la limitación y usar el fallback.

La allowlist común de huérfanos esperados es `00-Home.md` y `README.md`: son entradas del vault, no fallas. Los routers de instrucciones `AGENTS.md` y `CLAUDE.md` quedan fuera del grafo documental. Scripts, plantillas y metadocumentos deben quedar enlazados desde este framework o desde otro índice.

Las ejecuciones bajo `.operations/`, los expedientes `.investigations/` y los handoffs `.knowledge-os-handoffs/` quedan fuera de la indexación, del grafo y de los gates. Su exclusión no autoriza convertirlos en una fuente paralela de verdad.

## Criterio de cierre

Un levantamiento o sincronización termina cuando:

- cada unidad y elemento de inventario tiene una decisión respaldada;
- cada afirmación técnica creada o modificada supera el gate de realidad productiva y ningún estado futuro quedó publicado;
- cada aprendizaje creado o modificado supera su gate independiente, conserva evidencia acumulativa y aplicabilidad, y toda evaluación no extractable terminó sin escritura;
- las notas cumplen [[Convenciones]] y separan hechos de límites;
- topics, integraciones, arquitectura, flujos y MOCs afectados fueron propagados;
- cuando cambia el dominio operacional, su índice, MOCs, Base, guías, catálogos, estándares, procedimientos, reportes, resolvedor y routing de skills fueron propagados;
- cuando cambia el dominio de aprendizajes, su índice, contrato, routing, notas relacionadas y validator fueron propagados;
- la trazabilidad de cada paquete aceptado coincide con su rama productiva; una review no aceptada conserva la trazabilidad semántica anterior;
- cada repositorio inspeccionado sin paquete semántico aceptado tiene un cursor operacional coincidente con su rama y SHA; `no-durable-node` existe sólo tras una review aceptada y no coexiste con una nota;
- cada nota de repo desplegable reanalizada conserva una fila por ambiente/deployable con la cadena de deploy resuelta o una limitación explícita;
- los gates pertinentes a los archivos y contratos modificados pasan; una sync no convierte la suite completa de CI en etapas de análisis;
- una sync consumió exactamente una extracción, una finalización con recibo y como máximo una review por repositorio/OID; una review parcial publicó únicamente sus claims no cuestionadas y cualquier paquete sin claims aceptadas terminó en `review-rejected` o `inspection-limited`, siempre sin segunda pasada; cada unidad pasó `validate-projection` y se aplicó idempotentemente mediante el mismo `run_id`;
- el reporte final distingue revisado, cambiado, sin aporte durable, límites de evidencia y pendientes reales de la ejecución, sin asignar remediaciones a los repositorios fuente.
