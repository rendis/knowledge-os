// Package gitsync publishes knowledge through Git: a local branch is the synchronization run,
// commits are its checkpoints, a review commit binds the accepted content by digest and records
// it in the tree (90-Meta/sync-reviews/), and a fast-forward merge publishes it. No other run state exists.
package gitsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"knowledge-os/internal/audit"
	"knowledge-os/internal/cases"
	"knowledge-os/internal/check"
	"knowledge-os/internal/config"
	"knowledge-os/internal/discover"
)

const Help = `sync COMMAND --vault PATH [options]
  start   --name SLUG [--base BRANCH]   Create and switch to branch sync/SLUG from the base.
  status                                Branch, base, changed knowledge files and review state.
  review  --verdict accept|revise --reviewer NAME [--summary TEXT]
                                        Record the reviewer's verdict on the committed content
                                        as a commit whose trailers bind it by digest and that adds a
                                        record under 90-Meta/sync-reviews/ (it survives rebase and
                                        squash merges).
  verify  [--base REV] [--allow-no-change]
                                        Gates for the branch: every changed repository note passes
                                        discover check, no new audit/link/Bases issue versus the
                                        base, clean tree, and an accepted review of the final content.
                                        --allow-no-change accepts a range without knowledge changes
                                        (the CI gate on pushes, e.g. a kernel update).
  acknowledge --repo NAME --commit SHA --decision D [--branch main] [--date YYYY-MM-DD]
                                        Record a reviewed repository commit that needs no note change
                                        (D: no-documentation-change | no-durable-node |
                                        review-rejected | inspection-limited).
  finish  [--base BRANCH]               verify, then fast-forward the base and delete the branch.
  pull    [--base BRANCH]               Fetch; fast-forward the base when possible. On divergence or
                                        on a sync/ branch whose base moved, list the knowledge files
                                        changed on both sides (they need the merged meaning reviewed).
Publishing to the remote follows the repository's Git policy. All output is JSON.`

const (
	ackRel     = "90-Meta/.sync-acknowledgements.json"
	reviewsDir = "90-Meta/sync-reviews"
)

var (
	knowledgePath = regexp.MustCompile(`^([1-7]\d-[^/]+/.+\.md|00-Home\.md|[^/]+\.base|90-Meta/\.sync-acknowledgements\.json|investigations/[^/]+/.+)$`)
	slugRE        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,80}$`)
	sha12         = regexp.MustCompile(`^[0-9a-f]{12,40}$`)
	decisions     = map[string]bool{"no-documentation-change": true, "no-durable-node": true, "review-rejected": true, "inspection-limited": true}
)

type opts struct {
	vault, name, base, verdict, reviewer, summary, repo, commit, decision, branch, date string
	allowNoChange                                                                       bool
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotePath=false", "-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

// Run dispatches the sync subcommands.
func Run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	cmd := args[0]
	o := opts{branch: "main"}
	fields := map[string]*string{"--vault": &o.vault, "--name": &o.name, "--base": &o.base, "--verdict": &o.verdict, "--reviewer": &o.reviewer,
		"--summary": &o.summary, "--repo": &o.repo, "--commit": &o.commit, "--decision": &o.decision, "--branch": &o.branch, "--date": &o.date}
	for i := 1; i < len(args); i++ {
		if args[i] == "--allow-no-change" {
			o.allowNoChange = true
			continue
		}
		p, ok := fields[args[i]]
		if !ok || i+1 >= len(args) {
			return fmt.Errorf("unknown or incomplete option %s", args[i])
		}
		i++
		*p = args[i]
	}
	if o.vault == "" {
		return errors.New("--vault is required")
	}
	r, e := config.Resolve(o.vault)
	if e != nil {
		return e
	}
	o.vault, _ = r["vault_root"].(string)
	if o.base == "" {
		o.base = defaultBase(o.vault, cmd == "start")
	}
	switch cmd {
	case "start":
		return start(o, out)
	case "status":
		s, e := status(o)
		if e != nil {
			return e
		}
		return emit(out, s)
	case "review":
		return review(o, out)
	case "verify":
		res, e := verify(o)
		if e != nil {
			return e
		}
		if err := emit(out, res); err != nil {
			return err
		}
		if !res["ok"].(bool) {
			return errors.New("sync verification failed")
		}
		return nil
	case "acknowledge":
		return acknowledge(o, out)
	case "finish":
		return finish(o, out)
	case "pull":
		return pull(o, out)
	}
	return fmt.Errorf("unknown sync command %q; see sync --help", cmd)
}

// defaultBase is the branch a sync starts from and publishes to: the one recorded when the sync branch
// was started, else the branch checked out when starting, else the remote's default branch, else main.
// The local origin/HEAD is only a cache of the remote's default and can be stale.
func defaultBase(vault string, starting bool) string {
	cur := current(vault)
	if b, e := git(vault, "config", "--get", "branch."+cur+".vault-base"); e == nil && b != "" {
		return b
	}
	if starting && cur != "" && cur != "HEAD" && !strings.HasPrefix(cur, "sync/") {
		return cur
	}
	if r, e := git(vault, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); e == nil && r != "" {
		return strings.TrimPrefix(r, "origin/")
	}
	return "main"
}

func current(vault string) string {
	b, _ := git(vault, "rev-parse", "--abbrev-ref", "HEAD")
	return b
}

func dirty(vault string) ([]string, error) {
	s, e := git(vault, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return nil, e
	}
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if len(l) > 3 && knowledgePath.MatchString(strings.Trim(l[3:], `"`)) {
			out = append(out, strings.Trim(l[3:], `"`))
		}
	}
	return out, nil
}

