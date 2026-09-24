package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		return Object{"status": "uninitialized", "source_context": Object{"status": "unavailable", "roots": []any{}, "clone_root": nil, "clone_origin": nil, "clone_authorized": false, "warnings": []string{"workspace configuration is missing; configure workspace"}}}, nil
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
	w, e := Workspace(root)
	if e != nil {
		return nil, e
	}
	matches := []string{}
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
			if e != nil {
				continue
			}
			if seen[canonical] {
				continue
			}
			seen[canonical] = true
			if _, e := os.Stat(filepath.Join(canonical, ".git")); e != nil {
				continue
			}
			got, e := RemoteIdentity(gitRemote(canonical))
			if e == nil && got == want {
				matches = append(matches, canonical)
			}
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		return Object{"status": "not_found", "remote": want}, nil
	case 1:
		return Object{"status": "ok", "path": matches[0], "remote": want}, nil
	default:
		return Object{"status": "ambiguous", "matches": matches, "remote": want}, nil
	}
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
