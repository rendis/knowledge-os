package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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

// StaleNeighbours lists knowledge notes that cite a synced repository at an older commit on files
// that changed up to the repository note's new `commit-analizado`. A sync that updates the repository
// note must update or re-anchor those neighbours too, or their claims silently describe old code.
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
			for _, a := range parseAnchors(string(nb)) {
				if !strings.EqualFold(strings.SplitN(a.Repo, "/", 2)[1], repo) || strings.HasPrefix(head, a.Commit) {
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
// not block re-anchoring. Without base content every error fails.
func CheckNoteIntroduced(vault, note string, base []byte) (bool, []string, int, error) {
	r, e := checkNote(vault, note, "")
	if e != nil || r.OK {
		return r.OK, nil, 0, e
	}
	keys := func(c noteCheck) map[string]bool {
		out := map[string]bool{}
		for _, i := range c.Issues {
			if i.Severity == "error" {
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
	rb, e := checkNote(vault, tmp, "")
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
