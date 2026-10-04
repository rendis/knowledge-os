package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const WorkspaceFile = ".knowledge-os-config.yaml"

func ValidateWorkspace(m Object) error {
	if m["version"] != 1 {
		return errors.New("workspace version must be 1")
	}
	for _, field := range []string{"workspace", "skills"} {
		if v, ok := m[field]; ok {
			if _, ok := v.(map[string]any); !ok {
				return fmt.Errorf("%s must be a mapping", field)
			}
		}
	}
	w := obj(m["workspace"])
	if v, ok := w["repository_roots"]; ok {
		if _, e := stringsList(v, false); e != nil {
			return fmt.Errorf("repository_roots: %w", e)
		}
	}
	for _, pair := range []struct {
		m Object
		k string
	}{{w, "managed_clone"}, {obj(m["skills"]), "manage-development-handoff"}, {obj(m["skills"]), "inspect-database"}} {
		if v, ok := pair.m[pair.k]; ok {
			if _, ok := v.(map[string]any); !ok {
				return fmt.Errorf("%s must be a mapping", pair.k)
			}
		}
	}
	managed := obj(w["managed_clone"])
	if v, ok := managed["enabled"]; ok {
		if _, ok := v.(bool); !ok {
			return errors.New("managed_clone.enabled must be boolean")
		}
	}
	for _, v := range []any{managed["root"], obj(obj(m["skills"])["manage-development-handoff"])["worktree_root"]} {
		if v != nil {
			if _, ok := v.(string); !ok || strings.ContainsAny(str(v), "\n\r\x00") {
				return errors.New("invalid workspace path")
			}
		}
	}
	env := obj(obj(m["skills"])["inspect-database"])["environments"]
	if env != nil {
		if _, ok := env.(map[string]any); !ok {
			return errors.New("environments must be mapping")
		}
		for name, p := range obj(env) {
			if !nonempty(name) || strings.ContainsAny(name, "\r\n\x00") {
				return errors.New("invalid environment name")
			}
			if _, e := portValue(p); e != nil {
				return e
			}
		}
	}
	return nil
}
func portValue(v any) (string, error) {
	if m, ok := v.(map[string]any); ok {
		v = m["proxy_port"]
	}
	s := ""
	switch x := v.(type) {
	case string:
		s = x
	case int:
		s = strconv.Itoa(x)
	default:
		return "", errors.New("invalid proxy port")
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < 1 || n > 65535 || strings.Trim(s, "0123456789") != "" {
		return "", errors.New("proxy port must be integer 1..65535")
	}
	return strconv.Itoa(n), nil
}
func Workspace(root string) (Object, error) {
	_, m, _, e := readYAML(filepath.Join(root, WorkspaceFile))
	if os.IsNotExist(e) {
		return Object{"status": "uninitialized", "source_context": Object{"status": "unavailable", "roots": []any{}, "clone_root": nil, "clone_origin": nil, "clone_authorized": false, "warnings": []string{"workspace configuration is missing: run onboard-developer (config detect proposes it)"}}}, nil
	}
	if e != nil {
		return nil, e
	}
	if e = ValidateWorkspace(m); e != nil {
		return nil, e
	}
	w := obj(m["workspace"])
	managed := obj(w["managed_clone"])
	clone := str(managed["root"])
	enabled, _ := managed["enabled"].(bool)
	roots := []any{}
	for _, r := range list(w["repository_roots"]) {
		roots = append(roots, Object{"path": r, "origin": "config", "managed": enabled && r == clone})
	}
	status, source := "initialized", "resolved"
	warnings := []string{}
	if len(roots) == 0 {
		status, source = "uninitialized", "unavailable"
		warnings = append(warnings, "no repository roots configured")
	}
	ports := Object{}
	for env, v := range obj(obj(obj(m["skills"])["inspect-database"])["environments"]) {
		p, _ := portValue(v)
		ports[env] = p
	}
	var origin any
	if enabled {
		origin = "config"
	}
	return Object{"status": status, "source_context": Object{"status": source, "roots": roots, "clone_root": nullable(clone), "clone_origin": origin, "clone_authorized": enabled && clone != "", "warnings": warnings}, "worktree_root": nullable(str(obj(obj(m["skills"])["manage-development-handoff"])["worktree_root"])), "proxy_ports": ports}, nil
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func LocateRepository(root, remote string) (Object, error) {
	want, e := RemoteIdentity(remote)
	if e != nil {
		return nil, e
	}
	paths, e := Checkouts(root)
	if e != nil {
		return nil, e
	}
	matches := []string{}
	for _, p := range paths {
		got, e := RemoteIdentity(gitRemote(p))
		if e == nil && got == want {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return Object{"status": "not_found", "remote": want}, nil
	case 1:
		return Object{"status": "ok", "path": matches[0], "remote": want}, nil
	default:
		return Object{"status": "ambiguous", "matches": matches, "remote": want}, nil
	}
}

// Checkouts are the Git checkouts the workspace roots expose: a root that is itself a checkout, otherwise its
// immediate children. There is no recursive scan.
func Checkouts(root string) ([]string, error) {
	w, e := Workspace(root)
	if e != nil {
		return nil, e
	}
	out := []string{}
	seen := map[string]bool{}
	for _, r := range list(obj(w["source_context"])["roots"]) {
		p := str(obj(r)["path"])
		if !filepath.IsAbs(p) {
			return nil, errors.New("repository roots must be absolute for identity lookup")
		}
		candidates := []string{p}
		if _, e := os.Stat(filepath.Join(p, ".git")); os.IsNotExist(e) {
			entries, e := os.ReadDir(p)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			candidates = nil
			for _, entry := range entries {
				if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
					candidates = append(candidates, filepath.Join(p, entry.Name()))
				}
			}
		}
		for _, candidate := range candidates {
			canonical, e := filepath.EvalSymlinks(candidate)
			if e != nil || seen[canonical] {
				continue
			}
			seen[canonical] = true
			if _, e := os.Stat(filepath.Join(canonical, ".git")); e == nil {
				out = append(out, canonical)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

var (
	noteAliases  = regexp.MustCompile(`(?m)^aliases:\s*\[(.*?)\]`)
	noteAnalyzed = regexp.MustCompile(`(?m)^commit-analizado:\s*"?([0-9a-f]+)"?`)
)

// LocateRepositoryByName binds a repository named by its note, one of the note's aliases or its repository
// name to the local checkout and the reference branch a reader must use, without contacting any remote: the
// branch is `sources.reference_branches`, else the first of `sources.reference_branch_order` present in the
// checkout, and `note` compares the note's `commit-analizado` with that branch as of the checkout's last fetch.
func LocateRepositoryByName(root, name string) (Object, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("--repo needs a repository, note or alias name")
	}
	names := map[string]bool{strings.ToLower(name): true}
	notePath, analyzed := "", ""
	_ = filepath.WalkDir(filepath.Join(root, "20-Repos"), func(p string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() || !strings.HasSuffix(p, ".md") || notePath != "" {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil
		}
		own := []string{strings.TrimSuffix(d.Name(), ".md")}
		if m := noteAliases.FindStringSubmatch(string(b)); m != nil {
			for _, a := range strings.Split(m[1], ",") {
				if a = strings.Trim(strings.TrimSpace(a), `"'`); a != "" {
					own = append(own, a)
				}
			}
		}
		for _, n := range own {
			if strings.EqualFold(n, name) {
				rel, _ := filepath.Rel(root, p)
				notePath = filepath.ToSlash(rel)
				if m := noteAnalyzed.FindStringSubmatch(string(b)); m != nil {
					analyzed = m[1]
				}
				for _, o := range own {
					names[strings.ToLower(o)] = true
				}
				break
			}
		}
		return nil
	})
	paths, e := Checkouts(root)
	if e != nil {
		return nil, e
	}
	type match struct{ path, repo, remote string }
	matches := []match{}
	for _, p := range paths {
		remote := gitRemote(p)
		repo := filepath.Base(p)
		if remote != "" {
			r := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
			repo = r[strings.LastIndexAny(r, "/:")+1:]
		}
		if names[strings.ToLower(repo)] {
			matches = append(matches, match{p, repo, remote})
		}
	}
	res := Object{"repo": name}
	if notePath != "" {
		res["note"] = notePath
	}
	switch len(matches) {
	case 0:
		res["status"] = "not_found"
		return res, nil
	case 1:
	default:
		found := []string{}
		for _, m := range matches {
			found = append(found, m.path)
		}
		res["status"], res["matches"] = "ambiguous", found
		return res, nil
	}
	m := matches[0]
	res["status"], res["repo"], res["path"] = "ok", m.repo, m.path
	if id, e := RemoteIdentity(m.remote); e == nil {
		res["remote"] = id
	}
	inst, e := LoadInstance(root)
	if e != nil {
		return nil, e
	}
	branches, e := ReferenceBranches(inst)
	if e != nil {
		return nil, e
	}
	candidates := ReferenceBranchOrder(inst)
	if configured := branches[strings.ToLower(m.repo)]; configured != "" {
		candidates = []string{configured}
	}
	for _, b := range candidates {
		for _, ref := range []string{"refs/remotes/origin/" + b, "refs/heads/" + b} {
			out, e := exec.Command("git", "-C", m.path, "rev-parse", "--verify", "--quiet", ref+"^{commit}").Output()
			if e != nil {
				continue
			}
			head := strings.TrimSpace(string(out))
			res["reference_branch"], res["ref"], res["head"] = b, ref, head[:12]
			switch {
			case notePath == "":
				res["note_state"] = "no-note"
			case analyzed != "" && strings.HasPrefix(head, analyzed):
				res["note_state"] = "current"
			default:
				res["note_state"] = "changed"
			}
			return res, nil
		}
	}
	res["reference_error"] = "none of " + strings.Join(candidates, ", ") + " exists in the checkout; fetch it or declare the branch in sources.reference_branches"
	return res, nil
}

func SchemaRepository(root string) (Object, error) {
	m, e := LoadInstance(root)
	if e != nil {
		return nil, e
	}
	s := obj(obj(m["sources"])["schema_repository"])
	remote := str(s["remote"])
	if remote == "" {
		return Object{"status": "not_configured"}, nil
	}
	r, e := LocateRepository(root, remote)
	if e == nil {
		r["note"] = s["note"]
	}
	return r, e
}
