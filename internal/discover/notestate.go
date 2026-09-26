package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NoteState is what a reader needs to trust a repository note without running its full gate: the
// repository, its local checkout, whether cited files changed on the reference branch since
// `commit-analizado`, and how the last discovery run compared the note's relations with the code.
// It reads only the note, git and the stored comparison, so it stays fast enough to answer with.
type NoteState struct {
	Repo          string        `json:"repo"`
	Checkout      string        `json:"checkout,omitempty"`
	Commit        string        `json:"commit_analizado,omitempty"`
	Ref           string        `json:"ref,omitempty"`
	Head          string        `json:"head,omitempty"`
	CheckoutHead  string        `json:"checkout_head,omitempty"` // when the working tree is not at the reference
	ChangedFiles  int           `json:"changed_files"`
	StaleCited    []string      `json:"stale_cited_files"`
	Unknown       string        `json:"freshness_unknown,omitempty"`
	Supported     []string      `json:"supported,omitempty"`
	Discrepancies []discrepancy `json:"discrepancies,omitempty"`
	Undocumented  []string      `json:"undocumented,omitempty"`
}

// RepositoryNoteState returns the state of a repository note, or false when the note names no
// repository the vault tracks.
func RepositoryNoteState(vault, note string) (NoteState, bool) {
	full := filepath.Join(vault, note)
	b, e := os.ReadFile(full)
	if e != nil {
		return NoteState{}, false
	}
	fm := frontmatterOf(b)
	repo := noteRepository(vault, full, fm)
	if repo == "" {
		return NoteState{}, false
	}
	s := NoteState{Repo: repo, Commit: strings.Trim(fm["commit-analizado"], `"'`), StaleCited: []string{}}
	if cmp, e := loadComparison(vault); e == nil {
		for _, c := range cmp {
			if strings.EqualFold(c.Repo, repo) {
				s.Supported, s.Discrepancies, s.Undocumented = c.Supported, c.Discrepancies, c.Undocumented
			}
		}
	}
	inputs, e := discoverRepositories(vault, map[string]bool{repo: true})
	if e != nil || len(inputs) != 1 || inputs[0].Path == "" {
		s.Unknown = "no local checkout of " + repo + "; clone it through onboard-developer to judge freshness"
		return s, true
	}
	in := inputs[0]
	s.Checkout = in.Path
	if in.RefErr != "" {
		s.Unknown = in.RefErr
		return s, true
	}
	s.Ref = in.Ref
	head, e := resolveCommit(in.Path, in.Ref)
	if e != nil || head == "" {
		s.Unknown = "reference " + in.Ref + " does not resolve in " + in.Path
		return s, true
	}
	s.Head = head[:12]
	if h, e := resolveCommit(in.Path, "HEAD"); e == nil && h != "" && h != head {
		s.CheckoutHead = h[:12]
	}
	if s.Commit == "" {
		s.Unknown = "the note has no commit-analizado"
		return s, true
	}
	full2, e := resolveCommit(in.Path, s.Commit)
	if e != nil || full2 == "" {
		s.Unknown = "commit " + s.Commit + " is not in the local checkout; fetch it"
		return s, true
	}
	if full2 == head {
		return s, true
	}
	changed, _ := gitOutput(in.Path, "diff", "--name-only", full2, head)
	set := map[string]bool{}
	for _, c := range strings.Split(changed, "\n") {
		if c != "" {
			set[c] = true
		}
	}
	s.ChangedFiles = len(set)
	cited := map[string]bool{}
	for _, a := range parseAnchors(string(b)) {
		if parts := strings.SplitN(a.Repo, "/", 2); len(parts) == 2 && strings.EqualFold(parts[1], repo) && set[a.Path] {
			cited[a.Path] = true
		}
	}
	for p := range cited {
		s.StaleCited = append(s.StaleCited, p)
	}
	sort.Strings(s.StaleCited)
	return s, true
}
