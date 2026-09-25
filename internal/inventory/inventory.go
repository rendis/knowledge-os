// Package inventory classifies configured repositories against observed GitHub refs.
package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
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
	"go.yaml.in/yaml/v3"
)

type object = map[string]any

func obj(v any) object {
	m, _ := v.(map[string]any)
	if m == nil {
		return object{}
	}
	return m
}
func str(v any) string { s, _ := v.(string); return s }
func arr(v any) []any  { a, _ := v.([]any); return a }

var sha12 = regexp.MustCompile(`^[0-9a-f]{12}$`)
var repoName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var scopeRow = regexp.MustCompile("^\\s*\\|\\s*`([^`]+)`\\s*\\|")
var decisions = map[string]string{"no-durable-node": "acknowledged-no-node", "no-documentation-change": "acknowledged-no-change", "review-rejected": "acknowledged-review-rejected", "inspection-limited": "acknowledged-inspection-limited"}
var statuses = []string{"current", "changed", "new", "acknowledged-no-node", "acknowledged-no-change", "acknowledged-review-rejected", "acknowledged-inspection-limited", "container", "archived", "renamed-or-transferred", "missing-candidate", "branch-ambiguous", "invalid-note"}

type github struct {
	Login  string `json:"login"`
	Source string `json:"source"`
	Host   string `json:"host"`
	token  string
}
type scope struct {
	root      string
	prefixes  []string
	approved  map[string]bool
	container string
	instance  object
}

