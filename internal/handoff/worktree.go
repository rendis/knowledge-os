package handoff

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"documentation-vault/internal/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type WorktreeOptions struct{ Vault, Bundle, BaseBranch, BaseSource, Description, BranchPrefix string }
type WorktreePlan struct {
	Status    string            `json:"status"`
	Operation string            `json:"operation"`
	Source    map[string]string `json:"source"`
	Base      map[string]any    `json:"base"`
	Worktree  map[string]string `json:"worktree"`
	Effects   []map[string]any  `json:"effects"`
	Token     string            `json:"plan_token"`
}
type bundle struct {
	fingerprint, remote string
	metadata            map[string]any
	work                map[string]any
	documents           map[string][]byte
}

func worktreeRun(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o WorktreeOptions
	var token string
	fs.StringVar(&o.Vault, "vault-root", "", "vault root")
	fs.StringVar(&o.Vault, "vault", "", "vault root")
	fs.StringVar(&o.Bundle, "bundle", "", "exported bundle")
	fs.StringVar(&o.BaseBranch, "base-branch", "", "selected base branch")
	fs.StringVar(&o.BaseSource, "base-source", "", "local or remote")
	fs.StringVar(&o.Description, "description", "", "branch description")
	fs.StringVar(&o.BranchPrefix, "branch-prefix", "issue", "branch prefix")
	fs.StringVar(&token, "plan-token", "", "approved plan")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	var p any
	var err error
	if args[0] == "plan-handoff" {
		p, err = PlanComplete(o)
	} else if args[0] == "prepare-handoff" {
		if token == "" {
			return errors.New("--plan-token is required")
		}
		p, err = PrepareComplete(o, token)
	} else if args[0] == "create-worktree" {
		if token == "" {
			return errors.New("--plan-token is required")
		}
		p, err = CreateWorktree(o, token)
	} else {
		p, err = PlanWorktree(o)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(p)
}

func textField(m map[string]any, key string, max int) (string, error) {
	s, ok := m[key].(string)
	s = strings.TrimSpace(s)
	if !ok || s == "" || utf8.RuneCountInString(s) > max || strings.ContainsRune(s, 0) {
		return "", fmt.Errorf("invalid %s", key)
	}
	return s, nil
}
func exactFields(m map[string]any, required string, optional string) error {
	allowed := map[string]bool{}
	for _, key := range strings.Fields(required) {
		allowed[key] = true
		if _, ok := m[key]; !ok {
			return fmt.Errorf("missing %s", key)
		}
	}
	for _, key := range strings.Fields(optional) {
		allowed[key] = true
	}
	for key := range m {
		if !allowed[key] {
			return fmt.Errorf("unknown field %s", key)
		}
	}
	return nil
}
func mapping(m map[string]any, key string) (map[string]any, error) {
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a mapping", key)
	}
	return v, nil
}
func timestampField(m map[string]any, key string) error {
	s, e := textField(m, key, 64)
	if e != nil {
		return e
	}
	if !updateTimestamp(s) {
		return errors.New("timestamp requires ISO-8601 time and offset")
	}
	return nil
}
func trackerURL(s string) (string, error) {
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid credential-free tracker URL")
	}
	return "https://" + strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/"), nil
}

var probableSecret = regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}\b|\bgithub_pat_[A-Za-z0-9_]{20,}\b|\bglpat-[A-Za-z0-9_-]{20,}\b|\bxox[baprs]-[A-Za-z0-9-]{20,}\b|\bAKIA[0-9A-Z]{16}\b|\bAIza[0-9A-Za-z_-]{30,}\b|https?://[^/\s:@]+:[^/\s@]+@`)
var assignedSecret = regexp.MustCompile(`(?i)\b(password|passwd|pwd|client[_-]?secret|api[_-]?key|access[_-]?token)\b\s*[:=]\s*["']?([^\s"'#,;]+)`)

func scanSecrets(raw []byte) error {
	if probableSecret.Match(raw) {
		return errors.New("bundle contains a probable secret")
	}
	safe := map[string]bool{"redacted": true, "<redacted>": true, "[redacted]": true, "***": true, "masked": true, "placeholder": true, "example": true, "none": true, "null": true}
	for _, match := range assignedSecret.FindAllSubmatch(raw, -1) {
		s := strings.ToLower(string(match[2]))
		if !safe[s] && !strings.HasPrefix(s, "${") && !strings.HasPrefix(s, "{{") {
			return errors.New("bundle contains a probable assigned secret")
		}
	}
	return nil
}

