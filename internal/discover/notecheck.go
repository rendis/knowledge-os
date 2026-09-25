package discover

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"documentation-vault/internal/config"
)

// Note gates. G1: every source anchor resolves at its commit and the identifiers it names are in
// the cited lines. G2: every connector the code uses and every resource its configuration names
// is evidenced or explicitly addressed in the note. Freshness: anchors citing files changed since
// the analyzed commit. Optional semantic check: Jev judges whether the cited lines support the
// sentence that cites them; without Jev the reviewer does it.

var (
	permalink   = regexp.MustCompile(`https://github\.com/([^/\s]+)/([^/\s]+)/blob/([0-9a-f]{7,40})/([^)#\s]+)(?:#L(\d+)(?:-L(\d+))?)?`)
	footnoteDef = regexp.MustCompile(`(?m)^\[\^([^\]]+)\]:\s*(.*)$`)
	backtick    = regexp.MustCompile("`([^`\\n]{2,120})`")
	sentenceEnd = regexp.MustCompile(`[.!?](\s|$)`)
	hexRef      = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	versionRef  = regexp.MustCompile(`^v?\d+(\.\d+)+([-+][\w.]+)?$`)
	pathLike    = regexp.MustCompile(`^\.?[\w.\-]*(/[\w.\-]+)+/?$|^[\w\-]+\.[a-z0-9]{1,5}$`)
)

type anchor struct {
	Footnote string `json:"footnote,omitempty"`
	Repo     string `json:"repo"`
	Commit   string `json:"commit"`
	Path     string `json:"path"`
	From     int    `json:"from,omitempty"`
	To       int    `json:"to,omitempty"`
	Text     string `json:"text,omitempty"`
}

type checkIssue struct {
	Gate     string `json:"gate"`
	Severity string `json:"severity"` // error | warning | pending | review
	Where    string `json:"where"`
	Detail   string `json:"detail"`
}

type noteCheck struct {
	Note      string         `json:"note"`
	Repo      string         `json:"repo,omitempty"`
	Commit    string         `json:"commit_analizado,omitempty"`
	Anchors   map[string]int `json:"anchors"`
	Coverage  map[string]int `json:"coverage"`
	Freshness map[string]any `json:"freshness,omitempty"`
	Semantic  map[string]any `json:"semantic,omitempty"`
	Issues    []checkIssue   `json:"issues"`
	OK        bool           `json:"ok"`
}

type gitCache struct {
	commits map[string]bool
	files   map[string]*string
}

func (g *gitCache) file(repo, commit, path string) (string, bool) {
	k := repo + "\x00" + commit + "\x00" + path
	if v, ok := g.files[k]; ok {
		if v == nil {
			return "", false
		}
		return *v, true
	}
	out, e := gitOutputRaw(repo, "show", commit+":"+path)
	if e != nil {
		g.files[k] = nil
		return "", false
	}
	g.files[k] = &out
	return out, true
}

func gitOutputRaw(repo string, args ...string) (string, error) {
	cmd := execGit(repo, args...)
	b, e := cmd.Output()
	return string(b), e
}

func (g *gitCache) commit(repo, sha string) bool {
	k := repo + "\x00" + sha
	if v, ok := g.commits[k]; ok {
		return v
	}
	_, e := resolveCommit(repo, sha)
	g.commits[k] = e == nil
	return e == nil
}

func parseAnchors(note string) []anchor {
	out := []anchor{}
	defs := map[int]string{}
	for _, m := range footnoteDef.FindAllStringSubmatchIndex(note, -1) {
		defs[m[0]] = note[m[2]:m[3]]
	}
	for _, line := range strings.Split(note, "\n") {
		id := ""
		if m := footnoteDef.FindStringSubmatch(line); m != nil {
			id = m[1]
		}
		for _, m := range permalink.FindAllStringSubmatchIndex(line, -1) {
			a := anchor{Footnote: id, Repo: line[m[2]:m[3]] + "/" + line[m[4]:m[5]], Commit: line[m[6]:m[7]], Path: line[m[8]:m[9]]}
			if m[10] >= 0 {
				a.From, _ = strconv.Atoi(line[m[10]:m[11]])
				a.To = a.From
			}
			if m[12] >= 0 {
				a.To, _ = strconv.Atoi(line[m[12]:m[13]])
			}
			if id != "" {
				a.Text = line[m[1]:]
			}
			out = append(out, a)
		}
	}
	return out
}

