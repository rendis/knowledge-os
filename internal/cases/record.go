package cases

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Record kinds: evidence, findings, questions and decisions in every case; requirements and handoffs in
// a development case. What changes where and how it is accepted lives in each handoff package.
type recordKind struct{ prefix, section string }

var kinds = map[string]recordKind{
	"evidence": {"E", "evidence"}, "finding": {"F", "findings"}, "question": {"Q", "questions"},
	"decision": {"D", "decisions"}, "requirement": {"R", "requirements"}, "handoff": {"DH", "handoffs"},
}

var labels = map[string]map[string]string{
	"es": {"source": "Fuente", "level": "Nivel", "demonstrated": "demostrado", "observed": "observado", "limits": "límites",
		"demonstrated-by": "demostrada por", "inferred-from": "inferida desde", "unresolved": "sin resolver; falta",
		"resolve-by": "Se resuelve con", "by": "Decidió", "origin": "Origen", "for-vault": "Para el vault",
		"repository": "Repositorio", "branch": "rama", "package": "paquete", "files": "Archivos",
		"superseded": "reemplazado por", "resolved": "resuelta por", "absorbed": "absorbida", "reconciled": "reconciliado hasta",
		"opened": "caso abierto", "added": "agregado", "resolves": "resuelve", "supersedes": "reemplaza a",
		"state": "estado actual actualizado", "closed": "cerrado", "reopened": "reabierto", "reconciliation": "reconciliación",
		"ask-evidence": "evidencia del desarrollo (commit, archivo@commit, test o pull request)", "development": "desarrollo", "requester-or-cell": "solicitante o célula"},
	"en": {"source": "Source", "level": "Level", "demonstrated": "demonstrated", "observed": "observed", "limits": "limits",
		"demonstrated-by": "demonstrated by", "inferred-from": "inferred from", "unresolved": "unresolved; missing",
		"resolve-by": "Resolved by", "by": "Decided by", "origin": "Origin", "for-vault": "For the vault",
		"repository": "Repository", "branch": "branch", "package": "package", "files": "Files",
		"superseded": "superseded by", "resolved": "resolved by", "absorbed": "absorbed", "reconciled": "reconciled through",
		"opened": "case opened", "added": "added", "resolves": "resolves", "supersedes": "supersedes",
		"state": "current state updated", "closed": "closed", "reopened": "reopened", "reconciliation": "reconciliation",
		"ask-evidence": "evidence from the development (commit, file@commit, test or pull request)", "development": "development", "requester-or-cell": "requester or cell"},
}

const maxArtifact = 20 << 20

var forVaultMark = regexp.MustCompile(`(?:Para el vault|For the vault): (\[\[[^\]]+\]\])`)

// file is a file attached to a record: read and screened before the write, copied after the gate.
type file struct {
	src  string
	data []byte
	rel  string
}

