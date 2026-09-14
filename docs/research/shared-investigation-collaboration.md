# Investigaciones técnicas colaborativas sobre Git y Markdown

**Estado:** investigación de diseño, 2026-09-01

> Implementation decision (2026-09-13): the shipped first version deliberately keeps one `investigation.md` per case. The hybrid per-register file model discussed below is deferred as unnecessary complexity. Current contracts live in `kernel/.agents/skills/manage-investigation/` and use `investigations/` plus optional `.investigations-private/`.

**Alcance:** expediente compartido y versionado como fuente canónica; overlay privado exclusivamente para material no compartible; continuidad y colaboración concurrente eventual.

## Conclusión

El modelo más simple que evita dos fuentes de verdad no es sincronizar un expediente público con una copia privada. Es mantener **un único expediente compartido canónico** y un **overlay privado no autoritativo**, referenciado por IDs estables. El overlay puede complementar la lectura local, pero no puede definir ni sobrescribir estado, decisiones, evidencia, alcance o criterios de aceptación. Promover algo desde el overlay es una operación selectiva de redacción y revisión hacia un PR, no una sincronización bidireccional.

Para soportar concurrencia, conviene evolucionar desde el actual `investigation.md` monolítico a una estructura híbrida: un snapshot narrativo actual y pocos metadatos mutables, más registros materiales independientes e inmutables por archivo. Git resuelve conflictos textuales de tres vías; no detecta que dos cambios sin conflicto de líneas contradigan una decisión. Por eso el merge debe combinar revisión humana, validaciones semánticas y concurrencia optimista.

En este documento, **público** significa versionado y compartido bajo los permisos del repositorio; no implica necesariamente exposición en Internet. **Resolución manual** significa resolución semántica explícita y registrada, ejecutada por una persona o por un agente autorizado. Nunca significa aceptar como correcta la salida implícita de un merge textual limpio.

## Decisiones confirmadas

- El expediente público es la única fuente canónica de conocimiento, decisiones, estado, alcance y criterios de aceptación.
- La skill siempre debe descubrir si existe un overlay privado y considerarlo al reunir contexto, pero nunca usarlo como fallback para una escritura canónica.
- El overlay es opcional, local, ignorado y no autoritativo. Su uso se limita mediante una allowlist; no es un segundo expediente ni un espacio general de notas.
- No existe sincronización bidireccional público/privado. Promover contenido privado produce una contribución pública saneada y revisable.
- Todo cambio público material pasa por reconciliación semántica explícita, incluso cuando Git no reporta conflicto textual.
- La resolución puede ser humana o agéntica, pero debe identificar base, candidatos, conflicto, evidencia, decisión y resultado verificado.
- Las conversaciones crudas y la solicitud original literal dejan de persistirse como parte del expediente. Se conserva únicamente una formulación profesional, fiel y trazable de lo material.
- Las rutas locales dejan de ser coordenadas persistidas. El worktree se reconstruye desde identidad de repositorio, rama y configuración local, y luego se valida contra Git.

## Evidencia primaria y consecuencias de diseño

### 1. Merge documental y docs-as-code

