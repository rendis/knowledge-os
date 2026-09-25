package devhandoff

import (
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

	"documentation-vault/internal/config"
)

const Help = `handoff COMMAND --vault PATH [options]
  start    --package <case>/handoffs/DH-NNN.md [--worktree PATH] [--apply]
           Prepare the repository for the task: create (or reuse) its worktree and branch,
           copy the task into .handoff/, exclude .handoff/ locally, and keep the managed
           development-handoff segment in AGENTS.md (and CLAUDE.md when it does not import
           AGENTS.md). Packages naming the same repository and branch are tasks of one
           milestone and share the worktree; a task whose depends-on names a task of the same
           worktree starts after it. Without --apply it only shows the effects.
  status   [--worktree PATH]   Read-only, per worktree: its tasks with state (pending,
           blocked, in-progress, verified), commits (by Handoff trailer) and deltas, the next
           task, local changes, stale packages and the segment state; every worktree under the
           configured root when --worktree is omitted.
  refresh  --worktree PATH --handoff DH-NNN [--apply]   Replace the task with the current
           package when it changed; deltas stay.
  reconcile --worktree PATH [--id CASE] [--apply]   Import the commits and deltas since the
           last mark into the development case with a fixed mapping (see investigation --help).
All output is JSON.`

type options struct {
	vault, pkg, worktree, handoffID string
	apply                           bool
}

// Run executes a handoff command; reconcile is routed to the case package by the caller.
func Run(args []string, out io.Writer) error {
	if len(args) == 0 || contains(args, "--help") || contains(args, "-h") {
		_, e := io.WriteString(out, Help+"\n")
		return e
	}
	verb := args[0]
	var o options
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--apply":
			o.apply = true
			continue
		}
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires a value", args[i])
		}
		v := args[i+1]
		switch args[i] {
		case "--vault":
			o.vault = v
		case "--package":
			o.pkg = v
		case "--worktree":
			o.worktree = v
		case "--handoff":
			o.handoffID = v
		default:
			return fmt.Errorf("unknown option %s", args[i])
		}
		i++
	}
	if o.vault == "" {
		return errors.New("--vault is required")
	}
	r, e := config.Resolve(o.vault)
	if e != nil {
		return e
	}
	o.vault, _ = r["vault_root"].(string)
	switch verb {
	case "start":
		return start(o, out)
	case "status":
		return status(o, out)
	case "refresh":
		return refresh(o, out)
	}
	return fmt.Errorf("unknown handoff command %q", verb)
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotePath=false", "-C", dir}, args...)...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}

func worktreeRoot(vault string) (string, error) {
	w, e := config.Workspace(vault)
	if e != nil {
		return "", e
	}
	root, _ := w["worktree_root"].(string)
	if root == "" {
		return "", errors.New("no development worktree root configured: set it with onboard-developer")
	}
	return root, nil
}

// taskCopy is the worktree copy of a package: the package plus where it came from and when.
func taskCopy(vault string, p Package, baseCommit, started string) []byte {
	fm, body := frontmatter(p.Text)
	fm["package"] = relOrAbs(vault, p.Path)
	fm["package-sha256"] = p.SHA256
	fm["base-commit"] = baseCommit
	fm["started"] = started
	keys := []string{"handoff", "case", "repository", "base", "branch", "base-commit", "package", "package-sha256", "started"}
	var b strings.Builder
	b.WriteString("---\n")
	seen := map[string]bool{}
	for _, k := range keys {
		if v, ok := fm[k]; ok {
			b.WriteString(k + ": " + v + "\n")
			seen[k] = true
		}
	}
	extra := []string{}
	for k := range fm {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		b.WriteString(k + ": " + fm[k] + "\n")
	}
	b.WriteString("---\n\n" + body)
	return []byte(b.String())
}

const deltasHeader = "# Deltas\n\nChanges to the task definition, decisions, deviations, questions and verification results, in order. Format: the development-handoff section of AGENTS.md.\n"

type effect struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Detail string `json:"detail,omitempty"`
}