func start(o opts, out io.Writer) error {
	if !slugRE.MatchString(o.name) {
		return errors.New("--name must be a lowercase slug")
	}
	if d, e := dirty(o.vault); e != nil || len(d) > 0 {
		return fmt.Errorf("uncommitted knowledge changes %v: commit or set them aside before starting", d)
	}
	branch := "sync/" + o.name
	if _, e := git(o.vault, "switch", "-c", branch, o.base); e != nil {
		return e
	}
	if _, e := git(o.vault, "config", "branch."+branch+".vault-base", o.base); e != nil {
		return e
	}
	return emit(out, map[string]any{"branch": branch, "base": o.base, "next": "write the notes, commit, run `discover check`, record the review with `sync review`, then `sync verify` and `sync finish`"})
}

// changed lists knowledge files that differ between the merge base and HEAD.
func changed(o opts) ([]string, string, error) {
	mb, e := git(o.vault, "merge-base", o.base, "HEAD")
	if e != nil {
		return nil, "", e
	}
	d, e := git(o.vault, "diff", "--name-only", "--no-renames", mb, "HEAD")
	if e != nil {
		return nil, "", e
	}
	files := []string{}
	for _, f := range strings.Split(d, "\n") {
		if f != "" && knowledgePath.MatchString(f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, mb, nil
}

// stateOnly reports whether the branch changes only the versioned discovery state.
func stateOnly(vault, mb string) bool {
	d, e := git(vault, "diff", "--name-only", "--no-renames", mb, "HEAD")
	if e != nil || d == "" {
		return false
	}
	for _, f := range strings.Split(d, "\n") {
		if !strings.HasPrefix(f, "90-Meta/discovery/") {
			return false
		}
	}
	return true
}

// digest binds the exact content of the changed files at HEAD (deletions included).
func digest(vault string, files []string) string {
	h := sha256.New()
	for _, f := range files {
		blob, e := git(vault, "rev-parse", "--verify", "--quiet", "HEAD:"+f)
		if e != nil {
			blob = "deleted"
		}
		fmt.Fprintf(h, "%s\x00%s\n", f, blob)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// digestAt is digest over the files' content at rev.
func digestAt(vault, rev string, files []string) string {
	h := sha256.New()
	for _, f := range files {
		blob, e := git(vault, "rev-parse", "--verify", "--quiet", rev+":"+f)
		if e != nil {
			blob = "deleted"
		}
		fmt.Fprintf(h, "%s\x00%s\n", f, blob)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func knowledgeChanges(vault, from, to string) []string {
	d, e := git(vault, "diff", "--name-only", "--no-renames", from, to)
	if e != nil {
		return nil
	}
	out := []string{}
	for _, f := range strings.Split(d, "\n") {
		if f != "" && knowledgePath.MatchString(f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// reviewedRange checks a range that may hold several finished syncs, as a push to the base branch does. Each
// accepted review covers the commits of its sync, from the base it records (Knowledge-Base) to the review itself,
// when its digest matches the knowledge changes of that span. A review recorded before Knowledge-Base existed
// covers from the nearest earlier point whose changes match its digest. A commit that changes knowledge outside
// every covered span, including after the last review, is unreviewed.
func reviewedRange(vault, base string) []string {
	list, e := git(vault, "rev-list", "--reverse", "--first-parent", base+"..HEAD")
	if e != nil {
		return []string{"cannot list the range: " + e.Error()}
	}
	points := []string{base}
	for _, c := range strings.Split(list, "\n") {
		if c != "" {
			points = append(points, c)
		}
	}
	covered := make([]bool, len(points))
	for i := 1; i < len(points); i++ {
		body, _ := git(vault, "log", "-1", "--format=%B", points[i])
		if !strings.Contains(body, "Knowledge-Review: accept") {
			continue
		}
		want, from := "", ""
		for _, l := range strings.Split(body, "\n") {
			if v, ok := strings.CutPrefix(l, "Knowledge-Digest:"); ok {
				want = strings.TrimSpace(v)
			}
			if v, ok := strings.CutPrefix(l, "Knowledge-Base:"); ok {
				from = strings.TrimSpace(v)
			}
		}
		if from != "" {
			start := -1
			for p := i - 1; p >= 0; p-- {
				if points[p] == from {
					start = p
					break
				}
			}
			if _, e := git(vault, "merge-base", "--is-ancestor", from, points[0]); start < 0 && e == nil {
				start = 0 // the sync started before the checked range
			}
			if start >= 0 && digestAt(vault, points[i], knowledgeChanges(vault, from, points[i])) == want {
				for k := start + 1; k <= i; k++ {
					covered[k] = true
				}
			}
			continue
		}
		for p := i - 1; p >= 0; p-- {
			if files := knowledgeChanges(vault, points[p], points[i]); len(files) > 0 && digestAt(vault, points[i], files) == want {
				for k := p + 1; k <= i; k++ {
					covered[k] = true
				}
				break
			}
		}
	}
	problems := []string{}
	for i := 1; i < len(points); i++ {
		if !covered[i] && len(knowledgeChanges(vault, points[i-1], points[i])) > 0 {
			problems = append(problems, fmt.Sprintf("commit %s changes knowledge that no accepted review covers; publish knowledge through a reviewed sync/ branch", points[i][:12]))
		}
	}
	return problems
}

type blobChange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// reviewFile is the review record kept in the tree, so it survives any merge method that keeps the content.
type reviewFile struct {
	Verdict  string                `json:"verdict"`
	Reviewer string                `json:"reviewer"`
	Summary  string                `json:"summary,omitempty"`
	Base     string                `json:"base"`
	Digest   string                `json:"digest"`
	Files    map[string]blobChange `json:"files"`
}

func blobAt(vault, rev, f string) string {
	if b, e := git(vault, "rev-parse", "--verify", "--quiet", rev+":"+f); e == nil {
		return b
	}
	return "deleted"
}

// reviewedContent checks a range by content, whatever merge method brought it: every knowledge file the range
// changes must go from its content at base to its content at HEAD through the transitions of accepted review
// records the range adds. An edit no review saw breaks the chain. It returns the problems and whether the
// range adds any record.
func reviewedContent(vault, base string) ([]string, bool) {
	added, e := git(vault, "diff", "--name-only", "--no-renames", "--diff-filter=A", base, "HEAD", "--", reviewsDir)
	if e != nil {
		return []string{"cannot list the review records: " + e.Error()}, false
	}
	recs := []reviewFile{}
	for _, p := range strings.Split(added, "\n") {
		if !strings.HasSuffix(p, ".json") {
			continue
		}
		body, e := git(vault, "show", "HEAD:"+p)
		var r reviewFile
		if e == nil && json.Unmarshal([]byte(body), &r) == nil && r.Verdict == "accept" {
			recs = append(recs, r)
		}
	}
	problems := []string{}
	for _, f := range knowledgeChanges(vault, base, "HEAD") {
		cur, want := blobAt(vault, base, f), blobAt(vault, "HEAD", f)
		for step := 0; cur != want && step < len(recs); step++ {
			next := cur
			for _, r := range recs {
				if c, ok := r.Files[f]; ok && c.From == cur && c.To != cur {
					next = c.To
					break
				}
			}
			if next == cur {
				break
			}
			cur = next
		}
		if cur != want {
			problems = append(problems, fmt.Sprintf("%s changes knowledge that no accepted review covers; publish knowledge through a reviewed sync/ branch", f))
		}
	}
	return problems, added != ""
}

type reviewRecord struct {
	Commit, Verdict, Reviewer, Digest string
}

func lastReview(o opts, mb string) *reviewRecord {
	logs, e := git(o.vault, "log", "--format=%H%x1f%B%x1e", mb+"..HEAD")
	if e != nil {
		return nil
	}
	for _, entry := range strings.Split(logs, "\x1e") {
		parts := strings.SplitN(strings.TrimSpace(entry), "\x1f", 2)
		if len(parts) != 2 || !strings.Contains(parts[1], "Knowledge-Review:") {
			continue
		}
		rr := &reviewRecord{Commit: parts[0]}
		for _, l := range strings.Split(parts[1], "\n") {
			k, v, ok := strings.Cut(l, ":")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch k {
			case "Knowledge-Review":
				rr.Verdict = v
			case "Knowledge-Reviewer":
				rr.Reviewer = v
			case "Knowledge-Digest":
				rr.Digest = v
			}
		}
		return rr
	}
	return nil
}

func status(o opts) (map[string]any, error) {
	files, mb, e := changed(o)
	if e != nil {
		return nil, e
	}
	d := digest(o.vault, files)
	pending, _ := dirty(o.vault)
	s := map[string]any{"branch": current(o.vault), "base": o.base, "changed": files, "uncommitted": pending, "content_digest": d, "review": nil}
	if rr := lastReview(o, mb); rr != nil {
		s["review"] = map[string]any{"verdict": rr.Verdict, "reviewer": rr.Reviewer, "commit": rr.Commit[:12], "covers_current_content": rr.Digest == d}
	}
	return s, nil
}

func review(o opts, out io.Writer) error {
	if o.verdict != "accept" && o.verdict != "revise" {
		return errors.New("--verdict must be accept or revise")
	}
	if strings.TrimSpace(o.reviewer) == "" {
		return errors.New("--reviewer is required (who reviewed: a fresh agent session or a person)")
	}
	if !strings.HasPrefix(current(o.vault), "sync/") {
		return errors.New("record reviews on a sync/ branch")
	}
	if d, _ := dirty(o.vault); len(d) > 0 {
		return fmt.Errorf("commit the reviewed content first: %v", d)
	}
	files, mb, e := changed(o)
	if e != nil {
		return e
	}
	if len(files) == 0 {
		return errors.New("nothing to review: no knowledge change on this branch")
	}
	d := digest(o.vault, files)
	rec := reviewFile{Verdict: o.verdict, Reviewer: o.reviewer, Summary: strings.TrimSpace(o.summary), Base: mb, Digest: d, Files: map[string]blobChange{}}
	for _, f := range files {
		rec.Files[f] = blobChange{From: blobAt(o.vault, mb, f), To: blobAt(o.vault, "HEAD", f)}
	}
	// The record is content, not only history: a rebase merge drops empty commits and a squash may drop trailers.
	rel := fmt.Sprintf("%s/%s-%s-%s.json", reviewsDir, time.Now().UTC().Format("20060102T150405Z"), o.verdict, d[:12])
	b, _ := json.MarshalIndent(rec, "", "  ")
	if e := os.MkdirAll(filepath.Join(o.vault, reviewsDir), 0o755); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(o.vault, rel), append(b, '\n'), 0o644); e != nil {
		return e
	}
	if _, e := git(o.vault, "add", "--", rel); e != nil {
		return e
	}
	msg := fmt.Sprintf("chore(sync): record review %s\n\n%s\n\nKnowledge-Review: %s\nKnowledge-Reviewer: %s\nKnowledge-Digest: %s\nKnowledge-Base: %s\n", o.verdict, strings.TrimSpace(o.summary), o.verdict, o.reviewer, d, mb)
	if _, e := git(o.vault, "commit", "-q", "-m", msg); e != nil {
		return e
	}
	return emit(out, map[string]any{"recorded": o.verdict, "files": files, "digest": d, "record": rel})
}

// structural returns audit, link and Bases issues of a vault tree as comparable strings.
func structural(root string) ([]string, error) {
	out := []string{}
	_, issues, e := audit.Audit(root)
	if e != nil {
		return nil, e
	}
	for _, i := range issues {
		out = append(out, "audit: "+i)
	}
	l, e := check.Links(root)
	if e != nil {
		return nil, e
	}
	for _, b := range l.Broken {
		out = append(out, "broken link: "+b)
	}
	for _, d := range l.Duplicates {
		out = append(out, "duplicate: "+d)
	}
	for _, orphan := range l.OrphanPaths {
		out = append(out, "orphan: "+orphan)
	}
	b, e := check.Bases(root)
	if e != nil {
		return nil, e
	}
	for _, i := range b.Issues {
		out = append(out, "bases: "+i)
	}
	return out, nil
}

// baseTree materializes a commit's tree in a temporary directory.
func baseTree(vault, rev string) (string, func(), error) {
	dir, e := os.MkdirTemp("", "vault-base-")
	if e != nil {
		return "", nil, e
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	archive := exec.Command("git", "-C", vault, "archive", rev)
	untar := exec.Command("tar", "-x", "-C", dir)
	pipe, e := archive.StdoutPipe()
	if e != nil {
		cleanup()
		return "", nil, e
	}
	untar.Stdin = pipe
	if e := untar.Start(); e != nil {
		cleanup()
		return "", nil, e
	}
	if e := archive.Run(); e != nil {
		cleanup()
		return "", nil, e
	}
	if e := untar.Wait(); e != nil {
		cleanup()
		return "", nil, e
	}
	return dir, cleanup, nil
}

var (
	analyzed        = regexp.MustCompile(`(?m)^commit-analizado:.*$`)
	knowledgeFolder = regexp.MustCompile(`^[1-7]\d-`)
)

func verify(o opts) (map[string]any, error) {
	res := map[string]any{"branch": current(o.vault), "base": o.base}
	problems := []string{}
	if d, _ := dirty(o.vault); len(d) > 0 {
		problems = append(problems, fmt.Sprintf("uncommitted knowledge changes: %v", d))
	}
	files, mb, e := changed(o)
	if e != nil {
		return nil, e
	}
	res["changed"] = files
	notes := []any{}
	for _, f := range files {
		if !strings.HasPrefix(f, "20-Repos/") {
			continue
		}
		if _, e := os.Stat(filepath.Join(o.vault, f)); e != nil {
			continue
		}
		// The synced note (new, or with a new commit-analizado) passes every gate; a neighbour touched
		// to re-anchor may keep pre-existing errors but must not add any.
		var base []byte
		if old, e := git(o.vault, "show", mb+":"+f); e == nil {
			cur, _ := os.ReadFile(filepath.Join(o.vault, f))
			if analyzed.FindString(old) == analyzed.FindString(string(cur)) {
				base = []byte(old + "\n")
			}
		}
		ok, introduced, preexisting, e := discover.CheckNoteIntroduced(o.vault, f, base)
		if e != nil {
			return nil, e
		}
		entry := map[string]any{"note": f, "ok": ok, "synced": base == nil}
		if preexisting > 0 {
			entry["preexisting_errors"] = preexisting
		}
		notes = append(notes, entry)
		if !ok {
			problems = append(problems, fmt.Sprintf("note gates failed: %s, %d error(s) (run `discover check --note %s`)", f, len(introduced), f))
		}
	}
	res["note_gates"] = notes
	// A topic, flow, integration or other knowledge note changed on the branch must not copy a repository
	// note: the repository note owns the fact and the others link it. Copies its base already had are debt.
	copies := []any{}
	for _, f := range files {
		top := strings.SplitN(f, "/", 2)[0]
		if !strings.HasSuffix(f, ".md") || top == "20-Repos" || !knowledgeFolder.MatchString(top) {
			continue
		}
		if _, e := os.Stat(filepath.Join(o.vault, f)); e != nil {
			continue
		}
		var base []byte
		if old, e := git(o.vault, "show", mb+":"+f); e == nil {
			base = []byte(old + "\n")
		}
		introduced, e := discover.CopiesIntroduced(o.vault, f, base)
		if e != nil {
			return nil, e
		}
		if len(introduced) > 0 {
			copies = append(copies, map[string]any{"note": f, "copied": introduced})
			problems = append(problems, fmt.Sprintf("%s copies %d paragraph(s) of repository notes; keep each fact in its repository note and link that note", f, len(introduced)))
		}
	}
	res["copied_paragraphs"] = copies
	caseGates := []any{}
	for _, f := range files {
		if !strings.HasPrefix(f, "investigations/") || filepath.Base(f) != "investigation.md" {
			continue
		}
		if _, e := os.Stat(filepath.Join(o.vault, f)); e != nil {
			continue // retired: removed on this branch
		}
		var base []byte
		if old, e := git(o.vault, "show", mb+":"+f); e == nil {
			base = []byte(old + "\n")
		}
		ok, introduced, preexisting, e := cases.CheckIntroduced(o.vault, f, base)
		if e != nil {
			return nil, e
		}
		entry := map[string]any{"case": f, "ok": ok}
		if preexisting > 0 {
			entry["preexisting_errors"] = preexisting
		}
		caseGates = append(caseGates, entry)
		if !ok {
			problems = append(problems, fmt.Sprintf("case gate failed: %s, %d error(s) (run `investigation check`)", f, len(introduced)))
		}
	}
	res["case_gates"] = caseGates
	repoNotes := []string{}
	for _, n := range notes {
		repoNotes = append(repoNotes, n.(map[string]any)["note"].(string))
	}
	stale, e := discover.StaleNeighbours(o.vault, repoNotes)
	if e != nil {
		return nil, e
	}
	res["stale_neighbours"] = stale
	if len(stale) > 0 {
		problems = append(problems, fmt.Sprintf("%d note(s) still cite changed files of the synced repository at an older commit; update or re-anchor them", len(stale)))
	}
	now, e := structural(o.vault)
	if e != nil {
		return nil, e
	}
	dir, cleanup, e := baseTree(o.vault, mb)
	if e != nil {
		return nil, e
	}
	defer cleanup()
	if cfg, e := os.ReadFile(filepath.Join(o.vault, ".knowledge-os-config.yaml")); e == nil {
		_ = os.WriteFile(filepath.Join(dir, ".knowledge-os-config.yaml"), cfg, 0o644)
	}
	before, e := structural(dir)
	if e != nil {
		return nil, e
	}
	old := map[string]bool{}
	for _, i := range before {
		old[i] = true
	}
	introduced := []string{}
	for _, i := range now {
		if !old[i] {
			introduced = append(introduced, i)
		}
	}
	res["new_structural_issues"] = introduced
	res["preexisting_structural_issues"] = len(before)
	if len(introduced) > 0 {
		problems = append(problems, fmt.Sprintf("%d structural issue(s) introduced versus the base", len(introduced)))
	}
	d := digest(o.vault, files)
	rr := lastReview(o, mb)
	state := len(files) == 0 && stateOnly(o.vault, mb)
	switch {
	case state:
		if e := discover.ValidateState(o.vault); e != nil {
			problems = append(problems, "discovery state: "+e.Error())
		}
		res["discovery_state_only"] = true
	case len(files) == 0 && o.allowNoChange:
		res["no_knowledge_change"] = true
	case o.allowNoChange:
		// A pushed range can hold several finished syncs, merged by any method: the review records in the tree
		// cover it by content. Reviews recorded only as commit trailers (before the records) are checked on the
		// first-parent history.
		byContent, recorded := reviewedContent(o.vault, mb)
		if len(byContent) > 0 {
			if byHistory := reviewedRange(o.vault, mb); len(byHistory) == 0 {
				byContent = nil
			} else if !recorded {
				byContent = byHistory
			}
		}
		problems = append(problems, byContent...)
	case len(files) == 0:
		problems = append(problems, "no knowledge change on this branch")
	case rr == nil:
		problems = append(problems, "no review recorded; run `sync review` after the reviewer's verdict")
	case rr.Verdict != "accept":
		problems = append(problems, "last review verdict is "+rr.Verdict)
	case rr.Digest != d:
		problems = append(problems, "content changed after the accepted review; the reviewer must check the new changes")
	}
	if rr != nil {
		res["review"] = map[string]any{"verdict": rr.Verdict, "reviewer": rr.Reviewer, "covers_current_content": rr.Digest == d}
	}
	res["problems"] = problems
	res["ok"] = len(problems) == 0
	return res, nil
}

func finish(o opts, out io.Writer) error {
	branch := current(o.vault)
	if !strings.HasPrefix(branch, "sync/") {
		return errors.New("finish runs on a sync/ branch")
	}
	res, e := verify(o)
	if e != nil {
		return e
	}
	if !res["ok"].(bool) {
		_ = emit(out, res)
		return errors.New("sync verification failed; nothing merged")
	}
	if _, e := git(o.vault, "switch", o.base); e != nil {
		return e
	}
	if _, e := git(o.vault, "merge", "--ff-only", branch); e != nil {
		_, _ = git(o.vault, "switch", branch)
		return fmt.Errorf("base moved: rebase %s onto %s, re-run the gates and review the merged meaning: %w", branch, o.base, e)
	}
	if _, e := git(o.vault, "branch", "-d", branch); e != nil {
		return e
	}
	head, _ := git(o.vault, "rev-parse", "--short=12", "HEAD")
	return emit(out, map[string]any{"merged": branch, "base": o.base, "head": head, "next": "publish the base branch following the repository's Git policy"})
}

type ackEntry struct {
	AnalysisDate string `json:"analysis_date"`
	AnalyzedSHA  string `json:"analyzed_sha"`
	Branch       string `json:"branch"`
	Decision     string `json:"decision"`
	Repository   string `json:"repository"`
}

func acknowledge(o opts, out io.Writer) error {
	if !decisions[o.decision] {
		return errors.New("--decision must be no-documentation-change, no-durable-node, review-rejected or inspection-limited")
	}
	if !sha12.MatchString(o.commit) || o.repo == "" || !config.ValidBranch(o.branch) {
		return errors.New("--repo, a hexadecimal --commit (12+ chars) and a valid --branch are required")
	}
	if o.date == "" {
		o.date = time.Now().UTC().Format("2006-01-02")
	}
	p := filepath.Join(o.vault, ackRel)
	doc := struct {
		Repositories []ackEntry `json:"repositories"`
		Version      int        `json:"version"`
	}{Version: 1}
	if b, e := os.ReadFile(p); e == nil {
		if e := json.Unmarshal(b, &doc); e != nil {
			return fmt.Errorf("%s: %w", ackRel, e)
		}
	}
	kept := []ackEntry{}
	for _, r := range doc.Repositories {
		if r.Repository != o.repo {
			kept = append(kept, r)
		}
	}
	kept = append(kept, ackEntry{o.date, o.commit[:12], o.branch, o.decision, o.repo})
	sort.Slice(kept, func(i, j int) bool { return kept[i].Repository < kept[j].Repository })
	doc.Repositories = kept
	b, e := json.Marshal(doc)
	if e != nil {
		return e
	}
	if e := os.WriteFile(p, append(b, '\n'), 0o644); e != nil {
		return e
	}
	return emit(out, map[string]any{"acknowledged": o.repo, "commit": o.commit[:12], "decision": o.decision, "file": ackRel, "next": "commit it on the sync branch; it goes through the same review and verify"})
}

func filesBetween(vault, from, to string) map[string]bool {
	d, _ := git(vault, "diff", "--name-only", "--no-renames", from, to)
	m := map[string]bool{}
	for _, f := range strings.Split(d, "\n") {
		if f != "" && knowledgePath.MatchString(f) {
			m[f] = true
		}
	}
	return m
}

// pull integrates the remote base without merging meaning automatically: fast-forward only;
// anything else reports the overlapping knowledge files for a rebase and a review.
func pull(o opts, out io.Writer) error {
	if _, e := git(o.vault, "fetch", "--quiet", "origin"); e != nil {
		return e
	}
	remote := "origin/" + o.base
	if _, e := git(o.vault, "rev-parse", "--verify", "--quiet", remote); e != nil {
		return fmt.Errorf("remote base %s not found", remote)
	}
	branch := current(o.vault)
	if d, _ := dirty(o.vault); len(d) > 0 {
		return fmt.Errorf("uncommitted knowledge changes %v: commit them on a sync/ branch first", d)
	}
	mb, e := git(o.vault, "merge-base", remote, "HEAD")
	if e != nil {
		return e
	}
	head, _ := git(o.vault, "rev-parse", "HEAD")
	upstream, _ := git(o.vault, "rev-parse", remote)
	res := map[string]any{"branch": branch, "base": o.base}
	switch {
	case upstream == head || upstream == mb && branch != o.base:
		res["status"] = "up-to-date"
	case branch == o.base && mb == head:
		if _, e := git(o.vault, "merge", "--ff-only", remote); e != nil {
			return e
		}
		res["status"] = "fast-forwarded"
	default:
		theirs := filesBetween(o.vault, mb, remote)
		ours := filesBetween(o.vault, mb, "HEAD")
		both := []string{}
		for f := range ours {
			if theirs[f] {
				both = append(both, f)
			}
		}
		sort.Strings(both)
		res["status"] = "diverged"
		res["changed_on_both_sides"] = both
		res["next"] = "rebase this branch onto " + remote + "; for each file changed on both sides re-apply this run's facts onto the upstream note, commit, re-run the gates and get the merged meaning reviewed"
		if branch == o.base {
			res["next"] = "move local commits to a sync/ branch (git switch -c sync/<slug>), reset " + o.base + " to " + remote + ", then rebase the sync branch and review the merged meaning"
		}
	}
	return emit(out, res)
}