**Hechos.** Git hace un merge de tres vías usando ancestro común, `HEAD` y la revisión entrante; en conflictos conserva esas tres versiones para resolución ([Git, `git merge`](https://git-scm.com/docs/git-merge)). Los atributos de Git permiten elegir o implementar drivers de merge por tipo de archivo, pero esos drivers siguen operando sobre tres representaciones del archivo, no sobre el significado del dominio ([Git, `gitattributes`](https://git-scm.com/docs/gitattributes)). La investigación original sobre prácticas de computación científica recomienda texto plano versionable, un documento maestro compartido, cambios pequeños y frecuentes, y reconoce que Git facilita combinar aportes concurrentes y conservar un rastro de revisión ([Wilson et al., *Good enough practices in scientific computing*](https://doi.org/10.1371/journal.pcbi.1005510)).

Nygard propone registros de decisión pequeños y modulares en Markdown dentro del repositorio, un registro por decisión, y conservar una decisión reemplazada marcándola como supersedida ([*Documenting Architecture Decisions*](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)).

La guía oficial de GitLab trata la documentación como única fuente de verdad y canaliza sus cambios mediante merge requests ([GitLab, *Documentation Style Guide*](https://docs.gitlab.com/development/documentation/styleguide/)). Su workflow separa revisión técnica, editorial y de mantenimiento; para contenido generado con IA mantiene al autor responsable de la exactitud y exige revisión editorial ([GitLab, *Documentation workflow*](https://docs.gitlab.com/development/documentation/workflow/)).

**Inferencia de diseño.** Un único Markdown grande es aceptable para lectura individual, pero se vuelve un punto caliente cuando varios colaboradores actualizan snapshot, registros e historia en las mismas zonas. Un archivo por cada elemento material reduce la probabilidad de conflictos textuales y permite revisar una contribución como unidad. No elimina los conflictos semánticos, por lo que no justifica un merge automático.

No se recomienda crear inicialmente un merge driver para Markdown: separar archivos y validar contratos es más simple y auditable. Un driver propio solo sería justificable si se dispone de un formato estructurado cuya combinación tenga reglas deterministas y pruebas exhaustivas.

### 2. Concurrencia optimista y conflictos semánticos

**Hechos.** El patrón `If-Match` de HTTP condiciona una mutación a que la representación observada siga vigente y se usa para evitar *lost updates* cuando varios actores modifican un recurso en paralelo ([RFC 9110, §13.1.1](https://www.rfc-editor.org/rfc/rfc9110.html#name-if-match)). En GitHub, una rama protegida puede exigir PR, revisiones y checks; también puede descartar aprobaciones cuando se agregan commits, y `CODEOWNERS` puede solicitar y exigir al responsable de los archivos modificados ([ramas protegidas](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches), [CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)).

**Inferencia de diseño.** `branch + PR` constituye una forma adecuada de concurrencia optimista: cada aporte declara la revisión pública sobre la que se preparó, CI verifica que la base no quedó obsoleta y el colaborador reconcilia antes del merge. El SHA del commit base o una huella del expediente cumple el papel conceptual del validador fuerte; un contador mutable global no es imprescindible.

Un merge limpio de Git no debe satisfacer por sí solo el gate. Para cada cambio se calculan dos deltas de dominio contra la misma base: `base → canonical-current` y `base → contribution`. Si ambos afectan la misma unidad semántica, o si uno invalida una dependencia del otro, la integración se bloquea hasta producir una reconciliación explícita. La ausencia de solapamiento textual no demuestra independencia semántica.

Hay dos clases de conflicto y deben producir resultados diferentes:

- **Textual:** Git no puede combinar las mismas líneas. El autor actualiza su rama y resuelve el diff.
- **Semántico:** Git combina el texto, pero quedan dos decisiones activas incompatibles, una evidencia cambia de significado, una transición de estado no es válida o el snapshot contradice sus registros. CI debe bloquear los casos mecanizables; una persona debe resolver el significado.

Validaciones mínimas de PR:

1. esquema, IDs, referencias y estados válidos;
2. IDs nuevos no duplicados y significado de IDs existentes inmutable;
3. enlaces recíprocos `supersedes` / `superseded-by`;
4. como máximo una decisión activa por clave de decisión cuando el dominio declare exclusividad;
5. snapshot reconciliado con los registros agregados o modificados;
6. base/fingerprint vigente para toda modificación de un registro existente;
7. escaneo de secretos y de datos locales no portables;
8. revisión requerida para cambios de decisión, seguridad o publicación.

La validación puede detectar contradicciones estructurales; no debe fingir que decide entre argumentos técnicos rivales.

Una reconciliación material debería registrar como mínimo:

- revisión base y revisiones candidatas;
- IDs y claves semánticas afectadas;
- hechos, decisiones o estados incompatibles;
- resolución seleccionada y alternativas descartadas;
- evidencia y limitaciones consideradas;
- identidad y autoridad del reviewer humano o agéntico;
- digest del resultado validado.

La rama canónica debe exigir el check y la revisión correspondiente. En un Git distribuido no se puede impedir que alguien cree un merge local arbitrario sin controlar también la rama receptora; sin protección remota, el contrato solo puede detectar y rechazar ese estado, no impedir que exista.

### 3. Provenance, autoría y decisiones

**Hechos.** W3C PROV distingue `Entity`, `Activity` y `Agent`, y modela uso, generación, derivación, atribución, asociación y delegación. También distingue revisiones y fuentes primarias como formas de derivación ([W3C PROV-O](https://www.w3.org/TR/prov-o/)). Los principios FAIR requieren identificadores persistentes, referencias cualificadas y provenance detallada para reutilizar objetos de investigación ([Wilkinson et al., 2016](https://doi.org/10.1038/sdata.2016.18)). CRediT ofrece roles de contribución en vez de reducir la autoría a una lista indiferenciada de nombres ([NISO CRediT](https://credit.niso.org/)).

**Inferencia de diseño.** Cada registro material debería responder, sin copiar conversaciones crudas:

- qué afirmación, decisión o pregunta representa;
- quién o qué agente la aportó y con qué rol (`investigator`, `reviewer`, `decision-maker`, `software-agent`);
- qué actividad la produjo (`observation`, `analysis`, `review`, `migration`);
- de qué fuente o registros deriva;
- cuándo fue observada y contra qué revisión;
- qué la reemplaza o invalida, si corresponde.

Formalizar el lenguaje no equivale a anonimizar. El expediente compartido debe transformar expresiones informales en afirmaciones neutrales y verificables, sin atribuir culpa ni conservar comentarios personales irrelevantes. Sin embargo, Git y el PR conservan autoría técnica; no se debe falsificarla. Cuando la identidad personal no sea necesaria en el contenido, basta un rol o equipo, mientras el sistema de control de versiones conserva la trazabilidad autorizada.

Las decisiones aceptadas no deben reescribirse para que parezca que siempre fueron distintas. Se crea una nueva decisión y se enlaza la reemplazada. Correcciones no materiales —ortografía o enlaces rotos— pueden modificar el archivo preservando su identidad.

La formalización automática debe producir un borrador revisable, no reemplazar la responsabilidad del contribuidor ni del reviewer. Antes de colaborar conviene declarar objetivo, roles, custodia de los datos, autoridad para cambiar decisiones y procedimiento de desacuerdo; estas son también prácticas explícitas de la guía de integridad para investigación colaborativa de la Office of Research Integrity de EE. UU. ([ORI, *Collaborative Research: Roles and Relationships*](https://ori.hhs.gov/content/Chapter-8-Collaborative-Research-Roles-and-relationships)).

### 4. Minimización y material sensible

**Hechos.** El principio de minimización exige que los datos personales sean adecuados, pertinentes y limitados a lo necesario para el propósito ([RGPD, art. 5(1)(c)](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32016R0679)). La protección por diseño y por defecto extiende esa obligación a cantidad, tratamiento, retención y accesibilidad ([RGPD, art. 25](https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32016R0679)). GitHub advierte que retirar información sensible de Git exige reescribir historia, coordinar clones y forks, cambia hashes y aun así no elimina copias externas; para secretos, la primera respuesta es revocarlos o rotarlos ([GitHub, *Removing sensitive data from a repository*](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository)). La protección de push bloquea secretos antes de que queden incorporados a la historia ([GitHub, *Push protection*](https://docs.github.com/en/code-security/concepts/secret-security/push-protection)).

**Inferencia de diseño.** La clasificación debe ocurrir antes del primer commit. El estado por defecto de una entrada nueva es “no publicable hasta revisar”, no “publicable salvo que alguien detecte un problema”. Al expediente compartido solo entra lo necesario para comprender, verificar o decidir.

Esto no es contrario a una investigación reproducible: la política del NIH distingue los datos necesarios para validar y replicar resultados de cuadernos, análisis preliminares, borradores y comunicaciones entre colegas que no forman parte del material científico que obliga a compartir ([NIH, *Data Management & Sharing Policy*](https://www.grants.nih.gov/policy-and-compliance/policy-topics/sharing-policies/dms/policy-overview)). Por analogía, el expediente necesita evidencia y contexto suficientes, no una transcripción total del proceso social.

Clasificación recomendada:

| Clase | Destino |
|---|---|
| Hecho, decisión, alcance o evidencia durable y saneada | Expediente compartido |
| Conversación informal útil solo como materia prima | No se persiste; se redacta una síntesis formal pública |
| Dato personal necesario para responsabilidad | Versión mínima, preferentemente rol/equipo |
| Dato sensible necesario para continuar y excluido del público | Overlay privado con referencia estable, propósito y retención; nunca valor secreto |
| Credencial, token, cookie o clave | Gestor de secretos; ni público ni overlay |
| Ruta absoluta, usuario local, hostname o caché | Se reemplaza por coordenadas portables |
| Juicio personal, culpa, ironía o desahogo | Se descarta; se conserva únicamente el hecho comprobable |

El escaneo automático es una defensa, no una autorización de publicación. También deben revisarse nombres de archivos, metadatos, adjuntos, imágenes, logs y diffs, no solo el cuerpo de Markdown.

### 5. Versionado y migración

**Hechos.** JSON Schema recomienda declarar explícitamente el dialecto para que lectores y herramientas conozcan la semántica usada ([JSON Schema, `$schema`](https://json-schema.org/understanding-json-schema/reference/schema)). Semantic Versioning exige declarar el contrato público y no modificar el contenido de una versión publicada ([SemVer 2.0.0](https://semver.org/)). UUID está diseñado para generar identificadores de 128 bits sin registro central; UUIDv7 incorpora orden temporal aproximado ([RFC 9562](https://www.rfc-editor.org/rfc/rfc9562.html)).

**Inferencia de diseño.** Cada expediente necesita `schema-version`; las herramientas deben declarar qué versiones leen y cuál escriben. Una migración debe ser explícita, idempotente, validable y producir un reporte por caso. SemVer puede aplicarse al contrato del expediente, no al contenido narrativo de cada investigación.

La política de evolución de APIs de Kubernetes exige que las representaciones soportadas puedan convertirse en ambos sentidos sin pérdida antes de retirar una versión, y mantener solapamiento entre versión anterior y nueva ([Kubernetes, *API deprecation policy*](https://kubernetes.io/docs/reference/using-api/deprecation-policy/)). La aplicación aquí es temporal: primero se introduce un lector compatible y conversores de esquema con round-trip probado; después se migran casos; por último se cambia el writer canónico y se retira el formato anterior.

No conviene mantener un migrador bidireccional entre público y privado. Eso permitiría que ambos lados avancen y obliga a inventar reglas de reconciliación para dos autoridades. Se requieren dos operaciones diferentes:

- **Migrar una vez:** transformar el expediente local heredado en expediente compartido más overlay privado.
- **Promover selectivamente:** redactar material del overlay como candidato público, sanearlo y someterlo a PR.

Por tanto, “bidireccional” solo es deseable durante la transición **entre versiones del esquema** para demostrar que no se pierde información. No lo es **entre los dos espacios**, porque eso convertiría el overlay en una réplica con capacidad de divergir.

## Evaluación de las alternativas clave

| Decisión | Evaluación | Recomendación |
|---|---|---|
| Un documento | Lectura simple; alta contención; cambios no relacionados tocan el mismo archivo | Mantener solo el snapshot narrativo como documento principal |
| Archivo por registro | Menos colisiones, ownership y revisión granular; más archivos y necesidad de índice/vista | Usarlo para evidencia, decisiones, preguntas y eventos materiales |
| Branch/PR + concurrencia optimista | Usa capacidades nativas, auditable y escalable; obliga a reconciliar ramas obsoletas | Sí, con rama protegida, checks y revisión semántica |
| IDs incrementales `E-001` | Legibles; dos ramas pueden reservar el mismo próximo número | Preservarlos en casos migrados; para nuevos registros usar IDs sin coordinación |
| UUID por registro | No requiere autoridad central; peor lectura humana | UUIDv7 como identidad; título corto como etiqueta humana |
| Historia solo append-only | Excelente auditoría; costosa para conocer “qué vale ahora” | No usarla sola |
| Solo snapshot actual | Excelente consumo; pierde razón, reemplazos y provenance de dominio | No usarlo solo |
| Snapshot + registros append-only | Lectura inmediata y trazabilidad explícita | Sí; el snapshot es una proyección curada del conocimiento vigente |
| Overlay como copia privada completa | Puede quedar más avanzado y competir con el público | Rechazar |
| Overlay como complemento referenciado | Protege material no compartible sin duplicar decisiones | Sí, con campos permitidos y prohibidos explícitos |

## Alternativa histórica descartada para `documentation-vault`

The multi-file model below was researched but rejected for the first implementation. It is retained only as historical design context; it is not the current contract and `.investigations/` below must not be interpreted as the shipped public path.

### Autoridad y estructura

Estructura conceptual inicial:

```text
.investigations/<case-id>/                 # compartido y versionado
├── case.yaml                              # identidad, schema, lifecycle, revisión
├── current.md                             # snapshot canónico y consumible
├── evidence/<record-id>.md                # un hecho/inferencia/limitación por registro
├── decisions/<record-id>.md               # una decisión y sus enlaces de reemplazo
├── questions/<record-id>.md               # una pregunta y su resolución
├── events/<event-id>.yaml                 # cambios materiales del dominio
├── artifacts/                             # solo artefactos saneados y aprobados
└── exports/                               # derivados con revisión fuente

<ignored-private-root>/<case-id>/          # local, ignorado y no autoritativo
├── overlay.yaml                           # case-id + public-revision + política
├── notes/                                 # material sensible/no compartible
└── artifacts/                             # material local controlado
```

`current.md` es la respuesta a “¿qué sabemos y decidimos ahora?”. Los registros son la respuesta a “¿por qué, a partir de qué y quién lo revisó?”. Los índices y vistas se derivan; no son almacenes alternativos.

El overlay:

- referencia `case-id`, IDs canónicos y una revisión pública observada;
- puede guardar únicamente referencias de acceso, observaciones sensibles imprescindibles y artefactos no compartibles necesarios para continuar;
- no puede declarar `status`, `scope`, `decision`, `acceptance-criteria`, evidencia pública ni reemplazos;
- no conserva por defecto conversaciones crudas, borradores generales, copias completas del expediente público ni material que solo resulte incómodo de formalizar;
- cada entrada declara por qué no puede ser pública, qué registro canónico complementa y cuándo debe revisarse o eliminarse;
- muestra una advertencia si su revisión base quedó antigua;
- nunca se combina automáticamente hacia archivos compartidos;
- no contiene secretos: estos viven en el sistema destinado a secretos.

Continuar una investigación de otra persona requiere clonar/actualizar el expediente compartido y, si hace falta, inicializar un overlay vacío. No hace falta “traerla al privado”. Una vista local puede presentar `público + overlay`, claramente marcada, pero toda operación de dominio escribe primero contra el público mediante rama/PR.

### Worktrees portables

**Hecho.** `git worktree add <path> <branch>` crea un worktree para una rama existente y `git worktree list --porcelain` ofrece salida estable para automatización ([Git, `git worktree`](https://git-scm.com/docs/git-worktree.html)). Git también admite rutas relativas internamente, pero esa configuración no convierte una ruta de una máquina en coordenada de dominio portable.

**Recomendación.** Sustituir el path absoluto persistido por:

- remote normalizado del repositorio;
- branch exacta;
- commit observado (`HEAD`) cuando el registro se use como evidencia reproducible;
- identidad/revisión del handoff existente.

En cada máquina se derivan `repository-dir` desde el remote y `branch-dir` desde la rama, y se resuelve `configured_worktree_root / repository-dir / branch-dir`. No conviene persistir también esos valores derivados mientras la regla sea determinista: duplicarlos permitiría divergencia. Antes de crear o seleccionar el worktree, el helper debe normalizar el resultado, comprobar que permanece dentro del root configurado, rechazar colisiones y consultar el registro real de Git. No se debe concatenar una branch cruda que contenga `/`, `..` u otros segmentos de path. La branch localiza una línea móvil; no reemplaza al commit cuando se necesita reproducir exactamente una observación histórica.

### Flujo de contribución

1. Actualizar la rama canónica y abrir una rama de investigación.
2. Registrar la revisión/fingerprint pública observada.
3. Agregar o cambiar la unidad material más pequeña; regenerar la vista actual cuando cambie conocimiento vigente.
4. Calcular `base → canonical-current` y `base → contribution` por unidades semánticas.
5. Bloquear todo solapamiento o dependencia material hasta registrar una reconciliación explícita.
6. Ejecutar validación estructural, semántica, de referencias, portabilidad y seguridad.
7. Abrir PR con contribución, fuentes, registros afectados, impacto en snapshot y declaración de saneamiento.
8. Exigir revisión del steward del caso; para decisiones o material sensible, exigir además el owner correspondiente.
9. Si cambió la base, actualizar la rama, reevaluar semántica y descartar aprobaciones antiguas.
10. Integrar solo el resultado reconciliado; los cambios posteriores parten de la nueva revisión canónica.

No hace falta un lock pesimista global. Si la contención real demuestra que dos autores cambian repetidamente el mismo snapshot, se puede introducir un steward temporal por caso o serializar únicamente las transiciones de lifecycle y decisiones exclusivas.

### Migración de investigaciones locales

La migración debería llegar después del nuevo contrato, lector/escritor y evals:

1. **Inventario read-only:** enumerar casos, esquema, referencias, adjuntos, rutas locales, secretos potenciales y errores actuales.
2. **Clasificación:** por cada elemento decidir `shared`, `formalized`, `private-reference`, `drop` o `blocked-for-review`.
3. **Dry run:** generar en staging el expediente público candidato, overlay y reporte de transformación; no tocar el caso fuente.
4. **Compatibilidad temporal:** el runtime lee formato anterior y nuevo; los conversores de esquema pasan pruebas de ida y vuelta sin pérdida antes de habilitar el nuevo writer.
5. **Preservación:** conservar IDs existentes y enlaces; asignar IDs sin coordinación solo a nuevos registros producidos por el split. No reinterpretar silenciosamente decisiones históricas.
6. **Portabilidad:** convertir worktrees absolutos a remote + branch; derivar la ruta relativa, verificar contra Git y conservar `HEAD` observado cuando exista.
7. **Validación:** esquema, invariantes, referencias, snapshot/registros, hashes de artefactos, escaneo de secretos y prueba de reconstrucción del worktree.
8. **Revisión humana:** aprobar explícitamente qué se compartirá. Un hallazgo dudoso queda bloqueado, no se publica por defecto.
9. **Publicación gradual:** un PR pequeño por caso o grupo coherente; separar en commits la transformación estructural de la edición o formalización semántica.
10. **Cutover:** después de aceptar el PR, el expediente compartido se vuelve la única autoridad. El legado local queda read-only hasta comprobar el flujo completo y luego se retira conforme a una política explícita.

La migración debe poder reanudarse sin duplicar registros y comprobar que el input no cambió mediante hashes. El rollback seguro existe antes del primer merge; después de compartir, se corrige con nuevos commits y, si hubo exposición sensible, se aplica el procedimiento de incidente correspondiente. No se debe prometer que borrar un commit revierte una divulgación.

## Decisiones que aún conviene validar con prototipos

- si UUIDv7 completo es aceptable para humanos o se necesita una etiqueta visual corta no autoritativa;
- cuántos tipos de registro merecen archivo propio en la primera versión;
- qué contradicciones pueden detectarse mecánicamente sin introducir heurística opaca;
- qué porción de la vista actual puede generarse desde registros sin perder la narrativa necesaria; la vista no debería admitir escritura manual que contradiga sus fuentes;
- dónde alojar físicamente el overlay privado y qué controles de permisos/cifrado exige el entorno real;
- si el repositorio será privado al equipo o verdaderamente público en Internet, porque el umbral de minimización y revisión no es el mismo.

## Fuente principal de la recomendación

La recomendación combina el merge textual y el historial de Git, la revisión protegida de GitHub, el patrón de concurrencia optimista de RFC 9110, el modelo de provenance de W3C PROV, los registros de decisión modulares de Nygard, prácticas originales de investigación reproducible y los principios de minimización y prevención de exposición. La estructura concreta propuesta para `documentation-vault` es una inferencia de diseño; ninguna de esas fuentes prescribe por sí sola este layout.