// tokenPresent accepts an identifier literally, or a dotted key path whose parts appear in order
// (YAML nesting), inside the text.
func tokenPresent(tok, text string) bool {
	tok = strings.TrimSpace(tok)
	if strings.Contains(text, tok) {
		return true
	}
	if strings.Contains(tok, ".") && !strings.ContainsAny(tok, " /") {
		at := 0
		for _, p := range strings.Split(tok, ".") {
			i := strings.Index(text[at:], p)
			if p == "" || i < 0 {
				return false
			}
			at += i + len(p)
		}
		return true
	}
	return false
}

func lineWindow(content string, from, to, pad int) string {
	lines := strings.Split(content, "\n")
	a, b := from-1-pad, to+pad
	if a < 0 {
		a = 0
	}
	if b > len(lines) {
		b = len(lines)
	}
	if a >= b {
		return ""
	}
	return strings.Join(lines[a:b], "\n")
}

// claimFor returns the sentence(s) of the note body that cite a footnote.
func claimFor(body, id string) string {
	ref := "[^" + id + "]"
	refs := regexp.MustCompile(`\s*\[\^[^\]]+\]`)
	out := []string{}
	for _, para := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(para), "[^") || !strings.Contains(para, ref) {
			continue
		}
		idx := strings.Index(para, ref)
		// The cited sentence is the one the reference closes (references usually follow the
		// period) or, mid-sentence, the one it sits in.
		pre := strings.TrimSpace(refs.ReplaceAllString(para[:idx], ""))
		search := strings.TrimRight(pre, ".!?")
		start := 0
		for _, m := range sentenceEnd.FindAllStringIndex(search, -1) {
			start = m[1]
		}
		claim := strings.TrimSpace(pre[start:])
		if !strings.HasSuffix(pre, ".") && !strings.HasSuffix(pre, "!") && !strings.HasSuffix(pre, "?") {
			rest := refs.ReplaceAllString(para[idx:], "")
			if m := sentenceEnd.FindStringIndex(rest); m != nil {
				rest = rest[:m[1]]
			}
			claim = strings.TrimSpace(claim + " " + strings.TrimSpace(rest))
		}
		if claim != "" {
			out = append(out, claim)
		}
		if len(out) == 2 {
			break
		}
	}
	return strings.Join(out, " ")
}

