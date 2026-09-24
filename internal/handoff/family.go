package handoff

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"documentation-vault/internal/config"
	"go.yaml.in/yaml/v3"
)

type FamilyOptions struct{ Vault, Bundle, Worktree string }
type FamilyPlan struct {
	Status    string            `json:"status"`
	Operation string            `json:"operation"`
	Action    string            `json:"action"`
	Target    map[string]string `json:"target"`
	Handoff   map[string]any    `json:"handoff"`
	Effects   []map[string]any  `json:"effects"`
	Token     string            `json:"plan_token"`
	writes    map[string][]byte
}

func familyRun(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o FamilyOptions
	var token, remote, branch, id, state, closure string
	fs.StringVar(&o.Vault, "vault-root", "", "vault root")
	fs.StringVar(&o.Vault, "vault", "", "vault root")
	fs.StringVar(&o.Bundle, "bundle", "", "bundle")
	fs.StringVar(&o.Worktree, "worktree-path", "", "worktree path")
	fs.StringVar(&token, "plan-token", "", "approved plan token")
	fs.StringVar(&remote, "repository-remote", "", "repository remote")
	fs.StringVar(&branch, "branch", "", "branch")
	fs.StringVar(&id, "handoff-id", "", "handoff identity")
	fs.StringVar(&state, "state", "", "lifecycle state")
	fs.StringVar(&closure, "closure-fingerprint", "", "reviewed evidence fingerprint")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	var result any
	var err error
	switch args[0] {
	case "plan":
		result, err = PlanFamily(o)
	case "apply":
		if token == "" {
			return errors.New("--plan-token is required")
		}
		result, err = ApplyFamily(o, token)
	case "validate":
		result, err = ValidateFamilyRepository(o.Vault, remote, o.Worktree)
	case "resolve-branch":
		result, err = ResolveBranch(o.Vault, remote, branch)
	case "set-state":
		result, err = SetState(o.Vault, remote, o.Worktree, id, state, closure, token)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

func identity(work map[string]any, remote string) (family, id, token string) {
	id = digest([]byte(fmt.Sprintf("%s|%s|%s", work["tracker-id"], work["reference"], remote)))
	readable := slug(fmt.Sprintf("%s-%s", work["tracker-id"], work["reference"]))
	if len(readable) > 80 {
		readable = strings.TrimRight(readable[:80], "-")
	}
	token = readable + "-" + id[:10]
	family = token + "--" + slug(remote[strings.LastIndex(remote, "/")+1:])
	return
}
func remoteArgument(remote string) string {
	if !strings.Contains(remote, "://") && !strings.HasPrefix(remote, "git@") {
		return "https://" + remote
	}
	return remote
}
func resolveSource(vault, remote string) (string, string, string, error) {
	located, err := config.LocateRepository(vault, remoteArgument(remote))
	if err != nil {
		return "", "", "", err
	}
	if located["status"] != "ok" {
		return "", "", "", errors.New("repository did not resolve uniquely")
	}
	source := located["path"].(string)
	normalized := located["remote"].(string)
	value, err := gitCall(source, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", "", "", err
	}
	value = strings.TrimRight(value, "/")
	name := value[strings.LastIndexAny(value, "/:")+1:]
	if strings.HasSuffix(strings.ToLower(name), ".git") {
		name = name[:len(name)-4]
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "\\\x00") {
		return "", "", "", errors.New("unsafe repository name")
	}
	return source, normalized, name, nil
}
func resolveTarget(vault, remote, path string) (string, string, string, error) {
	source, normalized, name, err := resolveSource(vault, remote)
	if err != nil {
		return "", "", "", err
	}
	w, err := config.Workspace(vault)
	if err != nil {
		return "", "", "", err
	}
	root, _ := w["worktree_root"].(string)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", "", err
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", "", "", errors.New("worktree path must be absolute and normalized")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", "", err
	}
	if target != path {
		return "", "", "", errors.New("worktree path must have no symlink aliases")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", "", "", err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) != 2 || parts[0] != name || parts[1] == ".." {
		return "", "", "", errors.New("worktree is outside configured repository layout")
	}
	gitRoot, err := gitCall(target, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", "", err
	}
	gitRoot, err = filepath.EvalSymlinks(gitRoot)
	if err != nil || gitRoot != target {
		return "", "", "", errors.New("target is not exact Git root")
	}
	common := func(p string) (string, error) {
		s, e := gitCall(p, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return "", e
		}
		return filepath.EvalSymlinks(s)
	}
	a, err := common(source)
	if err != nil {
		return "", "", "", err
	}
	b, err := common(target)
	if err != nil || a != b {
		return "", "", "", errors.New("target is not a worktree of configured repository")
	}
	branch, err := gitCall(target, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", "", "", errors.New("handoff worktree must have a branch")
	}
	return target, normalized, branch, nil
}

func ResolveBranch(vault, remote, branch string) (map[string]any, error) {
	source, normalized, name, err := resolveSource(vault, remote)
	if err != nil {
		return nil, err
	}
	if _, err = gitCall(source, "check-ref-format", "--branch", branch); err != nil {
		return nil, errors.New("invalid branch")
	}
	w, err := config.Workspace(vault)
	if err != nil {
		return nil, err
	}
	root, _ := w["worktree_root"].(string)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, name, branch[strings.LastIndex(branch, "/")+1:])
	if _, err = os.Lstat(path); os.IsNotExist(err) {
		return map[string]any{"status": "unavailable", "availability": "not-local", "repository_remote": normalized, "branch": branch, "derived_path": path}, nil
	}
	target, remote, observed, err := resolveTarget(vault, remote, path)
	if err != nil {
		return nil, err
	}
	if observed != branch {
		return nil, errors.New("derived worktree is attached to different branch")
	}
	return map[string]any{"status": "available", "availability": "local-verified", "repository_remote": remote, "branch": branch, "worktree_path": target}, nil
}

