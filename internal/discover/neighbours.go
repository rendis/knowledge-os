package discover

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"knowledge-os/internal/config"
)

var aliasList = regexp.MustCompile(`\[(.*)\]`)

// noteRepository names the tracked repository a repository note documents, by its aliases or basename.
func noteRepository(vault, full string, fm map[string]string) string {
	names := []string{strings.TrimSuffix(filepath.Base(full), ".md")}
	if m := aliasList.FindStringSubmatch(fm["aliases"]); m != nil {
		for _, a := range strings.Split(m[1], ",") {
			if a = strings.Trim(strings.TrimSpace(a), `"'`); a != "" {
				names = append([]string{a}, names...)
			}
		}
	}
	for _, n := range names {
		if in, e := discoverRepositories(vault, map[string]bool{n: true}); e == nil && len(in) == 1 {
			return n
		}
	}
	return ""
}

var (
	learningHeading       = regexp.MustCompile(`^(#{1,3})[ 	]+(.*)$`)
	learningEvidenceEntry = regexp.MustCompile(`^EV-\d{3}\b`)
	durableSourceLine     = regexp.MustCompile(`^-[ 	]+Fuentes durables:[ 	]+\S`)
)

// Each durable source in a learning evidence entry records what was inspected at the time.
// Links in the rest of the note may support current claims and keep the normal freshness check.
func learningEvidenceSourceLines(rel string, note []byte) map[int]bool {
	if !strings.HasPrefix(rel, "70-Aprendizajes/") {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(note), "\r\n", "\n"), "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	end := slices.Index(lines[1:], "---") + 1
	var metadata struct {
		Type string `yaml:"tipo"`
	}
	if end == 0 || yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata) != nil || metadata.Type != "aprendizaje" {
		return nil
	}
	sources := map[int]bool{}
	inEvidence, inEntry := false, false
	for i := end + 1; i < len(lines); i++ {
		line := lines[i]
		if heading := learningHeading.FindStringSubmatch(line); heading != nil {
			if heading[1] == "###" {
				inEntry = inEvidence && learningEvidenceEntry.MatchString(heading[2])
			} else {
				inEvidence = heading[1] == "##" && strings.TrimSpace(heading[2]) == "Evidencia acumulada"
				inEntry = false
			}
		} else if inEntry && durableSourceLine.MatchString(line) {
			sources[i] = true
		}
	}
	return sources
}

func checkHistoricalAnchor(repo string, a anchor) error {
	commit, err := resolveCommit(repo, a.Commit)
	if err != nil || !strings.HasPrefix(commit, a.Commit) {
		return fmt.Errorf("commit %s is not available", a.Commit)
	}
	path, err := url.PathUnescape(a.Path)
	if err != nil {
		return fmt.Errorf("invalid path in historical source %s", a.Path)
	}
	content, err := gitOutputRaw(repo, "cat-file", "blob", commit+":"+path)
	if err != nil {
		return fmt.Errorf("cannot read %s at commit %s", path, a.Commit)
	}
	lines := 0
	if content != "" {
		lines = strings.Count(strings.TrimSuffix(content, "\n"), "\n") + 1
	}
	if a.hasRange && (a.From < 1 || a.From > a.To || a.To > lines) {
		return fmt.Errorf("lines L%d-L%d of %s do not exist at commit %s", a.From, a.To, path, a.Commit)
	}
	return nil
}

// StaleNeighbours lists knowledge notes that cite a synced repository at an older commit on files
// that changed up to the repository note's new `commit-analizado`. A sync that updates the repository
// note must update or re-anchor those neighbours too, or their claims silently describe old code.
// A historical learning source must resolve at its original revision.
func StaleNeighbours(vault string, repoNotes []string) ([]string, error) {
	out := []string{}
	for _, rel := range repoNotes {
		full := filepath.Join(vault, rel)
		b, e := os.ReadFile(full)
		if e != nil {
			continue
		}
		fm := frontmatterOf(b)
		commit := strings.Trim(fm["commit-analizado"], `"'`)
		repo := noteRepository(vault, full, fm)
		if repo == "" || commit == "" {
			continue
		}
		inputs, e := discoverRepositories(vault, map[string]bool{repo: true})
		if e != nil || len(inputs) != 1 {
			continue
		}
		path := inputs[0].Path
		sourceID, _ := config.RemoteIdentity(inputs[0].Remote)
		head, e := resolveCommit(path, commit)
		if e != nil {
			return nil, fmt.Errorf("%s: commit-analizado %s is not in %s", rel, commit, repo)
		}
		changedSince := map[string][][2]int{}
		e = filepath.WalkDir(vault, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			r, _ := filepath.Rel(vault, p)
			r = filepath.ToSlash(r)
			if d.IsDir() {
				if r != "." && (strings.HasPrefix(d.Name(), ".") || !regexp.MustCompile(`^[1-7]\d-`).MatchString(strings.SplitN(r, "/", 2)[0])) {
					return filepath.SkipDir
				}
				return nil
			}
			if r == rel || !strings.HasSuffix(r, ".md") {
				return nil
			}
			nb, _ := os.ReadFile(p)
			stale := map[string]bool{}
			historicalLines := learningEvidenceSourceLines(r, nb)
			for _, a := range parseAnchors(string(nb)) {
				if !strings.EqualFold(strings.SplitN(a.Repo, "/", 2)[1], repo) {
					continue
				}
				if historicalLines[a.line] && sourceID == "github.com/"+strings.ToLower(a.Repo) {
					if err := checkHistoricalAnchor(path, a); err != nil {
						return fmt.Errorf("%s: invalid historical source for %s: %w", r, a.Repo, err)
					}
					continue
				}
				if strings.HasPrefix(head, a.Commit) {
					continue
				}
				old, e := resolveCommit(path, a.Commit)
				if e != nil {
					continue // G1 of that note reports unresolvable anchors
				}
				if _, e := gitOutput(path, "merge-base", "--is-ancestor", old, head); e != nil {
					continue // cites a commit that is not older than the sync (e.g. a newer pinned version)
				}
				key := old + ":" + a.Path
				hunks, ok := changedSince[key]
				if !ok {
					hunks = changedLines(path, old, head, a.Path)
					changedSince[key] = hunks
				}
				if overlaps(hunks, a.From, a.To) {
					stale[a.Path+"@"+a.Commit[:minInt(12, len(a.Commit))]] = true
				}
			}
			if len(stale) > 0 {
				keys := make([]string, 0, len(stale))
				for k := range stale {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				out = append(out, fmt.Sprintf("%s cites %s files changed up to %s: %s", r, repo, head[:12], strings.Join(firstList(keys, 4), ", ")))
			}
			return nil
		})
		if e != nil {
			return nil, e
		}
	}
	sort.Strings(out)
	return out, nil
}