func checkNote(vault, notePath, repoOverride string, semantic bool) (noteCheck, error) {
	rel, full := notePath, notePath
	if filepath.IsAbs(notePath) {
		rel, _ = filepath.Rel(vault, notePath)
	} else {
		full = filepath.Join(vault, notePath)
	}
	b, e := os.ReadFile(full)
	if e != nil {
		return noteCheck{}, e
	}
	text := string(b)
	fm := frontmatterOf(b)
	r := noteCheck{Note: filepath.ToSlash(rel), Anchors: map[string]int{}, Coverage: map[string]int{}, Issues: []checkIssue{}}
	add := func(gate, sev, where, detail string) {
		r.Issues = append(r.Issues, checkIssue{gate, sev, where, detail})
	}
	g := &gitCache{commits: map[string]bool{}, files: map[string]*string{}}
	checkouts := map[string]string{}
	locate := func(repo string) string {
		if p, ok := checkouts[repo]; ok {
			return p
		}
		loc, e := config.LocateRepository(vault, "https://github.com/"+repo+".git")
		p := ""
		if e == nil && loc["status"] == "ok" {
			p, _ = loc["path"].(string)
		}
		checkouts[repo] = p
		return p
	}
	// G1 anchors
	anchors := parseAnchors(text)
	body := text
	if i := strings.Index(text[3:], "\n---"); strings.HasPrefix(text, "---") && i >= 0 {
		body = text[3+i+4:]
	}
	type verified struct {
		a    anchor
		code string
	}
	ok := []verified{}
	// A footnote may cite several files; an identifier it names needs to be in one of them, not in each.
	siblings := map[string][]anchor{}
	for _, a := range anchors {
		if a.Footnote != "" {
			siblings[a.Footnote] = append(siblings[a.Footnote], a)
		}
	}
	inSibling := func(a anchor, tok string) bool {
		for _, s := range siblings[a.Footnote] {
			if s == a {
				continue
			}
			if p := locate(s.Repo); p != "" && g.commit(p, s.Commit) {
				if c, ok := g.file(p, s.Commit, s.Path); ok && (tokenPresent(tok, c) || tokenPresent(strings.TrimSuffix(tok, "()"), c)) {
					return true
				}
			}
		}
		return false
	}
	for _, a := range anchors {
		r.Anchors["total"]++
		where := a.Path
		if a.Footnote != "" {
			where = "[^" + a.Footnote + "] " + a.Path
		}
		repo := locate(a.Repo)
		if repo == "" {
			r.Anchors["unverifiable"]++
			add("G1-anchor", "pending", where, "no local checkout of "+a.Repo+"; configure its repository root to verify")
			continue
		}
		if !g.commit(repo, a.Commit) {
			r.Anchors["unverifiable"]++
			add("G1-anchor", "pending", where, "commit "+a.Commit+" is not in the local checkout; fetch it to verify")
			continue
		}
		content, exists := g.file(repo, a.Commit, a.Path)
		if !exists {
			r.Anchors["errors"]++
			add("G1-anchor", "error", where, "file does not exist at commit "+a.Commit)
			continue
		}
		lines := strings.Count(strings.TrimRight(content, "\n"), "\n") + 1
		if a.From > a.To || a.To > lines+2 {
			r.Anchors["errors"]++
			add("G1-anchor", "error", where, fmt.Sprintf("cited lines L%d-L%d do not exist (file has %d lines)", a.From, a.To, lines))
			continue
		}
		if a.To > lines {
			r.Anchors["warnings"]++
			add("G1-anchor", "warning", where, fmt.Sprintf("cited range L%d-L%d ends past the last line (%d)", a.From, a.To, lines))
		}
		window := content
		if a.From > 0 {
			window = lineWindow(content, a.From, a.To, 3)
		}
		bad := false
		for _, m := range backtick.FindAllStringSubmatch(a.Text, -1) {
			tok := strings.TrimSpace(m[1])
			switch {
			case hexRef.MatchString(tok) || versionRef.MatchString(tok) || len(tok) < 3 || strings.HasPrefix(tok, ".") && len(tok) <= 5:
				continue // commit ids and very short tokens are references, not code
			case pathLike.MatchString(tok):
				if _, exists := g.file(repo, a.Commit, strings.TrimPrefix(tok, "./")); exists || tokenPresent(tok, content) {
					continue
				}
				bad = true
				r.Anchors["errors"]++
				add("G1-anchor", "error", where, "`"+tok+"` names a path that does not exist at this commit")
				continue
			}
			if tokenPresent(tok, window) || tokenPresent(strings.TrimSuffix(tok, "()"), window) {
				continue
			}
			if tokenPresent(tok, content) || tokenPresent(strings.TrimSuffix(tok, "()"), content) {
				r.Anchors["warnings"]++
				add("G1-anchor", "warning", where, "`"+tok+"` is in the file but outside the cited lines")
				continue
			}
			if inSibling(a, tok) {
				continue
			}
			if strings.ContainsAny(tok, " {}(,=") {
				r.Anchors["review"]++
				add("G1-anchor", "review", where, "code fragment `"+tok+"` is not literal in the cited file; confirm it is a faithful paraphrase")
				continue
			}
			bad = true
			r.Anchors["errors"]++
			add("G1-anchor", "error", where, "`"+tok+"` does not appear in the cited file at this commit")
		}
		if !bad {
			r.Anchors["verified"]++
			code := window
			if a.From == 0 && len(code) > 4000 {
				code = code[:4000]
			}
			ok = append(ok, verified{a, code})
		}
	}
	// G2 coverage against the repository's own facts at the analyzed commit.
	// The repository is named by the note's own aliases (candidates may live outside the vault).
	repoName := repoOverride
	if repoName == "" {
		repoName = noteRepository(vault, full, fm)
	}
	commit := strings.Trim(fm["commit-analizado"], `"'`)
	r.Repo, r.Commit = repoName, commit
	if repoName != "" && commit != "" {
		inputs, e := discoverRepositories(vault, map[string]bool{repoName: true})
		if e == nil && len(inputs) == 1 {
			in := inputs[0]
			ref, refErr := in.Ref, in.RefErr
			in.Ref = commit
			if s, e := scanRepository(in); e != nil {
				add("G2-coverage", "pending", repoName, "cannot scan the analyzed commit: "+e.Error())
			} else {
				resolveLibraries(append([]*repoScan{s}, libraryContext(vault, []*repoScan{s})...))
				st, _ := loadStore(vault)
				ix, _ := loadPlatform(vault)
				a := &assembly{scans: []*repoScan{s}, st: st, platform: ix, providers: configuredProviders(vault)}
				if jev := newJev(); jev != nil {
					if qs := a.pendingQuestions(); len(qs) > 0 && len(qs) <= 500 {
						if n, _ := jev.answerAll(context.Background(), st, qs); n > 0 {
							_ = st.save(vault)
						}
					}
				}
				f := a.facts()[0]
				cited := map[string]bool{}
				for _, an := range anchors {
					if strings.EqualFold(strings.SplitN(an.Repo, "/", 2)[1], repoName) {
						cited[an.Path] = true
					}
				}
				// File-level references (a backticked repository path) are weaker than
				// permalinks but still name real evidence at the analyzed commit.
				files := map[string]bool{}
				for _, f := range s.files() {
					files[f] = true
				}
				for _, m := range backtick.FindAllStringSubmatch(body, -1) {
					if p := strings.TrimPrefix(strings.TrimSpace(m[1]), "./"); files[p] {
						cited[p] = true
					}
				}
				lower := strings.ToLower(text)
				linked := []string{}
				for _, field := range edgeFields {
					for _, m := range wikilink.FindAllStringSubmatch(fm[field], -1) {
						linked = append(linked, m[1])
					}
				}
				// One requirement per connector category: evidence in a file that uses it, or its name.
				type need struct {
					files, names []string
				}
				categories := map[string]*need{}
				for _, d := range f.Dependencies {
					if !isConnectorCategory(d.Category) {
						continue
					}
					n := categories[d.Category]
					if n == nil {
						n = &need{}
						categories[d.Category] = n
					}
					n.files = append(n.files, d.Files...)
					n.names = append(n.names, d.ID[strings.Index(d.ID, ":")+1:])
				}
				for _, cat := range sortedKeys(categories) {
					n := categories[cat]
					r.Coverage["connector_categories"]++
					hit := false
					for _, file := range n.files {
						hit = hit || cited[file]
					}
					for _, nm := range n.names {
						hit = hit || strings.Contains(lower, strings.ToLower(nm))
					}
					if hit {
						r.Coverage["connector_categories_evidenced"]++
					} else {
						add("G2-coverage", "error", cat, fmt.Sprintf("connector (%s) used in %s is neither cited nor named", strings.Join(firstList(n.names, 3), ", "), strings.Join(firstList(n.files, 3), ", ")))
					}
				}
				// One requirement per logical resource group declared by this repository.
				groups := map[string]*need{}
				for _, res := range f.Resources {
					own := false
					files := []string{}
					for _, ev := range res.Evidence {
						if ev.Kind == "config" || ev.Kind == "code" {
							own = true
							files = append(files, ev.File)
						}
					}
					key := resourceGroup(res)
					if !own || key == "" || len(f.Languages) == 0 {
						// Infrastructure and schema repositories declare many resources; their
						// notes summarize them and cell-wide gaps are reported by `discover run`.
						continue
					}
					n := groups[key]
					if n == nil {
						n = &need{}
						groups[key] = n
					}
					n.files = append(n.files, files...)
					n.names = append(n.names, normalizeResource(res.Name))
				}
				for _, key := range sortedKeys(groups) {
					n := groups[key]
					r.Coverage["resource_groups"]++
					hit := strings.Contains(lower, strings.SplitN(key, " ", 2)[1])
					for _, nm := range n.names {
						hit = hit || strings.Contains(lower, nm)
						for _, l := range linked {
							hit = hit || nameMatch(l, nm)
						}
					}
					for _, file := range n.files {
						hit = hit || cited[file]
					}
					if hit {
						r.Coverage["resource_groups_addressed"]++
					} else {
						add("G2-coverage", "error", key, fmt.Sprintf("declared in %s (%s) but not named, linked or cited", strings.Join(firstList(uniqueStrings(n.files), 2), ", "), strings.Join(firstList(uniqueStrings(n.names), 3), ", ")))
					}
				}
				if r.Anchors["total"] == 0 {
					add("G1-anchor", "warning", r.Note, "the note has no source permalinks; claims cannot be verified mechanically (add anchors on its next update)")
				}
				for _, p := range f.Pending {
					add("pending", "pending", p.Subject, p.Kind+": "+p.Detail)
				}
				if cmp, e := compareNotes(vault, []repoFacts{f}); e == nil {
					for _, c := range cmp {
						for _, d := range c.Discrepancies {
							add("G3-relation", "review", d.Target, fmt.Sprintf("the note declares %s [[%s]] but the repository evidence at the analyzed commit names %s; verify it before relying on it", d.Field, d.Target, strings.Join(firstList(d.Found, 4), ", ")))
						}
					}
				}
				// Freshness: anchors on files changed since the analyzed commit.
				head, e := resolveCommit(in.Path, ref)
				if refErr != "" {
					add("freshness", "pending", repoName, refErr)
				}
				full, _ := resolveCommit(in.Path, commit)
				if e == nil && full != "" && head != full {
					changed, _ := gitOutput(in.Path, "diff", "--name-only", full, head)
					set := map[string]bool{}
					for _, c := range strings.Split(changed, "\n") {
						if c != "" {
							set[c] = true
						}
					}
					stale := []string{}
					for p := range cited {
						if set[p] {
							stale = append(stale, p)
						}
					}
					sort.Strings(stale)
					r.Freshness = map[string]any{"ref": ref, "head": head[:12], "changed_files": len(set), "stale_cited_files": stale}
				} else if e == nil {
					r.Freshness = map[string]any{"ref": ref, "head": head[:12], "changed_files": 0, "stale_cited_files": []string{}}
				}
			}
		}
	}
	// Optional semantic check of each verified footnote anchor.
	if semantic {
		jev := newJev()
		if jev == nil {
			r.Semantic = map[string]any{"status": "skipped", "reason": "TYPESAFE_API_KEY not set; the reviewer judges whether each citation supports its sentence"}
		} else {
			counts := map[string]int{}
			for _, v := range ok {
				if v.a.Footnote == "" {
					continue
				}
				claim := claimFor(body, v.a.Footnote)
				if claim == "" || strings.TrimSpace(v.code) == "" {
					continue
				}
				q := question{ID: v.a.Footnote, Kind: "citation", State: map[string]any{"claim": claim, "code": v.code},
					Instructions: "The `claim` (it may be written in Spanish) cites the source `code`. How does the code relate to the claim? Judge only from this code.",
					Options:      map[string]string{"supports": "The code states the claim or directly implies it.", "contradicts": "The code shows something incompatible with the claim.", "says_nothing": "The code does not address the claim."}}
				j, e := jev.ask(context.Background(), q)
				if e != nil {
					counts["failed"]++
					continue
				}
				counts[j.Choice]++
				where := "[^" + v.a.Footnote + "] " + v.a.Path
				switch {
				case j.Choice == "contradicts" && j.Confidence >= 0.7:
					add("semantic", "error", where, fmt.Sprintf("cited code contradicts the sentence (confidence %.2f): %s", j.Confidence, trim(claim, 160)))
				case j.Choice != "supports" || j.Confidence < 0.6:
					counts["to_review"]++
					add("semantic", "review", where, fmt.Sprintf("%s (confidence %.2f): %s", j.Choice, j.Confidence, trim(claim, 160)))
				}
			}
			r.Semantic = map[string]any{"status": "checked", "results": counts, "jev_calls": jev.calls}
		}
	}
	r.OK = true
	for _, i := range r.Issues {
		if i.Severity == "error" {
			r.OK = false
		}
	}
	return r, nil
}

