// Package config resolves vault identity and local operational configuration.
// It does not infer operational facts or execute configured procedures.
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Object = map[string]any

var kebab = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var markers = []string{"AGENTS.md", "00-Home.md", "instance.yaml", "90-Meta/Convenciones.md", "90-Meta/Auditoria - Framework.md"}
var defaultTypes = []string{"sistema", "servicio", "componente", "recurso-runtime", "repositorio", "topic", "evento", "flujo", "integracion-externa", "glosario", "operacional", "aprendizaje", "indice"}

func obj(x any) Object {
	if v, ok := x.(map[string]any); ok {
		return v
	}
	return Object{}
}
func str(x any) string    { s, _ := x.(string); return s }
func list(x any) []any    { a, _ := x.([]any); return a }
func nonempty(x any) bool { s, ok := x.(string); return ok && strings.TrimSpace(s) != "" }
func stringsList(x any, nonemptyList bool) ([]string, error) {
	a, ok := x.([]any)
	if !ok || (nonemptyList && len(a) == 0) {
		return nil, errors.New("expected text list")
	}
	r := []string{}
	seen := map[string]bool{}
	for _, v := range a {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" || s != strings.TrimSpace(s) || strings.ContainsAny(s, "\r\n\x00") || seen[s] {
			return nil, errors.New("invalid or duplicate text-list entry")
		}
		seen[s] = true
		r = append(r, s)
	}
	return r, nil
}

const maxConfigBytes int64 = 4 << 20

func readYAML(path string) (*yaml.Node, Object, []byte, error) {
	st, e := os.Lstat(path)
	if e != nil {
		return nil, nil, nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, nil, nil, fmt.Errorf("%s must be a regular file", path)
	}
	b, e := readConfigBytes(path)
	if e != nil {
		return nil, nil, nil, e
	}
	var n yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(b))
	if e = d.Decode(&n); e != nil {
		return nil, nil, nil, e
	}
	var tail any
	if e = d.Decode(&tail); e != io.EOF {
		return nil, nil, nil, errors.New("expected one YAML document")
	}
	var m Object
	if e = n.Decode(&m); e != nil {
		return nil, nil, nil, e
	}
	if m == nil {
		return nil, nil, nil, errors.New("expected YAML mapping")
	}
	return &n, m, b, nil
}