func loadBundle(path string) (bundle, error) {
	if err := safeDirectory(path); err != nil {
		return bundle{}, err
	}
	raw, err := readFile(filepath.Join(path, "bundle.yaml"))
	if err != nil {
		return bundle{}, err
	}
	var m map[string]any
	if err = decode(raw, &m); err != nil {
		return bundle{}, err
	}
	if err = exactFields(m, "schema-version source work-item repository change", ""); err != nil {
		return bundle{}, err
	}
	if m["schema-version"] != 2 {
		return bundle{}, errors.New("bundle schema must be 2")
	}
	source, err := mapping(m, "source")
	if err != nil {
		return bundle{}, err
	}
	if err = exactFields(source, "investigation-id investigation-updated-at story-id", ""); err != nil {
		return bundle{}, err
	}
	if _, err = textField(source, "investigation-id", 200); err != nil {
		return bundle{}, err
	}
	if err = timestampField(source, "investigation-updated-at"); err != nil {
		return bundle{}, err
	}
	story, err := textField(source, "story-id", 32)
	if err != nil || !regexp.MustCompile(`^S-[0-9]{3,}$`).MatchString(story) {
		return bundle{}, errors.New("invalid story-id")
	}
	work, err := mapping(m, "work-item")
	if err != nil {
		return bundle{}, err
	}
	if err = exactFields(work, "tracker-id provider tracker-url reference url updated-at captured-at freshness snapshot-source", ""); err != nil {
		return bundle{}, err
	}
	for _, key := range []string{"tracker-id", "provider"} {
		s, e := textField(work, key, 64)
		if e != nil || !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(s) {
			return bundle{}, fmt.Errorf("invalid %s", key)
		}
	}
	reference, err := textField(work, "reference", 200)
	if err != nil {
		return bundle{}, err
	}
	tracker, err := textField(work, "tracker-url", 512)
	if err != nil {
		return bundle{}, err
	}
	tracker, err = trackerURL(tracker)
	if err != nil {
		return bundle{}, err
	}
	work["tracker-url"] = tracker
	itemURL, err := textField(work, "url", 1024)
	if err != nil {
		return bundle{}, err
	}
	u, e := url.Parse(itemURL)
	base, _ := url.Parse(tracker)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || !strings.EqualFold(u.Hostname(), base.Hostname()) || effectivePort(u) != effectivePort(base) {
		return bundle{}, errors.New("work-item URL differs from tracker host")
	}
	for _, key := range []string{"updated-at", "captured-at"} {
		if err = timestampField(work, key); err != nil {
			return bundle{}, err
		}
	}
	if work["freshness"] != "current" {
		return bundle{}, errors.New("work-item snapshot is not current")
	}
	if work["snapshot-source"] != "connected-readback" && work["snapshot-source"] != "user-supplied-export" {
		return bundle{}, errors.New("unsupported work-item snapshot provenance")
	}
	repo, err := mapping(m, "repository")
	if err != nil {
		return bundle{}, err
	}
	if err = exactFields(repo, "remote", ""); err != nil {
		return bundle{}, err
	}
	remote, err := textField(repo, "remote", 1024)
	if err != nil {
		return bundle{}, err
	}
	if _, err = config.RemoteIdentity(remote); err != nil {
		return bundle{}, err
	}
	change, err := mapping(m, "change")
	if err != nil {
		return bundle{}, err
	}
	if err = exactFields(change, "summary", "reasons"); err != nil {
		return bundle{}, err
	}
	if _, err = textField(change, "summary", 500); err != nil {
		return bundle{}, err
	}
	if change["reasons"] != nil {
		reasons, e := mapping(change, "reasons")
		if e != nil {
			return bundle{}, e
		}
		if e = exactFields(reasons, "", "context.md work-item.md scope.md"); e != nil {
			return bundle{}, e
		}
		for key := range reasons {
			if _, e = textField(reasons, key, 500); e != nil {
				return bundle{}, e
			}
		}
	}
	total := len(raw)
	docs := map[string]string{}
	documents := map[string][]byte{}
	for _, name := range []string{"context.md", "work-item.md", "scope.md"} {
		data, e := readFile(filepath.Join(path, name))
		if e != nil {
			return bundle{}, e
		}
		if len(data) == 0 || len(data) > maxFileBytes || !utf8.Valid(data) {
			return bundle{}, fmt.Errorf("invalid UTF-8 document: %s", name)
		}
		if e = scanSecrets(data); e != nil {
			return bundle{}, fmt.Errorf("%s: %w", name, e)
		}
		if name == "work-item.md" && !strings.Contains(strings.ToLower(string(data)), strings.ToLower(reference)) {
			return bundle{}, errors.New("work-item document does not identify work item")
		}
		total += len(data)
		docs[name] = digest(data)
		documents[name] = data
	}
	if total > 3*maxFileBytes {
		return bundle{}, errors.New("bundle exceeds total byte limit")
	}
	payload, _ := json.Marshal(map[string]any{"metadata": digest(raw), "documents": docs})
	return bundle{fingerprint: digest(payload), remote: remote, metadata: m, work: work, documents: documents}, nil
}
func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	return "443"
}