var hostRE = regexp.MustCompile(`^(?:[a-z]+://)?([^/:?#]+)`)

// resourceGroup is the unit a note must address: a logical topic/subscription, an HTTP host's
// registrable domain, a database schema, or a bucket. Local and reserved example hosts are skipped.
func resourceGroup(r resource) string {
	n := normalizeResource(r.Name)
	switch r.Type {
	case "message_topic", "message_subscription":
		return "messaging " + logicalName(n)
	case "http_endpoint":
		m := hostRE.FindStringSubmatch(n)
		if m == nil || !strings.Contains(m[1], ".") || loopback(m[1]) {
			return ""
		}
		labels := strings.Split(m[1], ".")
		if len(labels) >= 2 {
			domain := strings.Join(labels[len(labels)-2:], ".")
			if domain == "example.com" || domain == "example.org" || strings.HasSuffix(m[1], ".local") || strings.HasSuffix(m[1], ".svc") || strings.Contains(m[1], ".svc.") {
				return ""
			}
			return "http " + domain
		}
		return ""
	case "database_object":
		head := strings.SplitN(n, ".", 2)[0]
		if loopback(n) || strings.HasPrefix(n, "-") {
			return ""
		}
		return "database " + head
	case "storage_bucket":
		return "storage " + n
	}
	return ""
}