var hunkHeader = regexp.MustCompile(`(?m)^@@ -(\d+)(?:,(\d+))? `)

// changedLines returns the old-side line ranges of file that changed between two commits; a nil result
// means the file did not change, and a single {0,0} range means it changed as a whole (deleted or binary).
func changedLines(repo, from, to, file string) [][2]int {
	d, e := gitOutput(repo, "diff", "-U0", "--no-renames", from, to, "--", file)
	if e != nil || d == "" {
		return nil
	}
	out := [][2]int{}
	for _, m := range hunkHeader.FindAllStringSubmatch(d, -1) {
		start, _ := strconv.Atoi(m[1])
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		if n == 0 { // pure insertion after old line `start`: recorded as an empty range {start+1, start}
			out = append(out, [2]int{start + 1, start})
			continue
		}
		out = append(out, [2]int{start, start + n - 1})
	}
	if len(out) == 0 {
		return [][2]int{{0, 0}}
	}
	return out
}

// overlaps reports whether a cited range (whole file when from is 0) intersects a changed range.
func overlaps(hunks [][2]int, from, to int) bool {
	for _, h := range hunks {
		switch {
		case from == 0 || h == [2]int{0, 0}:
			return true
		case h[1] < h[0]: // insertion after line h[1]: changes the cited block only when it falls inside it
			if from <= h[1] && h[1] < to {
				return true
			}
		case h[0] <= to && from <= h[1]:
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CheckNoteIntroduced gates a repository note. With base content (a neighbour touched without a new
// commit-analizado) only errors absent from the base version fail; pre-existing debt is reported but does
// not block re-anchoring. Without base content every error fails. Unresolved source anchors or coverage
// always block publication: unavailable validation is not pre-existing factual debt.
func CheckNoteIntroduced(vault, note string, base []byte) (bool, []string, int, error) {
	r, e := checkNoteMode(vault, note, "", false)
	if e != nil {
		return false, nil, 0, e
	}
	unresolved := []string{}
	for _, issue := range r.Issues {
		if issue.Severity == "pending" && (issue.Gate == "G1-anchor" || issue.Gate == "G2-coverage") {
			unresolved = append(unresolved, issue.Gate+"|"+issue.Where+"|"+issue.Detail)
		}
	}
	if len(unresolved) != 0 {
		sort.Strings(unresolved)
		return false, unresolved, 0, nil
	}
	if r.OK {
		return r.OK, nil, 0, e
	}
	keys := func(c noteCheck) map[string]bool {
		out := map[string]bool{}
		for _, i := range c.Issues {
			switch {
			case i.Severity != "error":
			case strings.HasPrefix(i.Gate, "G4-") || i.Gate == "G1-format": // counts in the detail change as debt is paid; the subject does not
				out[i.Gate+"|"+i.Where] = true
			default:
				out[i.Gate+"|"+i.Where+"|"+i.Detail] = true
			}
		}
		return out
	}
	now := keys(r)
	if base == nil {
		return false, sortedKeys(now), 0, nil
	}
	dir := filepath.Join(vault, ".agents", "state", "discovery", "base-notes")
	if e := os.MkdirAll(dir, 0o755); e != nil {
		return false, nil, 0, e
	}
	tmp := filepath.Join(dir, filepath.Base(note))
	if e := os.WriteFile(tmp, base, 0o644); e != nil {
		return false, nil, 0, e
	}
	defer os.Remove(tmp)
	rb, e := checkNoteMode(vault, tmp, "", false)
	if e != nil {
		return false, nil, 0, e
	}
	before := keys(rb)
	introduced := []string{}
	for k := range now {
		if !before[k] {
			introduced = append(introduced, k)
		}
	}
	sort.Strings(introduced)
	return len(introduced) == 0, introduced, len(now) - len(introduced), nil
}