func newScope(root string, instance object) (scope, error) {
	s := scope{root: root, instance: instance, approved: map[string]bool{}}
	for _, v := range arr(obj(instance["sources"])["repo_prefixes"]) {
		if p := strings.TrimRight(str(v), "-"); p != "" {
			s.prefixes = append(s.prefixes, p+"-")
		}
	}
	remote := strings.TrimRight(str(obj(instance["vault"])["remote"]), "/")
	s.container = strings.TrimSuffix(filepath.Base(remote), ".git")
	b, e := readVault(root, "90-Meta/Alcance.md")
	if e != nil && !os.IsNotExist(e) {
		return s, e
	}
	text := string(b)
	if at := strings.Index(text, "## Repositorios fuente"); at >= 0 {
		text = strings.SplitN(text[at+len("## Repositorios fuente"):], "\n## ", 2)[0]
		for _, line := range strings.Split(text, "\n") {
			m := scopeRow.FindStringSubmatch(line)
			if len(m) == 2 && repoName.MatchString(m[1]) && !strings.EqualFold(m[1], s.container) {
				s.approved[m[1]] = true
			}
		}
	}
	return s, nil
}
func (s scope) tracked(name string) bool {
	if s.approved[name] {
		return true
	}
	for _, p := range s.prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
func envToken(token string) []string {
	out := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GH_TOKEN=") && !strings.HasPrefix(v, "GITHUB_TOKEN=") && !strings.HasPrefix(v, "GH_HOST=") {
			out = append(out, v)
		}
	}
	out = append(out, "GH_HOST=github.com")
	if token != "" {
		out = append(out, "GH_TOKEN="+token)
	}
	return out
}
func gh(ctx context.Context, token string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = envToken(token)
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("GitHub CLI operation %s failed (check authentication/access): %w", args[0], e)
	}
	return b, nil
}
func api(ctx context.Context, token, query string, fields ...string) (object, error) {
	args := []string{"api", "graphql", "-f", "query=" + query}
	for _, field := range fields {
		args = append(args, "-f", field)
	}
	b, e := gh(ctx, token, args...)
	if e != nil {
		return nil, e
	}
	var result object
	if e = json.Unmarshal(b, &result); e != nil {
		return nil, e
	}
	if len(arr(result["errors"])) != 0 {
		return nil, errors.New("GitHub GraphQL returned errors")
	}
	return obj(result["data"]), nil
}
func accountToken(ctx context.Context, login string) (string, error) {
	b, e := gh(ctx, "", "auth", "token", "--hostname", "github.com", "--user", login)
	if e != nil {
		return "", e
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", errors.New("empty stored GitHub token")
	}
	return token, nil
}
func probe(ctx context.Context, org, token string) (string, error) {
	data, e := api(ctx, token, `query($org:String!){viewer{login} organization(login:$org){login}}`, "org="+org)
	if e != nil {
		return "", e
	}
	login := str(obj(data["viewer"])["login"])
	if login == "" || str(obj(data["organization"])["login"]) == "" {
		return "", errors.New("GitHub account cannot access organization")
	}
	return login, nil
}
func authenticate(ctx context.Context, org, user string) (github, error) {
	if user != "" {
		token, e := accountToken(ctx, user)
		if e != nil {
			return github{}, e
		}
		login, e := probe(ctx, org, token)
		return github{login, "explicit", "github.com", token}, e
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		login, e := probe(ctx, org, token)
		return github{login, "environment", "github.com", token}, e
	}
	b, e := gh(ctx, "", "auth", "status", "--hostname", "github.com", "--json", "hosts")
	if e != nil {
		return github{}, e
	}
	var data object
	if e = json.Unmarshal(b, &data); e != nil {
		return github{}, e
	}
	accounts := arr(obj(data["hosts"])["github.com"])
	sort.SliceStable(accounts, func(i, j int) bool { return obj(accounts[i])["active"] == true && obj(accounts[j])["active"] != true })
	var found []github
	for _, a := range accounts {
		account := obj(a)
		if account["state"] != "success" {
			continue
		}
		token, e = accountToken(ctx, str(account["login"]))
		if e != nil {
			continue
		}
		login, e := probe(ctx, org, token)
		if e != nil {
			continue
		}
		source := "discovered"
		if account["active"] == true {
			source = "active"
		}
		g := github{login, source, "github.com", token}
		if source == "active" {
			return g, nil
		}
		found = append(found, g)
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		return github{}, errors.New("multiple GitHub accounts can access organization; select --github-user")
	}
	return github{}, errors.New("no authenticated GitHub account can access organization")
}
func loadNotes(root string, s scope) ([]object, error) {
	paths, e := filepath.Glob(filepath.Join(root, "20-Repos", "*", "*.md"))
	if e != nil {
		return nil, e
	}
	out := []object{}
	for _, p := range paths {
		st, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("note is not regular: %s", p)
		}
		relPath, e := filepath.Rel(root, p)
		if e != nil {
			return nil, e
		}
		b, e := readVault(root, relPath)
		if e != nil {
			return nil, e
		}
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		var fields object
		if strings.HasPrefix(text, "---\n") {
			parts := strings.SplitN(text[4:], "\n---", 2)
			if len(parts) < 2 {
				return nil, fmt.Errorf("unclosed frontmatter: %s", p)
			}
			if e = yaml.Unmarshal([]byte(parts[0]), &fields); e != nil {
				return nil, e
			}
		}
		aliases := arr(fields["aliases"])
		if a, ok := fields["aliases"].(string); ok {
			aliases = []any{a}
		}
		name := ""
		for _, a := range aliases {
			if s.tracked(str(a)) {
				name = str(a)
				break
			}
		}
		stem := strings.TrimSuffix(filepath.Base(p), ".md")
		valid := name != "" && sha12.MatchString(str(fields["commit-analizado"])) && config.ValidBranch(str(fields["rama-analizada"]))
		if name == "" {
			name = stem
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, object{"note": stem, "path": filepath.ToSlash(rel), "repo": name, "recorded_sha": fields["commit-analizado"], "recorded_branch": fields["rama-analizada"], "valid": valid})
	}
	return out, nil
}
func acknowledgements(root string, s scope) ([]object, error) {

	b, e := readVault(root, "90-Meta/.sync-acknowledgements.json")
	if os.IsNotExist(e) {
		return []object{}, nil
	}
	if e != nil {
		return nil, e
	}
	var data struct {
		Version      int                 `json:"version"`
		Repositories []map[string]string `json:"repositories"`
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&data); e != nil {
		return nil, e
	}
	if data.Version != 1 || data.Repositories == nil {
		return nil, errors.New("invalid sync acknowledgements")
	}
	var tail any
	if dec.Decode(&tail) != io.EOF {
		return nil, errors.New("trailing JSON in acknowledgements")
	}
	// Decode independently to reject duplicate object keys before trusting cursors.
	if e = rejectDuplicateJSON(b); e != nil {
		return nil, e
	}
	prev := ""
	out := []object{}
	for _, r := range data.Repositories {
		if len(r) != 5 || !s.tracked(r["repository"]) || r["repository"] <= prev || !config.ValidBranch(r["branch"]) || !sha12.MatchString(r["analyzed_sha"]) || decisions[r["decision"]] == "" {
			return nil, errors.New("invalid sync acknowledgement entry")
		}
		date, e := time.Parse("2006-01-02", r["analysis_date"])
		if e != nil || date.Format("2006-01-02") != r["analysis_date"] {
			return nil, errors.New("invalid acknowledgement date")
		}
		m := object{}
		for k, v := range r {
			m[k] = v
		}
		out = append(out, m)
		prev = r["repository"]
	}
	return out, nil
}
func rejectDuplicateJSON(b []byte) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("JSON nesting exceeds 64 levels")
		}
		tok, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := tok.(json.Delim); ok {
			if delim == '{' {
				seen := map[string]bool{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := key.(string)
					if !ok || seen[s] {
						return errors.New("duplicate or invalid JSON key")
					}
					seen[s] = true
					if e = value(depth + 1); e != nil {
						return e
					}
				}
			} else if delim == '[' {
				for d.More() {
					if e = value(depth + 1); e != nil {
						return e
					}
				}
			} else {
				return errors.New("invalid JSON delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	return value(0)
}

const query = `query($org:String!,$cursor:String){organization(login:$org){repositories(first:100,after:$cursor){nodes{name isArchived isFork url defaultBranchRef{name} main:ref(qualifiedName:"refs/heads/main"){name target{oid}} master:ref(qualifiedName:"refs/heads/master"){name target{oid}}} pageInfo{hasNextPage endCursor}}}}`

func remoteRepos(ctx context.Context, s scope, org string, g github) ([]object, error) {
	out := []object{}
	cursor := ""
	seen := map[string]bool{}
	for {
		fields := []string{"org=" + org}
		if cursor != "" {
			fields = append(fields, "cursor="+cursor)
		}
		data, e := api(ctx, g.token, query, fields...)
		if e != nil {
			return nil, e
		}
		organization := obj(data["organization"])
		if len(organization) == 0 {
			return nil, errors.New("organization absent from GraphQL result")
		}
		conn := obj(organization["repositories"])
		for _, item := range arr(conn["nodes"]) {
			r := obj(item)
			name := str(r["name"])
			if !s.tracked(name) {
				continue
			}
			branches := config.ReferenceBranchOrder(s.instance)
			refs := map[string]any{"main": r["main"], "master": r["master"]}
			if configured := str(obj(obj(s.instance["sources"])["reference_branches"])[name]); configured != "" {
				branches = []string{configured}
			}
			for _, b := range branches {
				if _, known := refs[b]; known {
					continue // main and master come with the organization query
				}
				data, e := api(ctx, g.token, `query($org:String!,$repo:String!,$ref:String!){repository(owner:$org,name:$repo){ref(qualifiedName:$ref){name target{oid}}}}`, "org="+org, "repo="+name, "ref=refs/heads/"+b)
				if e != nil {
					return nil, e
				}
				refs[b] = obj(data["repository"])["ref"]
			}
			r["branch"] = nil
			r["sha"] = nil
			for _, branch := range branches {
				oid := str(obj(obj(refs[branch])["target"])["oid"])
				if len(oid) >= 12 {
					r["branch"] = branch
					r["sha"] = oid[:12]
					break
				}
			}
			out = append(out, r)
		}
		page := obj(conn["pageInfo"])
		if page["hasNextPage"] != true {
			break
		}
		cursor = str(page["endCursor"])
		if cursor == "" || seen[cursor] {
			return nil, errors.New("invalid or repeated pagination cursor")
		}
		seen[cursor] = true
	}
	return out, nil
}
func classify(s scope, notes, acks, repos []object) ([]object, error) {
	byNote := map[string]object{}
	byAck := map[string]object{}
	byRepo := map[string]bool{}
	for _, n := range notes {
		if s.tracked(str(n["repo"])) {
			name := str(n["repo"])
			if byNote[name] != nil {
				return nil, fmt.Errorf("duplicate repository notes: %s", name)
			}
			byNote[name] = n
		}
	}
	for _, a := range acks {
		name := str(a["repository"])
		byAck[name] = a
		n := byNote[name]
		if n != nil && (a["decision"] == "no-durable-node" || (a["decision"] != "no-documentation-change" && n["recorded_branch"] == a["branch"] && n["recorded_sha"] == a["analyzed_sha"])) {
			return nil, errors.New("incompatible note and acknowledgement")
		}
		if n == nil && a["decision"] == "no-documentation-change" {
			return nil, errors.New("no-change acknowledgement requires a note")
		}
	}
	out := []object{}
	for _, r := range repos {
		name := str(r["name"])
		byRepo[name] = true
		n, a := byNote[name], byAck[name]
		status, details := "new", "no matching vault note; prior cursor is stale"
		var note, recorded any
		if n != nil {
			note = n["note"]
			recorded = n["recorded_sha"]
		} else if a != nil {
			recorded = a["analyzed_sha"]
		}
		containerNote := filepath.Join(s.root, "90-Meta/Repositorio del vault.md")
		_, containerErr := os.Stat(containerNote)
		switch {
		case name == s.container && containerErr != nil:
			status, details = "invalid-note", "missing container note: 90-Meta/Repositorio del vault.md"
		case n != nil && n["valid"] != true:
			status, details = "invalid-note", "missing/invalid alias, SHA or reference branch"
		case r["isArchived"] == true:
			status, details = "archived", "repository is archived"
		case r["branch"] == nil:
			status, details = "branch-ambiguous", "no configured reference branch or main/master fallback"
		case name == s.container:
			status, details = "container", "vault container documented in Meta; SHA tracking is not self-referential"
			note = "Repositorio del vault"
			recorded = nil
		case a != nil && a["branch"] == r["branch"] && a["analyzed_sha"] == r["sha"]:
			status, details = decisions[str(a["decision"])], "explicit synchronization outcome at observed branch SHA"
		case n != nil && n["recorded_sha"] == r["sha"] && n["recorded_branch"] == r["branch"]:
			status, details = "current", "recorded SHA matches reference branch"
		case n != nil:
			status, details = "changed", "reference branch or SHA differs from note"
		}
		out = append(out, object{"note": note, "repo": name, "status": status, "branch": r["branch"], "recorded_sha": recorded, "remote_sha": r["sha"], "details": details})
	}
	for _, n := range notes {
		if byRepo[str(n["repo"])] {
			continue
		}
		status := "missing-candidate"
		if n["valid"] != true {
			status = "invalid-note"
		}
		out = append(out, object{"note": n["note"], "repo": n["repo"], "status": status, "branch": n["recorded_branch"], "recorded_sha": n["recorded_sha"], "remote_sha": nil, "details": "repository absent from organization inventory"})
	}
	for _, a := range acks {
		if byRepo[str(a["repository"])] {
			continue
		}
		out = append(out, object{"note": nil, "repo": a["repository"], "status": "missing-candidate", "branch": a["branch"], "recorded_sha": a["analyzed_sha"], "remote_sha": nil, "details": "repository absent from organization inventory"})
	}
	sort.Slice(out, func(i, j int) bool {
		return str(out[i]["repo"])+str(out[i]["note"]) < str(out[j]["repo"])+str(out[j]["note"])
	})
	return out, nil
}
func Run(args []string, out io.Writer) error {
	f := flag.NewFlagSet("inventory", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	root := f.String("vault", ".", "vault")
	org := f.String("org", "", "GitHub organization")
	user := f.String("github-user", "", "stored GitHub account")
	filter := f.String("repo", "", "exact note/repository name")
	format := f.String("format", "json", "json or markdown")
	if e := f.Parse(args); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return errors.New("unexpected inventory arguments")
	}
	if *format != "json" && *format != "markdown" {
		return errors.New("format must be json or markdown")
	}
	r, e := filepath.Abs(*root)
	if e != nil {
		return e
	}
	inst, e := config.LoadInstance(r)
	if e != nil {
		return e
	}
	s, e := newScope(r, inst)
	if e != nil {
		return e
	}
	if *org == "" {
		*org = str(obj(inst["sources"])["github_org"])
	}
	if *org == "" {
		return errors.New("sources.github_org is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	g, e := authenticate(ctx, *org, *user)
	if e != nil {
		return e
	}
	notes, e := loadNotes(r, s)
	if e != nil {
		return e
	}
	acks, e := acknowledgements(r, s)
	if e != nil {
		return e
	}
	repos, e := remoteRepos(ctx, s, *org, g)
	if e != nil {
		return e
	}
	records, e := classify(s, notes, acks, repos)
	if e != nil {
		return e
	}
	// Resolve redirects only for identities missing from the organization result.
	for _, record := range records {
		if record["status"] != "missing-candidate" {
			continue
		}
		b, e := gh(ctx, g.token, "repo", "view", *org+"/"+str(record["repo"]), "--json", "nameWithOwner,url")
		if e != nil {
			continue
		}
		var data object
		if json.Unmarshal(b, &data) == nil {
			actual := str(data["nameWithOwner"])
			if actual != "" && !strings.EqualFold(actual, *org+"/"+str(record["repo"])) {
				record["status"] = "renamed-or-transferred"
				record["details"] = actual
			}
		}
	}
	if *filter != "" {
		selected := []object{}
		for _, record := range records {
			match := strings.EqualFold(*filter, str(record["repo"])) || strings.EqualFold(*filter, str(record["note"]))
			for _, prefix := range s.prefixes {
				if strings.HasPrefix(str(record["repo"]), prefix) && strings.EqualFold(strings.TrimPrefix(str(record["repo"]), prefix), *filter) {
					match = true
				}
			}
			if match {
				selected = append(selected, record)
			}
		}
		if len(selected) == 0 {
			return errors.New("--repo did not match a note or repository")
		}
		records = selected
	}
	summary := map[string]int{}
	for _, status := range statuses {
		summary[status] = 0
	}
	for _, r := range records {
		summary[str(r["status"])]++
	}
	payload := object{"generated_at": time.Now().UTC().Format(time.RFC3339), "org": *org, "github": g, "summary": summary, "repos": records}
	if *format == "json" {
		return json.NewEncoder(out).Encode(payload)
	}
	fmt.Fprintf(out, "# Vault inventory\n\nOrganization: %s; account: %s\n\n| Repository | Note | Status | Branch | Recorded | Remote |\n|---|---|---|---|---|---|\n", *org, g.Login)
	for _, record := range records {
		var fields []string
		for _, key := range []string{"repo", "note", "status", "branch", "recorded_sha", "remote_sha"} {
			fields = append(fields, strings.ReplaceAll(str(record[key]), "|", "\\|"))
		}
		fmt.Fprintln(out, "| "+strings.Join(fields, " | ")+" |")
	}
	return nil
}

func readVault(root, path string) ([]byte, error) {
	r, e := os.OpenRoot(root)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	f, e := r.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("expected regular vault file")
	}
	b, e := io.ReadAll(io.LimitReader(f, 4*1024*1024+1))
	if e != nil {
		return nil, e
	}
	if len(b) > 4*1024*1024 {
		return nil, errors.New("vault metadata file exceeds 4 MiB")
	}
	return b, nil
}