// readFiles loads the files given with --file and refuses credentials in text content.
func readFiles(paths []string) ([]file, error) {
	out := []file{}
	for _, p := range paths {
		st, e := os.Stat(p)
		if e != nil || st.IsDir() {
			return nil, fmt.Errorf("%s is not a readable file", p)
		}
		if st.Size() > maxArtifact {
			return nil, fmt.Errorf("%s is larger than 20 MB: keep an excerpt with its coverage", p)
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		if utf8.Valid(b) && !strings.ContainsRune(string(b[:min(len(b), 8192)]), 0) {
			if ms := credentials(string(b), 1); len(ms) > 0 {
				m := ms[0]
				return nil, errors.New(filepath.Base(p) + " contains a credential (" + firstRunes(m, 16) + "…): redact it and attach the redacted copy")
			}
		}
		out = append(out, file{src: p, data: b})
	}
	return out, nil
}

// buildRecord appends one record to the case text and returns the new text, its ID and log entries.
func buildRecord(o options, c Case, text, loc string, files []file) (string, string, []string, error) {
	L := labels[loc]
	k, ok := kinds[o.recKind]
	if !ok {
		return "", "", nil, errors.New("--kind must be evidence, finding, question, decision, requirement or handoff")
	}
	if strings.TrimSpace(o.text) == "" && o.recKind != "handoff" {
		return "", "", nil, errors.New("--text is required")
	}
	if c.Type != "development" && (o.recKind == "requirement" || o.recKind == "handoff") {
		return "", "", nil, errors.New(o.recKind + " records belong to development cases")
	}
	if len(files) > 0 && o.recKind != "evidence" && o.recKind != "finding" {
		return "", "", nil, errors.New("--file attaches to the evidence or finding that uses it")
	}
	if o.forVault != "" && o.recKind != "finding" {
		return "", "", nil, errors.New("--for-vault marks a finding")
	}
	id := nextID(text, k.prefix)
	body := sentence(o.text)
	switch o.recKind {
	case "evidence":
		if !sourceRef.MatchString(o.source) {
			return "", "", nil, errors.New("--source must cite a permalink or file@commit, platform snapshot, query, work item key, record, or the requester statement with its date")
		}
		level := L["demonstrated"]
		switch o.level {
		case "demonstrated":
		case "observed":
			if strings.TrimSpace(o.limits) == "" {
				return "", "", nil, errors.New("--limits is required for observed evidence (sample, environment, time window or truncation)")
			}
			level = L["observed"] + " (" + L["limits"] + ": " + clause(o.limits) + ")"
		default:
			return "", "", nil, errors.New("--level must be demonstrated or observed")
		}
		body += " " + L["source"] + ": " + clause(o.source) + ". " + L["level"] + ": " + level + "."
	case "finding":
		switch o.level {
		case "demonstrated", "inferred":
			ids, e := requireRefs(text, "--from", o.from, "E", "F")
			if e != nil {
				return "", "", nil, e
			}
			key := map[string]string{"demonstrated": "demonstrated-by", "inferred": "inferred-from"}[o.level]
			body += " " + L["level"] + ": " + L[key] + " " + strings.Join(ids, ", ") + "."
		case "unresolved":
			if strings.TrimSpace(o.missing) == "" {
				return "", "", nil, errors.New("--missing is required for an unresolved conclusion: the source or check that would settle it")
			}
			body += " " + L["level"] + ": " + L["unresolved"] + " " + clause(o.missing) + "."
		default:
			return "", "", nil, errors.New("--level must be demonstrated, inferred or unresolved")
		}
		if o.forVault != "" {
			if !wikilink.MatchString(o.forVault) {
				return "", "", nil, errors.New("--for-vault names the destination note as [[note]]")
			}
			body += " " + L["for-vault"] + ": " + strings.TrimSpace(o.forVault) + "."
		}
	case "question":
		if strings.TrimSpace(o.resolveBy) == "" {
			return "", "", nil, errors.New("--resolve-by is required: the source, access or person that answers it")
		}
		body += " " + L["resolve-by"] + ": " + clause(o.resolveBy) + "."
	case "decision":
		if strings.TrimSpace(o.by) == "" {
			return "", "", nil, errors.New("--by is required: the role that made the decision")
		}
		body += " " + L["by"] + ": " + clause(o.by) + "."
	case "requirement":
		if strings.TrimSpace(o.origin) == "" {
			return "", "", nil, errors.New("--origin is required: the request, ticket or decision it comes from")
		}
		body += " " + L["origin"] + ": " + clause(o.origin) + "."
	case "handoff":
		p, e := casePackage(o.vault, c, o.pkg)
		if e != nil {
			return "", "", nil, e
		}
		if definedRecords(text)[p.Handoff] {
			return "", "", nil, fmt.Errorf("%s is already recorded in the case", p.Handoff)
		}
		id = p.Handoff
		body = sentence(p.Title)
		if o.text != "" {
			body += " " + sentence(o.text)
		}
		body += " " + L["repository"] + ": " + p.Repository + "; " + L["branch"] + " `" + p.Branch + "`; " + L["package"] + " `handoffs/" + filepath.Base(p.Path) + "`."
	}
	if len(files) > 0 {
		quoted, names := []string{}, map[string]bool{}
		for i := range files {
			files[i].rel = "artifacts/" + id + "-" + unsafeName.ReplaceAllString(filepath.Base(files[i].src), "-")
			if names[files[i].rel] {
				return "", "", nil, fmt.Errorf("two attached files would both be stored as %s: rename one so each keeps its content", files[i].rel)
			}
			names[files[i].rel] = true
			if _, e := os.Stat(filepath.Join(o.vault, filepath.Dir(c.Path), files[i].rel)); e == nil {
				return "", "", nil, fmt.Errorf("%s already exists: an attached file is never overwritten", files[i].rel)
			}
			quoted = append(quoted, "`"+files[i].rel+"`")
		}
		body += " " + L["files"] + ": " + strings.Join(quoted, ", ") + "."
	}
	logParts := []string{id + " " + L["added"]}
	if o.supersedes != "" {
		ids, e := requireRefs(text, "--supersedes", o.supersedes, k.prefix)
		if e != nil {
			return "", "", nil, e
		}
		for _, old := range ids {
			text = annotate(text, old, L["superseded"]+" "+id)
		}
		logParts = append(logParts, L["supersedes"]+" "+strings.Join(ids, ", "))
	}
	if o.resolves != "" {
		if o.recKind != "evidence" && o.recKind != "finding" && o.recKind != "decision" {
			return "", "", nil, errors.New("--resolves applies to evidence, findings and decisions")
		}
		ids, e := requireRefs(text, "--resolves", o.resolves, "Q")
		if e != nil {
			return "", "", nil, e
		}
		for _, q := range ids {
			text = annotate(text, q, L["resolved"]+" "+id)
		}
		logParts = append(logParts, L["resolves"]+" "+strings.Join(ids, ", "))
	}
	return appendToSection(text, k.section, loc, "- **"+id+"** — "+body), id, logParts, nil
}

func copyFiles(c Case, vault string, files []file) error {
	dir := filepath.Dir(filepath.Join(vault, c.Path))
	for _, f := range files {
		dest := filepath.Join(dir, f.rel)
		if e := os.MkdirAll(filepath.Dir(dest), 0o755); e != nil {
			return e
		}
		if e := os.WriteFile(dest, f.data, 0o644); e != nil {
			return e
		}
	}
	return nil
}

func add(o options, out io.Writer) error {
	files, e := readFiles(o.files)
	if e != nil {
		return e
	}
	var target Case
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		target = c
		next, id, logParts, e := buildRecord(o, c, text, loc, files)
		if e != nil {
			return "", nil, nil, e
		}
		res := map[string]any{"record": id}
		if len(files) > 0 {
			rels := []string{}
			for _, f := range files {
				rels = append(rels, f.rel)
			}
			res["files"] = rels
		}
		return next, logParts, res, nil
	}, func(Case) error { return copyFiles(target, o.vault, files) })
}

