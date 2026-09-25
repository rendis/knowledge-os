package cases

import "strings"

// Section names per note locale. Guidance lives in HTML comments so an untouched template holds no
// records and passes check; check ignores comments.
var sectionNames = map[string]map[string]string{
	"es": {"objective": "Objetivo y alcance", "state": "Estado actual", "evidence": "Evidencia", "findings": "Conclusiones",
		"requirements": "Requisitos", "changes": "Cambios por componente", "acceptance": "Criterios de aceptación",
		"decisions": "Decisiones", "questions": "Preguntas abiertas", "handoffs": "Handoffs", "absorption": "Absorción"},
	"en": {"objective": "Objective and scope", "state": "Current state", "evidence": "Evidence", "findings": "Conclusions",
		"requirements": "Requirements", "changes": "Changes by component", "acceptance": "Acceptance criteria",
		"decisions": "Decisions", "questions": "Open questions", "handoffs": "Handoffs", "absorption": "Absorption"},
}

// Sections each case type must keep, in template order.
var typeSections = map[string][]string{
	"understanding": {"objective", "state", "evidence", "findings", "decisions", "questions", "absorption"},
	"development":   {"objective", "state", "requirements", "evidence", "changes", "acceptance", "decisions", "questions", "handoffs", "absorption"},
}

var requiredSections = map[string][]string{
	"understanding": {"objective", "state", "evidence", "findings", "questions"},
	"development":   {"objective", "state", "requirements", "changes", "acceptance", "questions"},
}

var guidance = map[string]map[string]string{
	"es": {
		"objective":    "Qué se quiere entender o construir, qué entra y qué queda fuera.",
		"state":        "Resumen vivo para retomar en frío: qué se sabe, qué falta y el siguiente paso. Reescríbelo en cada actualización; el historial es Git.",
		"evidence":     "Solo evidencia nueva: lo que ya está en el vault se referencia con [[nota]], no se copia. Un registro por hecho con su fuente y nivel, p. ej.: - **E-001** — hecho. Fuente: permalink o archivo@commit, snapshot, consulta o registro. Nivel: demostrado | observado con límites.",
		"findings":     "Conclusiones con su nivel, p. ej.: - **F-001** — conclusión (demostrada por E-001 | inferida desde E-001 y E-002 | sin resolver: falta …).",
		"requirements": "Qué pide el desarrollo y de dónde sale, p. ej.: - **R-001** — requisito. Origen: pedido, ticket o decisión D-001.",
		"changes":      "Qué cambia y dónde, por componente, p. ej.: - **CH-001** — [[repositorio]]: qué cambia (archivo o módulo) y por qué (R-001).",
		"acceptance":   "Criterios verificables, p. ej.: - **AC-001** — resultado observable que prueba R-001.",
		"decisions":    "- **D-001** — decisión, quién la tomó y por qué.",
		"questions":    "- **Q-001** — pregunta; cómo se resuelve (fuente, acceso o persona).",
		"handoffs":     "Un handoff por repositorio, p. ej.: - **DH-001** — [[repositorio]], rama issue/…, cambios CH-001, criterios AC-001.",
		"absorption":   "Conocimiento que pasa al vault al publicar: | Afirmación | Destino [[nota]] | Estado |",
	},
	"en": {
		"objective":    "What is to be understood or built, what is in and out of scope.",
		"state":        "Living summary to resume cold: what is known, what is missing and the next step. Rewrite it on each update; history is Git.",
		"evidence":     "New evidence only: what the vault already holds is referenced with [[note]], never copied. One record per fact with its source and level, e.g.: - **E-001** — fact. Source: permalink or file@commit, snapshot, query or record. Level: demonstrated | observed within limits.",
		"findings":     "Conclusions with their level, e.g.: - **F-001** — conclusion (demonstrated by E-001 | inferred from E-001 and E-002 | unresolved: missing …).",
		"requirements": "What the development asks and where it comes from, e.g.: - **R-001** — requirement. Origin: request, ticket or decision D-001.",
		"changes":      "What changes where, per component, e.g.: - **CH-001** — [[repository]]: what changes (file or module) and why (R-001).",
		"acceptance":   "Verifiable criteria, e.g.: - **AC-001** — observable result that proves R-001.",
		"decisions":    "- **D-001** — decision, who made it and why.",
		"questions":    "- **Q-001** — question; how it gets resolved (source, access or person).",
		"handoffs":     "One handoff per repository, e.g.: - **DH-001** — [[repository]], branch issue/…, changes CH-001, criteria AC-001.",
		"absorption":   "Knowledge that goes into the vault on publication: | Claim | Destination [[note]] | Status |",
	},
}

func template(locale, kind, id, title, sourceRef, created string) string {
	var b strings.Builder
	b.WriteString("---\nid: " + id + "\ntitle: \"" + strings.ReplaceAll(title, `"`, `'`) + "\"\ntype: " + kind + "\nstatus: open\ncreated: " + created + "\n")
	if sourceRef != "" {
		b.WriteString("source-ref: \"" + strings.ReplaceAll(sourceRef, `"`, `'`) + "\"\n")
	}
	b.WriteString("---\n\n# " + title + "\n")
	for _, s := range typeSections[kind] {
		b.WriteString("\n## " + sectionNames[locale][s] + "\n\n<!-- " + guidance[locale][s] + " -->\n")
	}
	return b.String()
}
