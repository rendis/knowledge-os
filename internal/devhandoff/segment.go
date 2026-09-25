package devhandoff

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The managed segment is the same in every repository and changes only with the framework, so it can
// be committed with the repository like any other shared instruction. Task content never goes here.
const segmentStart = `<!-- knowledge-os:managed:start id="development handoff" -->`
const segmentEnd = `<!-- knowledge-os:managed:end id="development handoff" -->`

const segmentBody = `## Development handoff

A cell investigation may prepare this repository for a task. The task lives in ` + "`.handoff/`" + ` (local to this worktree, excluded from Git). This section is identical in every repository.

When ` + "`.handoff/`" + ` exists, before planning or changing code:

1. Read each ` + "`.handoff/DH-*.md`" + `: the task, the changes it asks for (what and where), the acceptance criteria, the context it needs and what is out of scope. It is the whole definition; you need neither the investigation nor the vault.
2. Implement only those changes, following this repository's own instructions, which prevail (closer instruction files govern their subtree). Verify every acceptance criterion with the repository's usual tests or checks.
3. Leave the task files unchanged. Record in ` + "`.handoff/deltas.md`" + `, as soon as it becomes known and before later work relies on it, anything that changes, adds to or contradicts the definition, plus material decisions, deviations, open questions and verification results (not routine progress). Number entries consecutively; supersede an earlier entry with a new one instead of editing it:

   ` + "```" + `
   ## DELTA-NNN — <short title>
   - Handoff: DH-NNN
   - Type: definition | decision | deviation | question | verification
   - Detail: <what differs from the task, or what was decided or verified, and why>
   - Evidence: <commit, test, pull request, file or dated agreement>
   ` + "```" + `

4. Progress is the branch itself: commits, tests and the pull request, following this repository's workflow; commits, pushes and pull requests need the user's authorization.
5. When the task is ambiguous or a criterion cannot be met, ask the user and record the question as a delta.

The cell later reads this branch and ` + "`deltas.md`" + ` to update its investigation; nothing here writes to the vault.`

// Segment returns the managed segment with its markers.
func Segment() string { return segmentStart + "\n" + segmentBody + "\n" + segmentEnd + "\n" }

var segmentRE = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(segmentStart) + `.*?` + regexp.QuoteMeta(segmentEnd) + `\n?`)

// segmentStatus is absent, current or outdated for one instruction file.
func segmentStatus(path string) string {
	b, e := os.ReadFile(path)
	if e != nil {
		return "absent"
	}
	m := segmentRE.Find(b)
	switch {
	case m == nil:
		return "absent"
	case strings.TrimSpace(string(m)) == strings.TrimSpace(Segment()):
		return "current"
	default:
		return "outdated"
	}
}

// instructionFiles are the root files the segment must reach: AGENTS.md (Codex, Cursor and most
// harnesses), plus the file Claude Code reads instead when one exists without importing AGENTS.md:
// Claude Code loads AGENTS.md only when no CLAUDE.md or CLAUDE.local.md is present.
func instructionFiles(worktree string) []string {
	files := []string{filepath.Join(worktree, "AGENTS.md")}
	for _, name := range []string{"CLAUDE.md", "CLAUDE.local.md"} {
		p := filepath.Join(worktree, name)
		fi, e := os.Lstat(p)
		if e != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			if b, e := os.ReadFile(p); e == nil && !strings.Contains(string(b), "AGENTS.md") {
				files = append(files, p)
			}
		}
		break // the first Claude file present is the one Claude reads alongside
	}
	return files
}

// withSegment returns the file content with the current segment in place: replaced where it exists,
// appended otherwise. It reports whether anything changed.
func withSegment(path string) ([]byte, bool, error) {
	b, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return nil, false, e
	}
	cur := string(b)
	var next string
	if segmentRE.MatchString(cur) {
		next = segmentRE.ReplaceAllLiteralString(cur, Segment())
	} else if strings.TrimSpace(cur) == "" {
		next = Segment()
	} else {
		next = strings.TrimRight(cur, "\n") + "\n\n" + Segment()
	}
	return []byte(next), next != cur, nil
}