// loopback hosts are local development endpoints by definition (RFC 6761 / RFC 5735).
func loopback(h string) bool {
	return strings.HasPrefix(h, "localhost") || strings.HasPrefix(h, "127.") || strings.HasPrefix(h, "0.0.0.0") || strings.Contains(h, "://localhost") || strings.Contains(h, "://127.")
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func uniqueStrings(xs []string) []string {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return firstN(m, len(m))
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func firstList(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

func runCheck(o options, out io.Writer, semantic bool) error {
	if len(o.notes) == 0 {
		return errors.New("--note is required")
	}
	results := []noteCheck{}
	failed := 0
	for _, n := range o.notes {
		repo := ""
		if len(o.repos) == 1 {
			repo = o.repos[0]
		}
		r, e := checkNote(o.vault, n, repo, semantic)
		if e != nil {
			return e
		}
		if !r.OK {
			failed++
		}
		results = append(results, r)
	}
	if e := emit(out, map[string]any{"notes": results, "failed": failed}); e != nil {
		return e
	}
	if failed > 0 {
		return fmt.Errorf("%d note(s) failed the gates", failed)
	}
	return nil
}

// CheckNote runs the note gates for one note or candidate and reports whether it passed.
func CheckNote(vault, note string) (bool, any, error) {
	r, e := checkNote(vault, note, "", false)
	return r.OK, r, e
}
