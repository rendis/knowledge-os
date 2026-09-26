package cell

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"knowledge-os/internal/config"
)

// Options are the answers onboarding needs; a flag skips its question.
type Options struct {
	CellName, Purpose, GithubOrg, Locale, EvidenceProfile, VaultRemote      string
	Systems, Trackers, Adapters, Platforms, RepoPrefixes, ReferenceBranches []string
	DisableTopics, Yes, Force                                               bool
}

var errCancelled = errors.New("Initialization cancelled: input interrupted.")

// asker reads one answer per question from in and writes the questions to out (stderr), so stdout
// stays JSON. An end of input or an interrupt cancels before anything is written.
type asker struct {
	ctx   context.Context
	lines chan string
	out   io.Writer
	yes   bool
}

func newAsker(ctx context.Context, in io.Reader, out io.Writer, yes bool) *asker {
	a := &asker{ctx: ctx, lines: make(chan string), out: out, yes: yes}
	if yes {
		return a
	}
	go func() {
		r := bufio.NewReader(in)
		for {
			line, e := r.ReadString('\n')
			if e != nil && line == "" {
				close(a.lines)
				return
			}
			a.lines <- strings.TrimSpace(line)
		}
	}()
	return a
}

// ask returns def with --yes; otherwise the answer, or def when it is empty.
func (a *asker) ask(label, def string) (string, error) {
	if a.yes {
		return def, nil
	}
	suffix := ""
	if def != "" {
		suffix = " [" + def + "]"
	}
	fmt.Fprintf(a.out, "%s%s: ", label, suffix)
	select {
	case <-a.ctx.Done():
		return "", errCancelled
	case line, ok := <-a.lines:
		if !ok {
			return "", errCancelled
		}
		if line == "" {
			return def, nil
		}
		return line, nil
	}
}