func checkTracker(vault string, b bundle) error {
	m, err := config.LoadInstance(vault)
	if err != nil {
		return err
	}
	items, _ := m["trackers"].([]any)
	n := 0
	for _, item := range items {
		entry, _ := item.(map[string]any)
		if entry["id"] == b.work["tracker-id"] {
			n++
			s, _ := entry["url"].(string)
			u, e := trackerURL(s)
			if e != nil || u != b.work["tracker-url"] || entry["provider"] != b.work["provider"] {
				return errors.New("tracker binding mismatch")
			}
		}
	}
	if n != 1 {
		return errors.New("tracker must resolve exactly once")
	}
	return nil
}
func optionalFile(path string) ([]byte, error) {
	raw, err := readFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}
func registry(target string, skipTransaction bool) (activeRegistry, error) {
	store := filepath.Join(target, storeName)
	var a activeRegistry
	if _, err := os.Lstat(store); err == nil {
		if err = safeDirectory(store); err != nil {
			return a, err
		}
	} else if !os.IsNotExist(err) {
		return a, err
	}
	if !skipTransaction {
		if err := noTransaction(store); err != nil {
			return a, err
		}
	}
	raw, err := optionalFile(filepath.Join(store, "ACTIVE.yaml"))
	if err != nil {
		return a, err
	}
	if raw == nil {
		return activeRegistry{Schema: 2, Entries: []Entry{}}, nil
	}
	if err = decode(raw, &a); err != nil {
		return a, err
	}
	if a.Schema == 1 {
		if a.Investigation != "" || a.Entries != nil {
			return a, errors.New("legacy registry contains schema-2 fields")
		}
		a.Entries = []Entry{{ID: a.ID, Family: a.Family, Revision: a.Revision, Manifest: a.Manifest, ActivatedAt: a.ActivatedAt, State: "active"}}
	} else if a.Schema != 2 {
		return a, errors.New("unsupported ACTIVE schema")
	} else if a.ID != "" || a.Family != "" || a.Revision != "" || a.Manifest != "" || a.ActivatedAt != "" || strings.TrimSpace(a.Investigation) == "" {
		return a, errors.New("invalid schema-2 ACTIVE registry")
	}
	seen := map[string]bool{}
	for _, e := range a.Entries {
		if err = validEntry(e); err != nil {
			return a, err
		}
		if seen[e.ID] || seen[e.Family] {
			return a, errors.New("duplicate active identity")
		}
		seen[e.ID], seen[e.Family] = true, true
		inv, err := inspectFamily(store, e)
		if err != nil {
			return a, err
		}
		if a.Schema == 1 {
			a.Investigation = inv
		}
		if inv != a.Investigation {
			return a, errors.New("investigation mismatch")
		}
	}
	if len(a.Entries) == 0 {
		return a, errors.New("empty persisted ACTIVE registry")
	}
	return activeRegistry{Schema: 2, Investigation: a.Investigation, Entries: a.Entries}, nil
}
func loadManifest(path string) (manifest, error) {
	var m manifest
	raw, err := readFile(path)
	if err != nil {
		return m, err
	}
	err = decode(raw, &m)
	return m, err
}