func start(o options, out io.Writer) error {
	if o.pkg == "" {
		return errors.New("--package is required")
	}
	pkgPath := o.pkg
	if !filepath.IsAbs(pkgPath) {
		pkgPath = filepath.Join(o.vault, pkgPath)
	}
	p, issues, e := CheckPackage(pkgPath)
	if e != nil {
		return e
	}
	if e := blocking(issues); e != nil {
		return e
	}
	loc, e := config.LocateRepository(o.vault, p.Repository)
	if e != nil {
		return e
	}
	repo, _ := loc["path"].(string)
	if loc["status"] != "ok" {
		return fmt.Errorf("repository %s is not in the configured roots (%v): clone it through onboard-developer", p.Repository, loc["status"])
	}
	dest := o.worktree
	if dest == "" {
		root, e := worktreeRoot(o.vault)
		if e != nil {
			return e
		}
		parts := strings.Split(p.Branch, "/")
		dest = filepath.Join(root, filepath.Base(repo), parts[len(parts)-1])
	}
	effects := []effect{}
	baseCommit := ""
	var createArgs []string
	if fi, e := os.Stat(dest); e == nil && fi.IsDir() {
		// Reuse: the directory must be a worktree of this repository on the task's branch.
		common, e1 := git(dest, "rev-parse", "--path-format=absolute", "--git-common-dir")
		mine, e2 := git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
		branch, _ := git(dest, "rev-parse", "--abbrev-ref", "HEAD")
		if e1 != nil || e2 != nil || common != mine {
			return fmt.Errorf("%s exists but is not a worktree of %s", dest, filepath.Base(repo))
		}
		if branch != p.Branch {
			return fmt.Errorf("%s is on branch %s, the task expects %s: pass the task's worktree or a new --worktree", dest, branch, p.Branch)
		}
		baseCommit, _ = git(dest, "merge-base", "HEAD", baseRef(repo, p.Base))
		effects = append(effects, effect{"reuse-worktree", dest, "branch " + branch})
	} else {
		ref := baseRef(repo, p.Base)
		if ref == "" {
			return fmt.Errorf("base %s is not in %s: fetch it first", p.Base, filepath.Base(repo))
		}
		baseCommit, _ = git(repo, "rev-parse", ref)
		if _, e := git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+p.Branch); e == nil {
			createArgs = []string{"worktree", "add", dest, p.Branch}
			effects = append(effects, effect{"create-worktree", dest, "existing branch " + p.Branch})
		} else {
			createArgs = []string{"worktree", "add", "-b", p.Branch, dest, baseCommit}
			effects = append(effects, effect{"create-worktree", dest, "new branch " + p.Branch + " from " + ref + " (" + baseCommit[:12] + ")"})
		}
	}
	// A task that builds on another task of the same worktree starts after it; one in another
	// repository or branch is a prerequisite the package itself must describe.
	for _, dep := range p.DependsOn {
		dp, e := ReadPackage(filepath.Join(filepath.Dir(pkgPath), dep+".md"))
		if e != nil {
			return fmt.Errorf("%s depends on %s, which is not a package of this case", p.Handoff, dep)
		}
		if dp.Repository == p.Repository && dp.Branch == p.Branch {
			if _, e := os.Stat(filepath.Join(dest, ".handoff", dep+".md")); e != nil {
				return fmt.Errorf("%s builds on %s in the same worktree: start %s first", p.Handoff, dep, dep)
			}
			effects = append(effects, effect{"depends-on", dep, "same worktree, already started"})
		} else {
			effects = append(effects, effect{"prerequisite", dep, dp.Repository + " on " + dp.Branch + ": the package states what this task needs from it"})
		}
	}
	taskRel := filepath.Join(".handoff", p.Handoff+".md")
	if b, e := os.ReadFile(filepath.Join(dest, taskRel)); e == nil {
		fm, _ := frontmatter(string(b))
		if fm["package-sha256"] == p.SHA256 {
			effects = append(effects, effect{"unchanged", taskRel, "task already at this package revision"})
		} else {
			return fmt.Errorf("%s already holds %s from another package revision: use refresh", dest, p.Handoff)
		}
	} else {
		effects = append(effects, effect{"write", taskRel, p.Title})
	}
	effects = append(effects, effect{"write-if-missing", ".handoff/deltas.md", ""}, effect{"exclude", "info/exclude", ".handoff/"})
	for _, f := range instructionFiles(dest) {
		if st := segmentStatus(f); st != "current" {
			effects = append(effects, effect{"managed-segment", filepath.Base(f), st + " → current (tracked file: commit it with the repository)"})
		}
	}
	res := map[string]any{"handoff": p.Handoff, "title": p.Title, "repository": filepath.Base(repo), "worktree": dest, "branch": p.Branch, "base_commit": baseCommit, "effects": effects, "applied": o.apply}
	if !o.apply {
		res["next"] = "review the effects with the user, then repeat with --apply"
		return emit(out, res)
	}
	if createArgs != nil {
		if e := os.MkdirAll(filepath.Dir(dest), 0o755); e != nil {
			return e
		}
		if _, e := git(repo, createArgs...); e != nil {
			return e
		}
	}
	if e := os.MkdirAll(filepath.Join(dest, ".handoff"), 0o755); e != nil {
		return e
	}
	if _, e := os.Stat(filepath.Join(dest, taskRel)); os.IsNotExist(e) {
		if e := os.WriteFile(filepath.Join(dest, taskRel), taskCopy(o.vault, p, baseCommit, time.Now().Format("2006-01-02")), 0o644); e != nil {
			return e
		}
	}
	if _, e := os.Stat(filepath.Join(dest, ".handoff", "deltas.md")); os.IsNotExist(e) {
		if e := os.WriteFile(filepath.Join(dest, ".handoff", "deltas.md"), []byte(deltasHeader), 0o644); e != nil {
			return e
		}
	}
	if e := excludeLocally(dest); e != nil {
		return e
	}
	for _, f := range instructionFiles(dest) {
		next, changed, e := withSegment(f)
		if e != nil {
			return e
		}
		if changed {
			if e := os.WriteFile(f, next, 0o644); e != nil {
				return e
			}
		}
	}
	res["next"] = "open a new agent session rooted at " + dest + " (or work there with work-in-repository); record " + p.Handoff + " in the case"
	return emit(out, res)
}