func listed(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var nonKebab = regexp.MustCompile(`[^a-z0-9]+`)

// parseSystem reads a system name (its id is derived) or id:Name.
func parseSystem(value string) map[string]any {
	id, name, colon := strings.Cut(strings.TrimSpace(value), ":")
	if !colon {
		name = id
		id = strings.Trim(nonKebab.ReplaceAllString(strings.ToLower(id), "-"), "-")
	}
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	if id == "" {
		return nil
	}
	return map[string]any{"id": id, "name": name, "aliases": []any{id}}
}

var trackerHosts = [][2]string{{"atlassian.net", "jira"}, {"jira", "jira"}, {"github.com", "github"}, {"gitlab", "gitlab"},
	{"dev.azure.com", "azure-devops"}, {"visualstudio.com", "azure-devops"}, {"linear.app", "linear"}}

// parseTracker reads a tracker URL (its provider comes from the host) or id:provider:https://url.
func parseTracker(value string) (map[string]any, error) {
	value = strings.TrimSpace(value)
	id, provider, raw := "", "", value
	if strings.HasPrefix(value, "https://") {
		u, e := url.Parse(value)
		if e != nil {
			return nil, fmt.Errorf("invalid tracker URL: %s", value)
		}
		host := u.Hostname()
		provider = strings.Split(host, ".")[0]
		for _, h := range trackerHosts {
			if strings.Contains(host, h[0]) {
				provider = h[1]
				break
			}
		}
		if provider == "" {
			provider = "tracker"
		}
		id = provider
	} else {
		var rest string
		var ok1, ok2 bool
		id, rest, ok1 = strings.Cut(value, ":")
		provider, raw, ok2 = strings.Cut(rest, ":")
		if !ok1 || !ok2 || id == "" || provider == "" || raw == "" {
			return nil, errors.New("a tracker is an https:// URL, or id:provider:https://url")
		}
	}
	canonical, e := config.CanonicalHTTPS(strings.TrimSpace(raw))
	if e != nil {
		return nil, fmt.Errorf("tracker URL must be a credential-free HTTPS URL: %s", raw)
	}
	return map[string]any{"id": strings.TrimSpace(id), "provider": strings.ToLower(strings.TrimSpace(provider)), "url": canonical}, nil
}

// buildInstance asks the cell's questions a flag did not answer: who the cell is, where its code is,
// where it runs. The evidence profile and adapters keep their defaults unless passed.
func buildInstance(o Options, a *asker) (map[string]any, error) {
	interactive := !o.Yes
	answer := func(given, label, def string) (string, error) {
		if given != "" {
			return given, nil
		}
		return a.ask(label, def)
	}
	name, e := answer(o.CellName, "Cell name", "Cell")
	if e != nil {
		return nil, e
	}
	purpose, e := answer(o.Purpose, "What the cell owns, in 1-3 sentences", "Describe this cell.")
	if e != nil {
		return nil, e
	}
	rawSystems := o.Systems
	if len(rawSystems) == 0 {
		if o.Yes {
			return nil, errors.New("--system Name (or id:Name) is required with --yes")
		}
		s, e := a.ask("Systems it owns, comma-separated (e.g. Orders, Payments)", "Platform")
		if e != nil {
			return nil, e
		}
		rawSystems = listed(s)
	}
	systems := []any{}
	for _, s := range rawSystems {
		if v := parseSystem(s); v != nil {
			systems = append(systems, v)
		}
	}
	org := o.GithubOrg
	if org == "" && interactive {
		if org, e = a.ask("GitHub organization of its repositories (empty to skip)", ""); e != nil {
			return nil, e
		}
	}
	prefixes := o.RepoPrefixes
	if len(prefixes) == 0 && interactive {
		s, e := a.ask("Repository name prefixes, comma-separated (e.g. APP01234-; empty to list them later)", "")
		if e != nil {
			return nil, e
		}
		prefixes = listed(s)
	}
	branches := o.ReferenceBranches
	if len(branches) == 0 {
		s, e := a.ask("Reference branches, in order: the first that exists in each repository is read (main, master, develop…)", "main,master")
		if e != nil {
			return nil, e
		}
		branches = listed(s)
	}
	platforms := o.Platforms
	if len(platforms) == 0 && interactive {
		s, e := a.ask("Clouds the systems run on (gcp, aws, azure; empty for none)", "")
		if e != nil {
			return nil, e
		}
		platforms = listed(s)
	}
	rawTrackers := o.Trackers
	if len(rawTrackers) == 0 && interactive {
		s, e := a.ask("Issue tracker URLs, comma-separated (empty for none)", "")
		if e != nil {
			return nil, e
		}
		rawTrackers = listed(s)
	}
	trackers := []any{}
	for _, t := range rawTrackers {
		v, e := parseTracker(t)
		if e != nil {
			return nil, e
		}
		trackers = append(trackers, v)
	}
	locale, e := answer(o.Locale, "Language of the notes (es|en)", "es")
	if e != nil {
		return nil, e
	}
	types := []any{}
	for _, t := range config.DefaultTypes() {
		if !(o.DisableTopics && t == "topic") {
			types = append(types, t)
		}
	}
	profile := o.EvidenceProfile
	if profile == "" {
		profile = "production-gate"
	}
	return map[string]any{
		"version":  1,
		"cell":     map[string]any{"name": strings.TrimSpace(name), "purpose": strings.TrimSpace(purpose)},
		"systems":  systems,
		"trackers": trackers,
		"vault":    map[string]any{"remote": strings.TrimSpace(o.VaultRemote)},
		"sources": map[string]any{
			"github_org":             strings.TrimSpace(org),
			"repo_prefixes":          anyList(prefixes),
			"reference_branch_order": anyList(branches),
			"schema_repository":      map[string]any{"remote": "", "note": ""},
		},
		"graph":    map[string]any{"enabled_types": types},
		"evidence": map[string]any{"profile": profile},
		"platform": map[string]any{"providers": anyList(platforms)},
		"adapters": anyList(o.Adapters),
		"locale":   map[string]any{"notes": locale},
	}, nil
}

func anyList(s []string) []any {
	out := make([]any, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}

func q(v any) string {
	b, _ := json.Marshal(fmt.Sprint(v))
	return string(b)
}

func joined(list []any, quote bool) string {
	parts := make([]string, 0, len(list))
	for _, v := range list {
		if quote {
			parts = append(parts, q(v))
		} else {
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func field(m any, key string) any {
	v, _ := m.(map[string]any)
	return v[key]
}

func items(v any) []any { a, _ := v.([]any); return a }

// dumpInstance writes instance.yaml with a stable layout.
func dumpInstance(m map[string]any) string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Cell identity. Kernel update never overwrites this file.")
	line("version: 1")
	line("")
	line("cell:")
	line("  name: %s", q(field(m["cell"], "name")))
	line("  purpose: %s", q(field(m["cell"], "purpose")))
	line("")
	line("systems:")
	for _, s := range items(m["systems"]) {
		line("  - id: %s", q(field(s, "id")))
		line("    name: %s", q(field(s, "name")))
		line("    aliases: %s", joined(items(field(s, "aliases")), true))
	}
	line("")
	if trackers := items(m["trackers"]); len(trackers) > 0 {
		line("trackers:")
		for _, t := range trackers {
			line("  - id: %s", q(field(t, "id")))
			line("    provider: %s", q(field(t, "provider")))
			line("    url: %s", q(field(t, "url")))
		}
	} else {
		line("trackers: []")
	}
	line("")
	line("vault:")
	line("  remote: %s", q(field(m["vault"], "remote")))
	src := m["sources"]
	line("sources:")
	line("  github_org: %s", q(field(src, "github_org")))
	line("  repo_prefixes: %s", joined(items(field(src, "repo_prefixes")), true))
	line("  reference_branch_order: %s", joined(items(field(src, "reference_branch_order")), true))
	line("  schema_repository:")
	line("    remote: %s", q(field(field(src, "schema_repository"), "remote")))
	line("    note: %s", q(field(field(src, "schema_repository"), "note")))
	line("platform:")
	line("  providers: %s", joined(items(field(m["platform"], "providers")), false))
	line("graph:")
	line("  enabled_types:")
	for _, t := range items(field(m["graph"], "enabled_types")) {
		line("    - %v", t)
	}
	line("evidence:")
	line("  profile: %v", field(m["evidence"], "profile"))
	line("adapters: %s", joined(items(m["adapters"]), false))
	line("locale:")
	line("  notes: %v", field(m["locale"], "notes"))
	return b.String()
}

// validate checks the text that will be written, with the same validator every kos command uses.
func validate(text string) error {
	m := config.Object{}
	if e := yaml.Unmarshal([]byte(text), &m); e != nil {
		return fmt.Errorf("instance.yaml: %w", e)
	}
	return config.ValidateInstance(m)
}