// LoadInstance validates known fields while preserving consumer-owned fields.
func LoadInstance(root string) (Object, error) {
	_, m, _, e := readYAML(filepath.Join(root, "instance.yaml"))
	if e != nil {
		return nil, e
	}
	if e = ValidateInstance(m); e != nil {
		return nil, e
	}
	return m, nil
}
func ValidateInstance(m Object) error {
	c := obj(m["cell"])
	if !nonempty(c["name"]) || !nonempty(c["purpose"]) {
		return errors.New("cell.name and cell.purpose are required")
	}
	systems, ok := m["systems"].([]any)
	if !ok || len(systems) == 0 {
		return errors.New("systems must be a nonempty list")
	}
	seen := map[string]bool{}
	for _, s := range systems {
		v := obj(s)
		id := str(v["id"])
		if !kebab.MatchString(id) || !nonempty(v["name"]) || seen[id] {
			return errors.New("systems require unique kebab-case id and name")
		}
		seen[id] = true
		if a, exists := v["aliases"]; exists && a != nil {
			if _, ok := a.(string); !ok {
				if _, e := stringsList(a, false); e != nil {
					return fmt.Errorf("system aliases: %w", e)
				}
			}
		}
	}
	for _, field := range []string{"vault", "sources", "graph", "evidence", "locale", "capabilities"} {
		if v, exists := m[field]; exists && v != nil {
			if _, ok := v.(map[string]any); !ok {
				return fmt.Errorf("%s must be a mapping", field)
			}
		}
	}
	for _, pair := range []struct {
		m     Object
		field string
	}{{obj(m["evidence"]), "profile"}, {obj(m["locale"]), "notes"}, {obj(m["vault"]), "remote"}} {
		if v, ok := pair.m[pair.field]; ok && v != nil {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s must be text", pair.field)
			}
		}
	}
	if v := str(obj(m["evidence"])["profile"]); v != "" && v != "production-gate" && v != "documented-source" && v != "mixed" {
		return errors.New("invalid evidence.profile")
	}
	if v := str(obj(m["locale"])["notes"]); v != "" && v != "es" && v != "en" {
		return errors.New("invalid locale.notes")
	}
	if v, ok := m["adapters"]; ok && v != nil {
		var names []string
		if s, ok := v.(string); ok {
			names = strings.Split(s, ",")
		} else {
			var e error
			names, e = stringsList(v, false)
			if e != nil {
				return e
			}
		}
		for _, s := range names {
			if strings.TrimSpace(s) != "reports" && strings.TrimSpace(s) != "" {
				return errors.New("unknown adapter")
			}
		}
	}
	for id, v := range obj(m["capabilities"]) {
		if !kebab.MatchString(id) {
			return errors.New("invalid capability id")
		}
		names, e := stringsList(v, true)
		if e != nil {
			return e
		}
		for _, n := range names {
			if !basename(n) {
				return errors.New("capabilities require canonical note basenames")
			}
		}
	}
	if v, exists := obj(m["sources"])["reference_branches"]; exists {
		branches, ok := v.(map[string]any)
		if !ok {
			return errors.New("reference_branches must be a mapping")
		}
		for repo, b := range branches {
			if !regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`).MatchString(repo) || !validBranch(str(b)) {
				return errors.New("invalid reference branch policy")
			}
		}
	}
	if v, exists := obj(m["sources"])["reference_branch_order"]; exists && v != nil {
		order, ok := v.([]any)
		seen := map[string]bool{}
		if !ok || len(order) == 0 {
			return errors.New("reference_branch_order must list branch names")
		}
		for _, b := range order {
			if !validBranch(str(b)) || seen[str(b)] {
				return errors.New("invalid reference_branch_order")
			}
			seen[str(b)] = true
		}
	}
	if t, exists := m["trackers"]; exists && t != nil {
		a, ok := t.([]any)
		if !ok {
			return errors.New("trackers must be a list")
		}
		ids, targets := map[string]bool{}, map[string]bool{}
		for _, v := range a {
			t := obj(v)
			id, p := str(t["id"]), strings.ToLower(str(t["provider"]))
			u, e := httpsURL(str(t["url"]))
			if e != nil || !kebab.MatchString(id) || !kebab.MatchString(p) || ids[id] || targets[p+u] {
				return errors.New("invalid or duplicate tracker")
			}
			ids[id] = true
			targets[p+u] = true
		}
	}
	if v, exists := m["database_targets"]; exists && v != nil {
		a, ok := v.([]any)
		if !ok {
			return errors.New("database_targets must be a list")
		}
		ids := map[string]bool{}
		for _, x := range a {
			t := obj(x)
			if e := allowed(t, "id system environment instance database procedure port_key schemas repositories"); e != nil {
				return e
			}
			for _, f := range []string{"id", "system", "environment", "instance", "database", "procedure"} {
				s := str(t[f])
				if !nonempty(s) || s != strings.TrimSpace(s) || strings.ContainsAny(s, "\r\n\x00") {
					return fmt.Errorf("invalid database target %s", f)
				}
			}
			id := str(t["id"])
			if !kebab.MatchString(id) || ids[id] || !seen[str(t["system"])] || !basename(str(t["procedure"])) {
				return errors.New("invalid database target identity or procedure")
			}
			ids[id] = true
			for _, f := range []string{"schemas", "repositories"} {
				if v, ok := t[f]; ok {
					values, e := stringsList(v, false)
					if e != nil {
						return e
					}
					if f == "repositories" {
						for _, s := range values {
							if _, e = httpsURL(s); e != nil {
								return e
							}
						}
					}
				}
			}
			if v, ok := t["port_key"]; ok {
				if !nonempty(v) || strings.ContainsAny(str(v), "\r\n\x00") {
					return errors.New("invalid target port_key")
				}
			}
		}
	}
	return nil
}
func basename(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.HasSuffix(s, ".md") && !strings.ContainsAny(s, "/\\\r\n[]#\x00")
}
func validBranch(s string) bool {
	if s == "" || s == "@" || s == "HEAD" || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "refs/") || strings.HasSuffix(s, ".") || strings.Contains(s, "..") || strings.Contains(s, "@{") || regexp.MustCompile(`[\x00-\x20\x7f~^:?*\[\\]`).MatchString(s) {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || strings.HasPrefix(p, ".") || strings.HasSuffix(p, ".lock") {
			return false
		}
	}
	return true
}
func httpsURL(s string) (string, error) {
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("expected credential-free HTTPS URL")
	}
	return "https://" + strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/"), nil
}
func allowed(m Object, names string) error {
	set := map[string]bool{}
	for _, n := range strings.Fields(names) {
		set[n] = true
	}
	for n := range m {
		if !set[n] {
			return fmt.Errorf("unsupported field %s", n)
		}
	}
	return nil
}

// RemoteIdentity compares credential-free HTTPS and SSH repository addresses.
func RemoteIdentity(s string) (string, error) {
	if strings.ContainsAny(s, " \t\n\r?#%\\") {
		return "", errors.New("repository must be a credential-free Git remote")
	}
	host, path, port := "", "", ""
	if strings.HasPrefix(s, "git@") && !strings.Contains(s, "://") {
		parts := strings.SplitN(strings.TrimPrefix(s, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", errors.New("invalid SSH remote")
		}
		host, path = parts[0], parts[1]
	} else {
		u, e := url.Parse(s)
		if e != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "ssh") {
			return "", errors.New("expected HTTPS or SSH repository URL")
		}
		if u.User != nil {
			_, pw := u.User.Password()
			if pw || u.Scheme == "https" || u.User.Username() != "git" {
				return "", errors.New("repository credentials are not permitted")
			}
		}
		host, path, port = u.Hostname(), strings.TrimPrefix(u.Path, "/"), u.Port()
		if u.Scheme == "https" && port == "443" || u.Scheme == "ssh" && port == "22" {
			port = ""
		}
	}
	path = strings.TrimRight(path, "/")
	if strings.HasSuffix(strings.ToLower(path), ".git") {
		path = path[:len(path)-4]
	}
	for _, p := range strings.Split(path, "/") {
		if p == "" || p == "." || p == ".." {
			return "", errors.New("invalid repository path")
		}
	}
	if host == "" {
		return "", errors.New("missing repository host")
	}
	if port != "" {
		host += ":" + port
	}
	return strings.ToLower(host + "/" + path), nil
}
func gitRemote(path string) string {
	b, e := exec.Command("git", "-C", path, "config", "--get", "remote.origin.url").Output()
	if e != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
func CanonicalRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("--vault is required")
	}
	if root == "~" || strings.HasPrefix(root, "~/") {
		h, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		if root == "~" {
			root = h
		} else {
			root = filepath.Join(h, strings.TrimPrefix(root, "~/"))
		}
	}
	p, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return "", e
	}
	st, e := os.Stat(p)
	if e != nil || !st.IsDir() {
		return "", errors.New("vault must be a directory")
	}
	return p, nil
}
func Resolve(root string) (Object, error) {
	r, e := CanonicalRoot(root)
	if e != nil {
		return nil, e
	}
	m, e := LoadInstance(r)
	if e != nil {
		return nil, e
	}
	for _, p := range markers {
		st, e := os.Stat(filepath.Join(r, p))
		if e != nil || !st.Mode().IsRegular() {
			return nil, fmt.Errorf("missing vault marker %s", p)
		}
	}
	decl := str(obj(m["vault"])["remote"])
	if decl != "" {
		wanted, e := RemoteIdentity(decl)
		if e != nil {
			return nil, e
		}
		actual, e := RemoteIdentity(gitRemote(r))
		if e != nil || wanted != actual {
			return nil, errors.New("declared vault identity differs from Git origin")
		}
	}
	workspace, e := Workspace(r)
	if e != nil {
		workspace = Object{"source_context": Object{"status": "unavailable", "roots": []any{}, "clone_root": nil, "clone_origin": nil, "clone_authorized": false, "warnings": []string{"workspace configuration is invalid; run configure-workspace before source access"}}}
	}
	available, name := obsidianBinding(r)
	mode := "filesystem"
	if name != "" {
		mode = "obsidian-cli"
	}
	return Object{"status": "resolved", "vault_root": r, "declared_remote": decl, "interaction_mode": mode, "obsidian_available": available, "obsidian_vault": nullable(name), "source_context": workspace["source_context"], "orientation": Orientation(r, m)}, nil
}
func Orientation(root string, m Object) Object {
	issues := []string{}
	for _, p := range markers {
		if st, e := os.Stat(filepath.Join(root, p)); e != nil || !st.Mode().IsRegular() {
			issues = append(issues, "missing marker: "+p)
		}
	}
	names := []string{}
	for _, x := range list(m["systems"]) {
		name := str(obj(x)["name"])
		names = append(names, name)
		p := filepath.Join("10-Sistemas", name+".md")
		if st, e := os.Stat(filepath.Join(root, p)); e != nil || !st.Mode().IsRegular() {
			issues = append(issues, "missing system note: "+filepath.ToSlash(p))
		}
	}
	reason := "ready"
	if len(issues) > 0 {
		reason = "complete-bootstrap"
	}
	profile := str(obj(m["evidence"])["profile"])
	if profile == "" {
		profile = "production-gate"
	}
	types := obj(m["graph"])["enabled_types"]
	if types == nil {
		types = defaultTypes
	}
	pending := false
	if b, e := os.ReadFile(filepath.Join(root, "00-Home.md")); e == nil {
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		for _, heading := range []string{"## Pending inventory", "## Inventario pendiente"} {
			if at := strings.Index(text, heading); at >= 0 {
				section := text[at+len(heading):]
				if end := strings.Index(section, "\n## "); end >= 0 {
					section = section[:end]
				}
				for _, line := range strings.Split(section, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "- ") && !strings.Contains(line, "No discovery roots") {
						pending = true
					}
				}
			}
		}
	}
	return Object{"ready": len(issues) == 0, "reason": reason, "cell": m["cell"], "systems": names, "issues": issues, "start_here": []string{"00-Home.md", "instance.yaml", "10-Sistemas/"}, "evidence_profile": profile, "enabled_types": types, "pending_inventory": pending}
}

// DiscoveryAcceleration reports whether classification questions can be answered by Jev.
// It is optional: without it the agent answers the same questions.
func DiscoveryAcceleration() Object {
	if strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")) != "" {
		return Object{"jev": "configured"}
	}
	return Object{"jev": "not-configured", "hint": "Optional: set TYPESAFE_API_KEY so `discover run` answers classification questions automatically (fast, low cost). Without it the agent answers them; nothing is blocked."}
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

// readConfigBytes rejects symlink targets, detects replacement while opening,
// and bounds allocation even if a file grows after the initial stat.
func readConfigBytes(path string) ([]byte, error) {
	before, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("configuration must be a regular file")
	}
	if before.Size() > maxConfigBytes {
		return nil, errors.New("configuration exceeds 4 MiB limit")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("configuration changed while opening")
	}
	b, e := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > maxConfigBytes {
		return nil, errors.New("configuration exceeds 4 MiB limit")
	}
	return b, nil
}

// ValidBranch checks the shared portable Git reference-name contract.
func ValidBranch(name string) bool { return validBranch(name) }

func obsidianBinding(root string) (bool, string) {
	executable, e := exec.LookPath("obsidian")
	if e != nil {
		return false, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Output is bounded independently of time to keep optional discovery cheap.
	cmd := exec.CommandContext(ctx, executable, "vaults", "verbose")
	var b limitedOutput
	cmd.Stdout = &b
	if e := cmd.Run(); e != nil {
		return false, ""
	}
	return true, matchingObsidianVault(root, b.String())
}

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, errors.New("Obsidian registration output exceeds 1 MiB")
	}
	return b.Buffer.Write(p)
}
func matchingObsidianVault(root, text string) string {
	names := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		name, path, ok := strings.Cut(line, "\t")
		if !ok {
			at := strings.IndexAny(line, " \t")
			if at < 0 {
				continue
			}
			name, path = line[:at], strings.TrimSpace(line[at:])
		}
		candidate, e := CanonicalRoot(strings.TrimSpace(path))
		if e == nil && candidate == root {
			names[strings.TrimSpace(name)] = true
		}
	}
	if len(names) != 1 {
		return ""
	}
	for name := range names {
		return name
	}
	return ""
}

// ResolvePath checks only the supplied path and its ancestors; never siblings.
func ResolvePath(path string) (Object, error) {
	if path == "" {
		return nil, errors.New("--vault is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, e := os.UserHomeDir()
		if e != nil {
			return nil, e
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	candidate, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	candidate, e = filepath.EvalSymlinks(candidate)
	if e != nil {
		return nil, e
	}
	st, e := os.Stat(candidate)
	if e != nil {
		return nil, e
	}
	if !st.IsDir() {
		candidate = filepath.Dir(candidate)
	}
	for {
		if _, e := os.Stat(filepath.Join(candidate, "instance.yaml")); e == nil {
			return Resolve(candidate)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}
	return nil, errors.New("explicit path is not inside a vault with instance.yaml")
}

// ReferenceBranchOrder is the cell's ordered branch preference for repositories without an explicit
// reference branch (default main, then master).
func ReferenceBranchOrder(inst Object) []string {
	out := []string{}
	for _, b := range list(obj(inst["sources"])["reference_branch_order"]) {
		if s := str(b); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{"main", "master"}
	}
	return out
}