func validateStoredWorkItem(work map[string]any) error {
	if err := exactFields(work, "tracker-id provider tracker-url reference url updated-at captured-at freshness snapshot-source", ""); err != nil {
		return err
	}
	for _, key := range []string{"tracker-id", "provider"} {
		value, err := textField(work, key, 64)
		if err != nil || !regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(value) {
			return fmt.Errorf("invalid stored %s", key)
		}
	}
	if _, err := textField(work, "reference", 200); err != nil {
		return err
	}
	base, err := textField(work, "tracker-url", 512)
	if err != nil {
		return err
	}
	base, err = trackerURL(base)
	if err != nil {
		return err
	}
	raw, err := textField(work, "url", 1024)
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	t, _ := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || !strings.EqualFold(u.Hostname(), t.Hostname()) || effectivePort(u) != effectivePort(t) {
		return errors.New("invalid stored work-item URL")
	}
	return nil
}
func slug(s string) string {
	s = norm.NFKD.String(cases.Fold().String(s))
	s = strings.Map(func(r rune) rune {
		if norm.NFKD.PropertiesString(string(r)).CCC() != 0 {
			return -1
		}
		return r
	}, s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func git(ctx context.Context, root string, args ...string) (string, error) {
	all := []string{"--no-optional-locks", "-C", root}
	all = append(all, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return strings.TrimSpace(string(data)), nil
}
func gitCall(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return git(ctx, root, args...)
}
func commit(root, ref string) string {
	s, err := gitCall(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return ""
	}
	return s
}
func nonemptyOption(s string, max int) bool {
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= max && !strings.ContainsRune(s, 0)
}

func PlanWorktree(o WorktreeOptions) (WorktreePlan, error) {
	p := WorktreePlan{}
	vault, err := config.CanonicalRoot(o.Vault)
	if err != nil {
		return p, err
	}
	b, err := loadBundle(o.Bundle)
	if err != nil {
		return p, err
	}
	instance, err := config.LoadInstance(vault)
	if err != nil {
		return p, err
	}
	trackers, _ := instance["trackers"].([]any)
	matches := 0
	for _, item := range trackers {
		t, _ := item.(map[string]any)
		if t["id"] == b.work["tracker-id"] {
			matches++
			s, _ := t["url"].(string)
			normalized, e := trackerURL(s)
			if e != nil || normalized != b.work["tracker-url"] || t["provider"] != b.work["provider"] {
				return p, errors.New("tracker binding mismatch")
			}
		}
	}
	if matches != 1 {
		return p, errors.New("work item must match exactly one configured tracker")
	}
	located, err := config.LocateRepository(vault, b.remote)
	if err != nil {
		return p, err
	}
	source, _ := located["path"].(string)
	remote, _ := located["remote"].(string)
	if located["status"] != "ok" || source == "" {
		return p, errors.New("repository did not resolve uniquely")
	}
	actualRoot, err := gitCall(source, "rev-parse", "--show-toplevel")
	if err != nil {
		return p, err
	}
	actualRoot, err = filepath.EvalSymlinks(actualRoot)
	if err != nil || actualRoot != source {
		return p, errors.New("configured repository must be exact Git root")
	}
	workspace, err := config.Workspace(vault)
	if err != nil {
		return p, err
	}
	worktreeRoot, _ := workspace["worktree_root"].(string)
	if !filepath.IsAbs(worktreeRoot) {
		return p, errors.New("absolute worktree root must be configured")
	}
	worktreeRoot, err = filepath.EvalSymlinks(worktreeRoot)
	if err != nil {
		return p, err
	}
	if err = safeDirectory(worktreeRoot); err != nil {
		return p, err
	}
	if !nonemptyOption(o.BaseBranch, 200) || !nonemptyOption(o.Description, 120) || !nonemptyOption(o.BranchPrefix, 100) || (o.BaseSource != "local" && o.BaseSource != "remote") {
		return p, errors.New("base branch, base source, description and branch prefix are required")
	}
	if _, err = gitCall(source, "check-ref-format", "--branch", o.BaseBranch); err != nil {
		return p, errors.New("invalid base branch")
	}
	prefix := strings.TrimRight(o.BranchPrefix, "/")
	if strings.HasPrefix(prefix, "/") || prefix == "" {
		return p, errors.New("invalid branch prefix")
	}
	id := digest([]byte(fmt.Sprintf("%s|%s|%s", b.work["tracker-id"], b.work["reference"], remote)))
	readable := slug(fmt.Sprintf("%s-%s", b.work["tracker-id"], b.work["reference"]))
	if readable == "" || slug(o.Description) == "" {
		return p, errors.New("description and identity must produce canonical names")
	}
	if len(readable) > 80 {
		readable = strings.TrimRight(readable[:80], "-")
	}
	token := readable + "-" + id[:10]
	leaf := token + "-" + slug(o.Description)
	branch := prefix + "/" + leaf
	if _, err = gitCall(source, "check-ref-format", "--branch", branch); err != nil {
		return p, errors.New("invalid generated branch")
	}
	remotes, err := gitCall(source, "remote")
	if err != nil {
		return p, err
	}
	remoteName, remoteURL := "", ""
	count := 0
	for _, name := range strings.Fields(remotes) {
		value, e := gitCall(source, "config", "--get", "remote."+name+".url")
		if e != nil {
			continue
		}
		identity, e := config.RemoteIdentity(value)
		if e == nil && identity == remote {
			count++
			if name == "origin" || remoteName != "origin" {
				remoteName, remoteURL = name, value
			}
		}
	}
	if count == 0 || (count > 1 && remoteName != "origin") || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`).MatchString(remoteName) {
		return p, errors.New("missing, unsafe or ambiguous matching Git remote")
	}
	repoName := remoteURL
	if strings.Contains(repoName, "://") {
		u, _ := url.Parse(repoName)
		repoName = u.Path
	}
	repoName = strings.TrimRight(repoName, "/")
	repoName = repoName[strings.LastIndexAny(repoName, "/:")+1:]
	if strings.HasSuffix(strings.ToLower(repoName), ".git") {
		repoName = repoName[:len(repoName)-4]
	}
	if repoName == "." || repoName == ".." || !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(repoName) {
		return p, errors.New("unsafe repository basename")
	}
	parent := filepath.Join(worktreeRoot, repoName)
	path := filepath.Join(parent, leaf)
	parentState := "absent"
	if info, e := os.Lstat(parent); e == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return p, errors.New("unsafe worktree parent")
		}
		if _, e = gitCall(parent, "rev-parse", "--is-inside-work-tree"); e == nil {
			return p, errors.New("worktree parent is inside Git checkout")
		}
		parentState = "directory"
	} else if !os.IsNotExist(e) {
		return p, e
	}
	if _, e := os.Lstat(path); e == nil {
		return p, errors.New("worktree path already exists")
	} else if !os.IsNotExist(e) {
		return p, e
	}
	if commit(source, "refs/heads/"+branch) != "" {
		return p, errors.New("worktree branch already exists")
	}
	listed, err := gitCall(source, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return p, err
	}
	for _, line := range strings.Split(listed, "\x00") {
		if strings.HasPrefix(line, "worktree ") && filepath.Clean(strings.TrimPrefix(line, "worktree ")) == path {
			return p, errors.New("worktree path already registered")
		}
	}
	local := commit(source, "refs/heads/"+o.BaseBranch)
	if local == "" && o.BaseSource == "local" {
		return p, errors.New("selected local base branch is missing")
	}
	observation, err := gitCall(source, "ls-remote", "--exit-code", remoteName, "refs/heads/"+o.BaseBranch)
	if err != nil {
		return p, errors.New("remote base could not be verified")
	}
	fields := strings.Fields(observation)
	if len(fields) != 2 || fields[1] != "refs/heads/"+o.BaseBranch || !regexp.MustCompile(`^[a-fA-F0-9]{40,64}$`).MatchString(fields[0]) {
		return p, errors.New("remote base did not resolve to one commit")
	}
	remoteCommit := strings.ToLower(fields[0])
	selected := local
	if o.BaseSource == "remote" {
		selected = remoteCommit
	}
	trackingRef := "refs/remotes/" + remoteName + "/" + o.BaseBranch
	tracking := commit(source, trackingRef)
	fetch := o.BaseSource == "remote" && commit(source, remoteCommit) != remoteCommit
	update := o.BaseSource == "remote" && tracking != remoteCommit
	freshness := "current"
	if local == "" {
		freshness = "local_missing"
	} else if local != remoteCommit {
		freshness = "local_differs_from_remote"
	}
	effects := []map[string]any{}
	add := func(path, action string) {
		effects = append(effects, map[string]any{"path": path, "action": action, "tracked": false})
	}
	if fetch {
		add("git-object:"+remoteCommit, "fetch")
	}
	if update {
		action := "update"
		if tracking == "" {
			action = "create"
		}
		add(trackingRef, action)
	}
	if parentState == "absent" {
		add(parent, "create-directory")
	}
	add("refs/heads/"+branch, "create")
	add(path, "create-worktree")
	p = WorktreePlan{Status: "planned", Operation: "create-worktree", Source: map[string]string{"path": source, "remote": remote, "remote_name": remoteName}, Base: map[string]any{"branch": o.BaseBranch, "local_commit": nullableCommit(local), "remote_commit": remoteCommit, "freshness": freshness, "selected_source": o.BaseSource, "selected_commit": selected, "fetch_required": fetch, "tracking_update_required": update, "remote_tracking_commit": nullableCommit(tracking)}, Worktree: map[string]string{"root": worktreeRoot, "repository_directory": parent, "path": path, "branch": branch}, Effects: effects}
	payload, _ := json.Marshal(map[string]any{"schema_version": 1, "plan": p, "bundle": b.fingerprint, "parent_state": parentState})
	p.Token = digest(payload)
	return p, nil
}
func nullableCommit(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func CreateWorktree(o WorktreeOptions, approved string) (WorktreePlan, error) {
	p, err := PlanWorktree(o)
	if err != nil {
		return p, err
	}
	if subtle.ConstantTimeCompare([]byte(p.Token), []byte(approved)) != 1 {
		return p, errors.New("plan_stale: source, bundle, remote base or destination changed")
	}
	source := p.Source["path"]
	selected := p.Base["selected_commit"].(string)
	if p.Base["fetch_required"] == true {
		if _, err = gitCall(source, "fetch", "--no-tags", "--no-write-fetch-head", p.Source["remote_name"], selected); err != nil {
			return p, fmt.Errorf("base fetch failed; inspect retained Git state: %w", err)
		}
		if commit(source, selected) != selected {
			return p, errors.New("fetched object differs from approved commit")
		}
	}
	if p.Base["tracking_update_required"] == true {
		old, _ := p.Base["remote_tracking_commit"].(string)
		if old == "" {
			old = strings.Repeat("0", len(selected))
		}
		ref := "refs/remotes/" + p.Source["remote_name"] + "/" + o.BaseBranch
		if _, err = gitCall(source, "update-ref", ref, selected, old); err != nil {
			return p, errors.New("remote tracking reference changed after planning")
		}
	}
	if err = os.MkdirAll(p.Worktree["repository_directory"], 0700); err != nil {
		return p, err
	}
	if err = safeDirectory(p.Worktree["repository_directory"]); err != nil {
		return p, err
	}
	if _, err = gitCall(source, "worktree", "add", "-b", p.Worktree["branch"], p.Worktree["path"], selected); err != nil {
		return p, fmt.Errorf("worktree creation failed; Git effects may remain, inspect %s before retry: %w", p.Worktree["path"], err)
	}
	branch, err := gitCall(p.Worktree["path"], "symbolic-ref", "--short", "HEAD")
	if err != nil || branch != p.Worktree["branch"] || commit(p.Worktree["path"], "HEAD") != selected {
		return p, errors.New("created worktree failed branch/base postcondition; retained for inspection")
	}
	p.Status = "created"
	return p, nil
}