// absorb marks a finding meant for the vault as absorbed, once the note that holds it is changed on the
// same sync branch.
func absorb(o options, out io.Writer) error {
	return mutate(o, out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		if c.Visibility != "published" {
			return "", nil, nil, errors.New("absorption happens when the case is published: move it to investigations/ on the sync branch first")
		}
		ids, e := requireRefs(text, "--finding", o.finding, "F")
		if e != nil {
			return "", nil, nil, e
		}
		lines := strings.Split(visible(text), "\n")
		for _, id := range ids {
			target := ""
			for _, l := range lines {
				if m := recordDef.FindStringSubmatch(l); m != nil && m[1] == id {
					if t := forVaultMark.FindStringSubmatch(l); t != nil {
						target = t[1]
					}
					if strings.Contains(l, " — "+labels[loc]["absorbed"]) {
						return "", nil, nil, fmt.Errorf("%s is already absorbed", id)
					}
				}
			}
			if target == "" {
				return "", nil, nil, fmt.Errorf("%s is not marked for the vault (--for-vault)", id)
			}
			text = annotate(text, id, labels[loc]["absorbed"]+" "+today(o))
		}
		return text, []string{labels[loc]["absorbed"] + ": " + strings.Join(ids, ", ")}, nil, nil
	}, nil)
}
