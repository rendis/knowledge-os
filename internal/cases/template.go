package cases

import "strings"

// Section names per note locale. Guidance lives in HTML comments so an untouched template holds no
// records and passes check; check ignores comments.
var sectionNames = map[string]map[string]string{
	"es": {"objective": "Objetivo y alcance", "state": "Estado actual", "evidence": "Evidencia", "findings": "Conclusiones",
		"requirements": "Requisitos", "decisions": "Decisiones", "questions": "Preguntas abiertas", "handoffs": "Handoffs", "log": "Bitácora"},
	"en": {"objective": "Objective and scope", "state": "Current state", "evidence": "Evidence", "findings": "Conclusions",
		"requirements": "Requirements", "decisions": "Decisions", "questions": "Open questions", "handoffs": "Handoffs", "log": "Log"},
}

// Sections each case type must keep, in template order.
var typeSections = map[string][]string{
	"understanding": {"objective", "state", "evidence", "findings", "decisions", "questions", "log"},
	"development":   {"objective", "state", "requirements", "evidence", "findings", "decisions", "questions", "handoffs", "log"},
}

var requiredSections = map[string][]string{
	"understanding": {"objective", "state", "evidence", "findings", "questions"},
	"development":   {"objective", "state", "requirements", "questions"},
}

var guidance = map[string]map[string]string{
	"es": {
		"objective":    "La solicitud formalizada: qué se necesita y para qué, el resultado esperado, qué entra y qué queda fuera. En lenguaje neutro y práctico, sin transcribir al solicitante.",
		"state":        "Resumen vivo para retomar en frío: qué se sabe, qué falta y el siguiente paso, citando registros. Se reescribe con `investigation state`.",
		"evidence":     "Solo evidencia nueva, un hecho por registro con su fuente y nivel (`investigation add --kind evidence`); lo que el vault ya documenta se referencia con [[nota]].",
		"findings":     "Conclusiones con su nivel: demostrada o inferida desde registros, o sin resolver con lo que falta. Lo que debe pasar al vault se marca con --for-vault.",
		"requirements": "Qué pide el desarrollo y de dónde sale (pedido, ticket o decisión).",
		"decisions":    "Decisiones, quién las tomó y por qué.",
		"questions":    "Lo que falta saber o buscar, y cómo se resuelve (fuente, acceso o persona).",
		"handoffs":     "Paquetes de tarea (handoffs/DH-NNN.md): cada uno define qué cambia, dónde y cómo se acepta, citando los requisitos.",
		"log":          "La escribe la CLI: una línea por cambio del caso (fecha, acción, registros).",
	},
	"en": {
		"objective":    "The formalized request: what is needed and why, the expected result, what is in and out of scope. Neutral, practical language; never a transcript of the requester.",
		"state":        "Living summary to resume cold: what is known, what is missing and the next step, citing records. Rewritten with `investigation state`.",
		"evidence":     "New evidence only, one fact per record with its source and level (`investigation add --kind evidence`); what the vault already documents is referenced with [[note]].",
		"findings":     "Conclusions with their level: demonstrated or inferred from records, or unresolved with what is missing. What belongs in the vault is marked with --for-vault.",
		"requirements": "What the development asks and where it comes from (request, ticket or decision).",
		"decisions":    "Decisions, who made them and why.",
		"questions":    "What is still to be known or searched, and how it gets resolved (source, access or person).",
		"handoffs":     "Task packages (handoffs/DH-NNN.md): each defines what changes, where and how it is accepted, citing the requirements.",
		"log":          "Written by the CLI: one line per change to the case (date, action, records).",
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
