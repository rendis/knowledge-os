// Package gitsync publishes knowledge through Git: a local branch is the synchronization run,
// commits are its checkpoints, a review commit binds the accepted content by digest, and a
// fast-forward merge publishes it. No other run state exists.
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

	"documentation-vault/internal/audit"
	"documentation-vault/internal/check"
	"documentation-vault/internal/config"
	"documentation-vault/internal/discover"
)

const Help = `sync COMMAND --vault PATH [options]
  start   --name SLUG [--base BRANCH]   Create and switch to branch sync/SLUG from the base.
  status                                Branch, base, changed knowledge files and review state.
  review  --verdict accept|revise --reviewer NAME [--summary TEXT]
                                        Record the reviewer's verdict on the committed content
                                        as a commit whose trailers bind it by digest.
  verify  [--base BRANCH]               Gates for the branch: every changed repository note passes
                                        discover check, no new audit/link/Bases issue versus the
                                        base, clean tree, and an accepted review of the final content.
  acknowledge --repo NAME --commit SHA --decision D [--branch main] [--date YYYY-MM-DD]
                                        Record a reviewed repository commit that needs no note change
                                        (D: no-documentation-change | no-durable-node |
                                        review-rejected | inspection-limited).
  finish  [--base BRANCH]               verify, then fast-forward the base and delete the branch.
Publishing to the remote follows the repository's Git policy. All output is JSON.`

const ackRel = "90-Meta/.sync-acknowledgements.json"

var (
	knowledgePath = regexp.MustCompile(`^([1-7]\d-[^/]+/.+\.md|00-Home\.md|[^/]+\.base|90-Meta/\.sync-acknowledgements\.json)$`)
	slugRE        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,80}$`)
	sha12         = regexp.MustCompile(`^[0-9a-f]{12,40}$`)
	decisions     = map[string]bool{"no-documentation-change": true, "no-durable-node": true, "review-rejected": true, "inspection-limited": true}
)

type opts struct {
	vault, name, base, verdict, reviewer, summary, repo, commit, decision, branch, date string
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
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
		o.base = defaultBase(o.vault)
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
	}
	return fmt.Errorf("unknown sync command %q; the legacy run commands were retired, see synchronize-ecosystem", cmd)
}

func defaultBase(vault string) string {
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
	files, _, e := changed(o)
	if e != nil {
		return e
	}
	if len(files) == 0 {
		return errors.New("nothing to review: no knowledge change on this branch")
	}
	d := digest(o.vault, files)
	msg := fmt.Sprintf("chore(sync): record review %s\n\n%s\n\nKnowledge-Review: %s\nKnowledge-Reviewer: %s\nKnowledge-Digest: %s\n", o.verdict, strings.TrimSpace(o.summary), o.verdict, o.reviewer, d)
	if _, e := git(o.vault, "commit", "--allow-empty", "-q", "-m", msg); e != nil {
		return e
	}
	return emit(out, map[string]any{"recorded": o.verdict, "files": files, "digest": d})
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
		ok, _, e := discover.CheckNote(o.vault, f)
		if e != nil {
			return nil, e
		}
		notes = append(notes, map[string]any{"note": f, "ok": ok})
		if !ok {
			problems = append(problems, "note gates failed: "+f+" (run `discover check --note "+f+"`)")
		}
	}
	res["note_gates"] = notes
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
	switch {
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
