package audit

var FORBIDDEN_RELATION_FILES = []string{"relations.csv", "relations.db", "relations.jsonl", "relations.sqlite"}

var REPO_REQUIRED = []string{"aliases", "cobertura-datos", "cobertura-entradas", "cobertura-flujos", "cobertura-infra", "cobertura-salidas", "commit-analizado", "consume-de", "escribe-en", "fecha-analisis", "gatillado-por", "lee-de", "lenguaje", "participa-en", "publica-en", "rama-analizada", "sistema", "tags", "tipo", "ultima-auditoria", "usa-infra"}

var REPO_LIST_FIELDS = []string{"aliases", "consume-de", "escribe-en", "gatillado-por", "lee-de", "participa-en", "publica-en", "tags", "usa-infra"}

var REPO_SECTIONS = []string{"Entradas y salidas", "Gatillo", "Infraestructura y scheduling", "Limitaciones y desconocimientos", "Persistencia y datos", "Propósito", "Qué hace", "Relaciones"}

var REPO_TYPES = []string{"adapter", "api", "bff", "frontend", "function", "http-adapter", "infraestructura", "job", "libreria", "publicador", "scaffold", "suscriptor"}

var COVERAGE_VALUES = []string{"completo", "no-aplica", "parcial", "por-confirmar"}

var COVERAGE_FIELDS = []string{"cobertura-datos", "cobertura-entradas", "cobertura-flujos", "cobertura-infra", "cobertura-salidas"}

var ARCH_COMMON = []string{"participa-en", "sistema", "tags", "tipo", "ultima-auditoria"}

var ARCH_SERVICE = []string{"compuesto-por"}

var ARCH_COMPONENT = []string{"implementado-por"}

var ARCH_RUNTIME_REQUIRED = []string{"ambiente", "nombre-raw", "plataforma", "proyecto", "ultima-verificacion"}

var ARCH_RUNTIME_OPTIONAL = []string{"ejecuta", "gatilla-a", "region", "usa-infra"}

var ARCH_ALLOWED = []string{"ambiente", "compuesto-por", "ejecuta", "gatilla-a", "implementado-por", "nombre-raw", "participa-en", "plataforma", "proyecto", "region", "sistema", "tags", "tipo", "ultima-auditoria", "ultima-verificacion", "usa-infra"}

var ARCH_SECTIONS = map[string][]string{"servicio": {"Interfaces y datos", "Limitaciones y desconocimientos", "Qué representa", "Relaciones", "Responsabilidad y límites", "Unidades que lo componen"}, "componente": {"Implementación", "Interfaces", "Limitaciones y desconocimientos", "Propósito", "Relaciones", "Runtime e infraestructura"}, "recurso-runtime": {"Ejecución o disparo", "Evidencia", "Limitaciones y desconocimientos", "Proyecto y ambiente", "Qué representa", "Relaciones"}}

var TOPIC_FIELDS = []string{"nombre-raw", "sistema", "tags", "tipo"}

var EVENT_FIELDS = []string{"nombre-raw", "sistema", "tags", "tipo", "topico"}

var TOPIC_SECTIONS = []string{"Contrato", "Infraestructura verificada", "Limitaciones y desconocimientos", "Qué representa"}

var TOPIC_FORBIDDEN_HEADINGS = []string{"Consumidores", "Productores", "Productores y consumidores"}

var INTEGRATION_FIELDS = []string{"aliases", "tags", "tipo"}

var INTEGRATION_SECTIONS = []string{"Contratos relevantes", "Cómo se usa", "Infraestructura o ownership", "Limitaciones y desconocimientos", "Qué es"}

var FLOW_FIELDS = []string{"sistema", "tags", "tipo"}

var FLOW_SECTIONS = []string{"Diagrama de componentes", "Diagrama de flujo", "Disparador", "Participantes", "Paso a paso", "Pendientes", "Qué resuelve"}

var SYSTEM_FIELDS = []string{"aliases", "tags", "tipo"}

var INDEX_FIELDS = []string{"tags", "tipo"}

var GLOSSARY_REQUIRED = []string{"tags", "tipo"}

var GLOSSARY_ALLOWED = []string{"aliases", "tags", "tipo"}

var OPERATIONAL_COMMON_REQUIRED = []string{"area", "canales", "clase", "estado", "owner", "tags", "tipo", "ultima-verificacion"}

var OPERATIONAL_OPTIONAL = []string{"relacionado-con"}

var OPERATIONAL_ALLOWED = []string{"area", "canales", "clase", "estado", "owner", "relacionado-con", "report-id", "tags", "tipo", "ultima-verificacion"}

var OPERATIONAL_STATES = []string{"borrador", "retirado", "vigente"}

var OPERATIONAL_CLASSES = []string{"catalogo", "estandar", "guia", "politica", "procedimiento", "reporte"}

var OPERATIONAL_SECTIONS = map[string][]string{"politica": {"Alcance", "Escalamiento", "Excepciones", "Mantenimiento", "Objetivo", "Reglas"}, "guia": {"Objetivo", "Prerrequisitos", "Problemas y escalamiento", "Procedimiento", "Validación"}, "catalogo": {"Catálogo", "Criterios de uso", "Limitaciones", "Mantenimiento", "Propósito"}, "estandar": {"Alcance", "Excepciones", "Objetivo", "Plantillas", "Reglas", "Validación"}, "procedimiento": {"Decisiones", "Disparador", "Entradas requeridas", "Evidencia de finalización", "Fallos y recuperación", "Limitaciones", "Objetivo", "Pasos"}, "reporte": {"Audiencia y decisiones", "Definiciones y grano", "Fuente y alcance", "Limitaciones", "Mantenimiento", "Período, filtros y exclusiones", "Propósito", "Salida", "Validación"}}

const OPERATIONAL_STEP_HEADER = "| ID | Acción | Capacidad | Efecto externo | Entrada | Salida | Continuar si |"

var LEARNING_FIELDS = []string{"aplica-a", "dimensiones", "estado", "fecha-conclusion", "investigaciones-origen", "resultado", "supersede-a", "tags", "tipo", "ultima-validacion"}

var LEARNING_STATES = []string{"cuestionado", "superado", "vigente"}

var LEARNING_RESULTS = []string{"alternativa-descartada", "baseline-conservado", "cambio-adoptado", "hallazgo-metodologico"}

var LEARNING_SECTION_ORDER = []string{"Resumen", "Pregunta y contexto", "Baseline y alternativas", "Método", "Evidencia acumulada", "Decisión y justificación", "Enseñanza reutilizable", "Cuándo aplica", "Cuándo no aplica", "Implementación y despliegue", "Limitaciones y revalidación", "Trazabilidad"}

var LEARNING_SECTIONS = []string{"Baseline y alternativas", "Cuándo aplica", "Cuándo no aplica", "Decisión y justificación", "Enseñanza reutilizable", "Evidencia acumulada", "Implementación y despliegue", "Limitaciones y revalidación", "Método", "Pregunta y contexto", "Resumen", "Trazabilidad"}

var LEARNING_LIST_FIELDS = []string{"aplica-a", "dimensiones", "investigaciones-origen", "supersede-a", "tags"}

var LEARNING_EVIDENCE_FIELDS = []string{"Investigación", "Fuentes durables", "Contexto y exclusiones", "Método y medidas", "Resultado", "Aporte a la conclusión"}