// baseRef prefers the remote-tracking base (the local branch may lag).
func baseRef(repo, base string) string {
	for _, r := range []string{"refs/remotes/origin/" + base, "refs/heads/" + base, base} {
		if _, e := git(repo, "rev-parse", "--verify", "--quiet", r+"^{commit}"); e == nil {
			return r
		}
	}
	return ""
}

// excludeLocally adds .handoff/ to the repository's shared info/exclude, never to a tracked file.
func excludeLocally(worktree string) error {
	common, e := git(worktree, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return e
	}
	p := filepath.Join(common, "info", "exclude")
	b, _ := os.ReadFile(p)
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == ".handoff/" || strings.TrimSpace(l) == "/.handoff/" {
			return nil
		}
	}
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return os.WriteFile(p, []byte(s+".handoff/\n"), 0o644)
}

var deltaHeading = regexp.MustCompile(`(?m)^##\s+(DELTA-\d{3,})\s+—\s+(.+)$`)
var deltaField = regexp.MustCompile(`(?m)^-\s+(Handoff|Type|Tipo|Detail|Detalle|Evidence|Evidencia):\s*(.+)$`)

// Delta is one entry of a worktree's deltas.md.
type Delta struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Handoff  string `json:"handoff,omitempty"`
	Type     string `json:"type,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

func readDeltas(path string) []Delta {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	text := string(b)
	idx := deltaHeading.FindAllStringSubmatchIndex(text, -1)
	out := []Delta{}
	for i, m := range idx {
		end := len(text)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		d := Delta{ID: text[m[2]:m[3]], Title: strings.TrimSpace(text[m[4]:m[5]])}
		for _, f := range deltaField.FindAllStringSubmatch(text[m[1]:end], -1) {
			v := strings.TrimSpace(f[2])
			switch f[1] {
			case "Handoff":
				d.Handoff = v
			case "Type", "Tipo":
				d.Type = strings.ToLower(v)
			case "Detail", "Detalle":
				d.Detail = v
			default:
				d.Evidence = v
			}
		}
		out = append(out, d)
	}
	return out
}

// Deltas returns the deltas recorded in a worktree, in order.
func Deltas(worktree string) []Delta {
	return readDeltas(filepath.Join(worktree, ".handoff", "deltas.md"))
}

// Tasks returns the task copies of a worktree and, per task, the commits since the base that belong to
// it ("<short sha> <subject>", newest first).
func Tasks(worktree string) ([]Package, map[string][]string) {
	files, _ := filepath.Glob(filepath.Join(worktree, ".handoff", "DH-*.md"))
	sort.Strings(files)
	pkgs, tasks, base := []Package{}, []task{}, ""
	for _, f := range files {
		p, e := ReadPackage(f)
		if e != nil {
			continue
		}
		pkgs = append(pkgs, p)
		tasks = append(tasks, task{Handoff: p.Handoff})
		if base == "" {
			base = p.Fields["base-commit"]
		}
	}
	by, _ := commitsByTask(worktree, base, tasks)
	return pkgs, by
}

type task struct {
	Handoff      string   `json:"handoff"`
	Title        string   `json:"title"`
	Case         string   `json:"case"`
	Package      string   `json:"package"`
	PackageStale bool     `json:"package_changed,omitempty"`
	BaseCommit   string   `json:"base_commit"`
	Started      string   `json:"started"`
	DependsOn    []string `json:"depends_on,omitempty"`
	State        string   `json:"state"` // pending | blocked | in-progress | verified
	Commits      []string `json:"commits"`
	Deltas       []string `json:"deltas"`
}

var trailer = regexp.MustCompile(`(?mi)^Handoff:\s*(.+)$`)

// commitsByTask attributes the branch's commits to tasks through their Handoff trailers; with a single
// task every commit is its own.
func commitsByTask(dir, base string, tasks []task) (map[string][]string, []string) {
	by, loose := map[string][]string{}, []string{}
	if base == "" {
		return by, loose
	}
	out, e := git(dir, "log", "--format=%h %s%x00%B%x1e", base+"..HEAD")
	if e != nil || out == "" {
		return by, loose
	}
	for _, rec := range strings.Split(out, "\x1e") {
		head, msg, _ := strings.Cut(strings.TrimSpace(rec), "\x00")
		if head == "" {
			continue
		}
		named := false
		for _, m := range trailer.FindAllStringSubmatch(msg, -1) {
			for _, id := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == ' ' }) {
				by[id] = append(by[id], head)
				named = true
			}
		}
		if !named {
			if len(tasks) == 1 {
				by[tasks[0].Handoff] = append(by[tasks[0].Handoff], head)
			} else {
				loose = append(loose, head)
			}
		}
	}
	return by, loose
}

func worktreeStatus(vault, dir string) map[string]any {
	res := map[string]any{"worktree": dir}
	res["branch"], _ = git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	tasks := []task{}
	base := ""
	files, _ := filepath.Glob(filepath.Join(dir, ".handoff", "DH-*.md"))
	sort.Strings(files)
	for _, f := range files {
		p, e := ReadPackage(f)
		if e != nil {
			continue
		}
		t := task{Handoff: p.Handoff, Title: p.Title, Case: p.Case, Package: p.Fields["package"], BaseCommit: p.Fields["base-commit"], Started: p.Fields["started"], DependsOn: p.DependsOn}
		if t.Package != "" {
			src := t.Package
			if !filepath.IsAbs(src) {
				src = filepath.Join(vault, src)
			}
			if b, e := os.ReadFile(src); e == nil && digest(b) != p.Fields["package-sha256"] {
				t.PackageStale = true
			}
		}
		if base == "" {
			base = t.BaseCommit
		}
		tasks = append(tasks, t)
	}
	if base != "" {
		if log, e := git(dir, "log", "--format=%h %s", base+"..HEAD"); e == nil && log != "" {
			res["commits_since_base"] = strings.Split(log, "\n")
		} else {
			res["commits_since_base"] = []string{}
		}
	}
	deltas := readDeltas(filepath.Join(dir, ".handoff", "deltas.md"))
	res["deltas"] = deltas
	byTask, loose := commitsByTask(dir, base, tasks)
	verified := map[string]bool{}
	for i := range tasks {
		t := &tasks[i]
		t.Commits, t.Deltas = append([]string{}, byTask[t.Handoff]...), []string{}
		for _, d := range deltas {
			if d.Handoff == t.Handoff {
				t.Deltas = append(t.Deltas, d.ID)
				if strings.EqualFold(d.Type, "verification") {
					verified[t.Handoff] = true
				}
			}
		}
	}
	started := map[string]bool{}
	for _, t := range tasks {
		started[t.Handoff] = true
	}
	next := ""
	for i := range tasks {
		t := &tasks[i]
		switch {
		case verified[t.Handoff]:
			t.State = "verified"
		case len(t.Commits) > 0 || len(t.Deltas) > 0:
			t.State = "in-progress"
		default:
			t.State = "pending"
			for _, d := range t.DependsOn {
				if started[d] && !verified[d] {
					t.State = "blocked"
				}
			}
		}
		if next == "" && (t.State == "pending" || t.State == "in-progress") {
			next = t.Handoff
		}
	}
	res["handoffs"] = tasks
	res["next_task"] = next
	if len(loose) > 0 {
		res["commits_without_task"] = loose
	}
	if st, e := git(dir, "status", "--porcelain"); e == nil {
		n := 0
		for _, l := range strings.Split(st, "\n") {
			if l != "" && !strings.Contains(l, ".handoff/") {
				n++
			}
		}
		res["uncommitted_changes"] = n
	}
	seg := map[string]string{}
	for _, f := range instructionFiles(dir) {
		seg[filepath.Base(f)] = segmentStatus(f)
	}
	res["managed_segment"] = seg
	return res
}

func status(o options, out io.Writer) error {
	if o.worktree != "" {
		return emit(out, worktreeStatus(o.vault, o.worktree))
	}
	root, e := worktreeRoot(o.vault)
	if e != nil {
		return e
	}
	found := []map[string]any{}
	repos, _ := os.ReadDir(root)
	for _, r := range repos {
		if !r.IsDir() {
			continue
		}
		dirs, _ := os.ReadDir(filepath.Join(root, r.Name()))
		for _, d := range dirs {
			dir := filepath.Join(root, r.Name(), d.Name())
			if _, e := os.Stat(filepath.Join(dir, ".handoff")); e == nil {
				found = append(found, worktreeStatus(o.vault, dir))
			}
		}
	}
	return emit(out, map[string]any{"worktree_root": root, "worktrees": found, "count": len(found)})
}

func refresh(o options, out io.Writer) error {
	if o.worktree == "" || o.handoffID == "" {
		return errors.New("--worktree and --handoff are required")
	}
	taskPath := filepath.Join(o.worktree, ".handoff", o.handoffID+".md")
	cur, e := ReadPackage(taskPath)
	if e != nil {
		return fmt.Errorf("no task %s in %s", o.handoffID, o.worktree)
	}
	src := cur.Fields["package"]
	if src == "" {
		return errors.New("the task does not record its package")
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(o.vault, src)
	}
	p, issues, e := CheckPackage(src)
	if e != nil {
		return e
	}
	if e := blocking(issues); e != nil {
		return e
	}
	res := map[string]any{"handoff": o.handoffID, "worktree": o.worktree, "applied": o.apply}
	if p.SHA256 == cur.Fields["package-sha256"] {
		res["effects"] = []effect{{"unchanged", taskPath, "package unchanged"}}
		return emit(out, res)
	}
	if p.Branch != cur.Fields["branch"] || p.Repository != cur.Fields["repository"] {
		return errors.New("the package now names another repository or branch: start a new handoff instead")
	}
	changed := []string{}
	_, oldBody := frontmatter(cur.Text)
	_, newBody := frontmatter(p.Text)
	ob, nb := sectionBodies(oldBody), sectionBodies(newBody)
	for _, s := range sections {
		if ob[s.key] != nb[s.key] {
			changed = append(changed, s.names[0])
		}
	}
	res["effects"] = []effect{{"replace", filepath.Join(".handoff", o.handoffID+".md"), "changed sections: " + strings.Join(changed, ", ")}}
	if !o.apply {
		res["next"] = "review with the user, then repeat with --apply; record a delta if work already done depends on the old definition"
		return emit(out, res)
	}
	if e := os.WriteFile(taskPath, taskCopy(o.vault, p, cur.Fields["base-commit"], cur.Fields["started"]), 0o644); e != nil {
		return e
	}
	return emit(out, res)
}

func contains(a []string, s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