func stateFingerprint(root string, paths ...string) (string, error) {
	records := map[string]any{}
	for _, rel := range paths {
		p := filepath.Join(root, rel)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			if rel != storeName {
				records[rel] = nil
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, e := os.Readlink(p)
			if e != nil {
				return "", e
			}
			records[rel] = map[string]string{"symlink": link}
			continue
		}
		if info.IsDir() {
			err = filepath.WalkDir(p, func(path string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if d.Name() == ".ACTIVE.lock" || d.Name() == ".APPLY.lock" || strings.HasPrefix(d.Name(), ".APPLY.transaction") {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				key, _ := filepath.Rel(root, path)
				if d.IsDir() {
					if key != storeName {
						records[key] = "directory"
					}
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 {
					return errors.New("symlink in handoff state")
				}
				data, e := readFile(path)
				if e != nil {
					return e
				}
				records[key] = digest(data)
				return nil
			})
			if err != nil {
				return "", err
			}
		} else {
			data, e := readFile(p)
			if e != nil {
				return "", e
			}
			records[rel] = digest(data)
		}
	}
	raw, _ := json.Marshal(records)
	return digest(raw), nil
}

func PlanFamily(o FamilyOptions) (FamilyPlan, error) {
	p := FamilyPlan{}
	b, err := loadBundle(o.Bundle)
	if err != nil {
		return p, err
	}
	if err = checkTracker(o.Vault, b); err != nil {
		return p, err
	}
	target, remote, branch, err := resolveTarget(o.Vault, b.remote, o.Worktree)
	if err != nil {
		return p, err
	}
	return planFamilyResolved(o, b, target, target, remote, branch)
}

func planFamilyResolved(o FamilyOptions, b bundle, target, displayTarget, remote, branch string) (FamilyPlan, error) {
	p := FamilyPlan{}
	a, err := registry(target, false)
	if err != nil {
		return p, err
	}
	legacyActive := false
	if raw, e := optionalFile(filepath.Join(target, storeName, "ACTIVE.yaml")); e != nil {
		return p, e
	} else if raw != nil {
		var header activeRegistry
		if e = decode(raw, &header); e != nil {
			return p, e
		}
		legacyActive = header.Schema == 1
	}
	source := b.metadata["source"].(map[string]any)
	investigation := source["investigation-id"].(string)
	family, id, token := identity(b.work, remote)
	if len(a.Entries) == 0 {
		if !strings.Contains(branch, "/") || !strings.HasPrefix(branch[strings.LastIndex(branch, "/")+1:], token+"-") {
			return p, errors.New("worktree branch does not match work item")
		}
	} else if a.Investigation != investigation {
		return p, errors.New("worktree investigation mismatch")
	}
	for i, e := range a.Entries {
		m, err := loadManifest(filepath.Join(target, storeName, e.Family, "handoff.yaml"))
		if err != nil {
			return p, err
		}
		if m.Repository.Remote != remote {
			return p, errors.New("stored repository remote mismatch")
		}
		if _, err = validateUpdates(filepath.Join(target, storeName, e.Family, "implementation-updates.md")); err != nil {
			return p, err
		}
		if i == 0 {
			_, _, first := identity(m.WorkItem, remote)
			if !strings.Contains(branch, "/") || !strings.HasPrefix(branch[strings.LastIndex(branch, "/")+1:], first+"-") {
				return p, errors.New("worktree branch does not match first handoff")
			}
		}
	}
	store := filepath.Join(target, storeName)
	familyPath := filepath.Join(store, family)
	var existing *manifest
	if _, err = os.Lstat(familyPath); err == nil {
		m, e := loadManifest(filepath.Join(familyPath, "handoff.yaml"))
		if e != nil {
			return p, e
		}
		entry := Entry{ID: id, Family: family, Revision: m.Revision, Manifest: family + "/handoff.yaml", ActivatedAt: m.UpdatedAt, State: "active"}
		if _, e = inspectFamily(store, entry); e != nil {
			return p, e
		}
		if m.Source.Investigation != investigation || m.Repository.Remote != remote {
			return p, errors.New("existing family identity mismatch")
		}
		for _, key := range []string{"tracker-id", "provider", "tracker-url", "reference"} {
			if m.WorkItem[key] != b.work[key] {
				return p, errors.New("existing family tracker binding mismatch")
			}
		}
		existing = &m
	} else if !os.IsNotExist(err) {
		return p, err
	}
	desired, err := prepareInstructions(target)
	if err != nil {
		return p, err
	}
	ignore, err := prepareIgnore(target)
	if err != nil {
		return p, err
	}
	if target == displayTarget {
		current, e := optionalFile(filepath.Join(target, ".gitignore"))
		if e != nil {
			return p, e
		}
		if bytes.Equal(current, ignore) {
			if e = verifyIgnored(target); e != nil {
				return p, e
			}
		}
	}
	desired[".gitignore"] = ignore
	writes := map[string][]byte{}
	for rel, data := range desired {
		current, e := optionalFile(filepath.Join(target, rel))
		if e != nil {
			return p, e
		}
		if !bytes.Equal(current, data) {
			writes[rel] = data
		}
	}
	updatesAction, updates, err := prepareUpdates(familyPath)
	if err != nil {
		return p, err
	}
	if existing != nil && updatesAction == "create" {
		return p, errors.New("existing handoff missing implementation updates")
	}
	prefix := storeName + "/" + family + "/"
	if updatesAction == "create" {
		writes[prefix+"implementation-updates.md"] = updatesAsset()
	}
	changed, unchanged := []string{}, []string{}
	documents := map[string][]byte{}
	for _, name := range []string{"context.md", "work-item.md", "scope.md"} {
		raw := b.documents[name]
		documents[name] = raw
		if existing == nil || existing.Files[name].SemanticSHA != digest(semanticBytes(raw)) {
			changed = append(changed, name)
		} else {
			unchanged = append(unchanged, name)
		}
	}
	action, revision := "create", "v0001"
	var previous *string
	if existing != nil {
		prev := existing.Revision
		previous = &prev
		revision = prev
		if len(changed) > 0 {
			n, _ := strconv.Atoi(prev[1:])
			if n >= 9999 {
				return p, errors.New("revision limit reached")
			}
			revision = fmt.Sprintf("v%04d", n+1)
			action = "update"
		} else {
			action = "activate"
			for _, e := range a.Entries {
				if e.ID == id && e.Revision == revision && e.State == "active" {
					action = "noop"
				}
			}
			if legacyActive {
				action = "activate"
			}
			if action == "noop" && len(writes) > 0 {
				action = "bootstrap"
			}
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if action == "create" || action == "update" {
		if existing == nil {
			writes[prefix+"START.md"] = startAsset()
		}
		eventRaw, e := familyHistory(b, id, revision, previous, existing, changed, unchanged, documents, familyPath, now)
		if e != nil {
			return p, e
		}
		writes[prefix+"history/"+revision+".md"] = eventRaw
		for _, name := range changed {
			writes[prefix+name] = documents[name]
		}
		m := manifest{Schema: 2, ID: id, Family: family, Revision: revision, Previous: previous, CreatedAt: now, UpdatedAt: now, WorkItem: b.work, Files: map[string]fileRecord{}}
		m.Repository.Remote = remote
		m.Source.Investigation = investigation
		m.Source.UpdatedAt = source["investigation-updated-at"].(string)
		m.Source.Story = source["story-id"].(string)
		if existing != nil {
			m.CreatedAt = existing.CreatedAt
			m.Files["START.md"] = existing.Files["START.md"]
		} else {
			m.Files["START.md"] = fileRecord{SHA: digest(startAsset())}
		}
		for _, name := range []string{"context.md", "work-item.md", "scope.md"} {
			data := documents[name]
			if existing != nil && existing.Files[name].SemanticSHA == digest(semanticBytes(data)) {
				m.Files[name] = existing.Files[name]
			} else {
				m.Files[name] = fileRecord{SHA: digest(data), SemanticSHA: digest(semanticBytes(data))}
			}
		}
		m.History.Path = "history/" + revision + ".md"
		m.History.SHA = digest(eventRaw)
		raw, e := yaml.Marshal(m)
		if e != nil {
			return p, e
		}
		writes[prefix+"handoff.yaml"] = raw
	}
	if action == "create" || action == "update" || action == "activate" {
		a.Investigation = investigation
		e := Entry{ID: id, Family: family, Revision: revision, Manifest: family + "/handoff.yaml", ActivatedAt: now, State: "active"}
		found := false
		for i, item := range a.Entries {
			if item.ID == id {
				a.Entries[i] = e
				found = true
			}
		}
		if !found {
			a.Entries = append(a.Entries, e)
		}
		raw, e2 := marshalRegistry(a)
		if e2 != nil {
			return p, e2
		}
		writes[storeName+"/ACTIVE.yaml"] = raw
	}
	effects := []map[string]any{}
	keys := []string{}
	for rel := range writes {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		action := "update"
		if _, e := os.Lstat(filepath.Join(target, rel)); os.IsNotExist(e) {
			action = "create"
		}
		_, trackedErr := gitCall(target, "ls-files", "--error-unmatch", "--", rel)
		effects = append(effects, map[string]any{"path": rel, "action": action, "tracked": trackedErr == nil})
	}
	fingerprint, err := stateFingerprint(target, "AGENTS.md", "CLAUDE.md", "AGENTS.override.md", ".gitignore", storeName)
	if err != nil {
		return p, err
	}
	policy := map[string]string{}
	for rel, data := range desired {
		policy[rel] = digest(data)
	}
	payload, _ := json.Marshal(map[string]any{"schema": 1, "operation": "apply", "bundle": b.fingerprint, "target": displayTarget, "remote": remote, "branch": branch, "head": commit(target, "HEAD"), "state": fingerprint, "policy": policy, "start": digest(startAsset()), "updates": digest(updatesAsset()), "action": action, "effects": effects})
	p = FamilyPlan{Status: "planned", Operation: "apply", Action: action, Target: map[string]string{"path": displayTarget, "remote": remote, "branch": branch}, Handoff: map[string]any{"id": id, "family": family, "revision": revision, "previous_revision": previous, "changed_documents": changed, "unchanged_documents": unchanged, "implementation_updates": updates}, Effects: effects, Token: digest(payload), writes: writes}
	return p, nil
}

func marshalRegistry(a activeRegistry) ([]byte, error) {
	return yaml.Marshal(map[string]any{"schema-version": 2, "investigation-id": a.Investigation, "handoffs": a.Entries})
}
func familyHistory(b bundle, id, revision string, previous *string, existing *manifest, changed, unchanged []string, docs map[string][]byte, dir, now string) ([]byte, error) {
	change := b.metadata["change"].(map[string]any)
	summary := change["summary"].(string)
	reasons, _ := change["reasons"].(map[string]any)
	entries := []any{}
	var previousHash *string
	if existing != nil {
		hash := existing.History.SHA
		previousHash = &hash
	}
	for _, name := range changed {
		var old any
		if existing != nil {
			old = existing.Files[name].SemanticSHA
		}
		reason := summary
		if s, ok := reasons[name].(string); ok {
			reason = s
		}
		entries = append(entries, map[string]any{"path": name, "previous-semantic-sha256": old, "semantic-sha256": digest(semanticBytes(docs[name])), "reason": reason})
	}
	event := historyEvent{Schema: 1, ID: id, Revision: revision, Previous: previous, PreviousSHA: previousHash, CreatedAt: now, Reason: summary, Changed: entries, Unchanged: unchanged}
	raw, err := yaml.Marshal(event)
	if err != nil {
		return nil, err
	}
	result := []byte("---\n" + string(raw) + "---\n# Revision " + revision + "\n\n" + summary + "\n")
	if existing != nil {
		for _, name := range changed {
			old, err := readFile(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			patch, err := historyPatch(name, semanticBytes(old), semanticBytes(docs[name]))
			if err != nil {
				return nil, err
			}
			result = append(result, patch...)
		}
	}
	return result, nil
}

func historyPatch(name string, before, after []byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "vaultctl-handoff-diff-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err = os.WriteFile(filepath.Join(dir, "before"), before, 0600); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dir, "after"), after, 0600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--text", "--unified=3", "--", "before", "after")
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, errors.New("could not compute handoff revision diff")
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\n### `%s`\n\n", name)
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "index ") {
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			line = "--- a/" + name
		}
		if strings.HasPrefix(line, "+++ ") {
			line = "+++ b/" + name
		}
		out.WriteString("    ")
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return []byte(out.String()), nil
}

func ApplyFamily(o FamilyOptions, token string) (FamilyPlan, error) {
	var result FamilyPlan
	b, err := loadBundle(o.Bundle)
	if err != nil {
		return result, err
	}
	if err = checkTracker(o.Vault, b); err != nil {
		return result, err
	}
	target, remote, _, err := resolveTarget(o.Vault, b.remote, o.Worktree)
	if err != nil {
		return result, err
	}
	err = WithStoreLock(target, func() error {
		legacy, err := checkLegacyRecoveryBundle(target, b, remote)
		if err != nil {
			return err
		}
		if err := recoverLocked(o.Worktree, token); err != nil {
			return err
		}
		if legacy {
			return errors.New("legacy handoff attempt recovered; generate and review a native plan before reapplying")
		}
		p, err := PlanFamily(o)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(p.Token), []byte(token)) != 1 {
			return errors.New("plan_stale: family or inputs changed")
		}
		result = p
		if p.Action == "noop" {
			if _, err = ValidateFamilyRepository(o.Vault, p.Target["remote"], o.Worktree); err != nil {
				return err
			}
			result.Status = "unchanged"
			return nil
		}
		err = applyTransactionLocked(o.Worktree, token, p.writes, func() error {
			_, e := validateFamilyRepository(o.Vault, p.Target["remote"], o.Worktree, true)
			return e
		})
		if err == nil {
			result.Status = "applied"
		}
		return err
	})
	return result, err
}

func checkLegacyRecoveryBundle(root string, b bundle, remote string) (bool, error) {
	raw, err := optionalFile(filepath.Join(root, storeName, transactionName, "transaction.json"))
	if err != nil {
		return false, err
	}
	if raw == nil {
		return false, nil
	}
	var metadata map[string]any
	if err = json.Unmarshal(raw, &metadata); err != nil {
		return false, err
	}
	if metadata["format"] != nil {
		return false, nil
	}
	_, id, _ := identity(b.work, remote)
	if metadata["bundle-fingerprint"] != b.fingerprint || metadata["handoff-id"] != id {
		return false, errors.New("legacy recovery requires its exact original bundle and handoff")
	}
	return true, nil
}
