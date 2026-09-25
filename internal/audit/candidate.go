package audit

import (
	"strings"
)

// CandidateIssue is a location-only structural defect safe to return before a
// semantic review. The full vault audit remains the publication-wide gate.
type CandidateIssue struct {
	Code  string
	Field string
	Line  int
}

// CandidateStructure validates one staged note with the same per-note contract
// used by Audit, without running vault-wide topology checks.
func CandidateStructure(root, path string) ([]CandidateIssue, error) {
	n, e := read(root, path)
	if e != nil {
		return nil, e
	}
	parts := strings.Split(path, "/")
	group := ""
	if len(parts) == 0 {
		return nil, nil
	}
	switch parts[0] {
	case "20-Repos":
		if len(parts) == 3 {
			group = "REPO_NOTES"
		}
	case "25-Topics":
		if len(parts) == 2 {
			group = "TOPIC_NOTES"
		}
	case "30-Flujos":
		if len(parts) == 2 && path != "30-Flujos/Flujos.md" {
			group = "FLOW_NOTES"
		}
	case "40-Integraciones":
		if len(parts) == 2 {
			group = "INTEGRATION_NOTES"
		}
	case "50-Glosario":
		if len(parts) == 2 {
			group = "GLOSSARY_NOTES"
		}
	}
	if group == "" {
		return nil, nil
	}
	a := auditor{systems: []string{s(n.f["sistema"])}, contracts: map[string][]string{}, areas: map[string]string{}}
	a.check(n, group)
	lines := strings.Split(strings.ReplaceAll(n.raw, "\r\n", "\n"), "\n")
	lineFor := func(field string) int {
		prefix := field + ":"
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), prefix) {
				return i + 1
			}
		}
		return 1
	}
	out := []CandidateIssue{}
	for _, raw := range a.issues {
		message := strings.TrimPrefix(raw, path+": ")
		issue := CandidateIssue{Code: "note-structure-invalid", Line: 1}
		if strings.Contains(message, "commit-analizado") {
			issue.Code, issue.Field, issue.Line = "repo-commit-invalid", "commit-analizado", lineFor("commit-analizado")
		} else if strings.HasPrefix(message, "missing section ") {
			issue.Code, issue.Field = "missing-required-section", strings.TrimPrefix(message, "missing section ")
		} else if strings.HasPrefix(message, "missing frontmatter field ") {
			issue.Code = "missing-frontmatter-field"
			issue.Field = strings.TrimPrefix(message, "missing frontmatter field ")
		}
		out = append(out, issue)
	}
	return out, nil
}
