// Package discover extracts connection facts from source repositories, their configuration
// and the platform, deterministically first. Judgments that code cannot make are asked once
// (Jev when TYPESAFE_API_KEY is set, otherwise the agent) and stored in the cell vault.
package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"documentation-vault/internal/config"
)

const Help = `discover COMMAND --vault PATH [options]
  run        [--repo NAME ...] [--at head|note] [--classify auto|off]
             Scan repositories at an exact commit, apply stored judgments and platform
             snapshots, write facts and the comparison with notes. With TYPESAFE_API_KEY
             set, pending judgments are answered automatically (--classify auto).
  questions  [--kind dependency|config_key|config_entry] [--limit N]
             Pending judgments as JSON for the agent to answer (from the last run).
  answer     --file ANSWERS.json   Record agent answers: [{"id":..,"choice":..,"confidence":0..1}]
  platform   [--provider NAME] [--scope ID ...] [--referenced] [--dry-run] | --record FILE
             Capture read-only messaging listings (topics, subscriptions, queues) with the
             provider's own CLI and the developer's login. Providers: gcp (project id),
             aws (<account>/<region>), azure (subscription id). --provider may be omitted
             when the cell configures one (platform.providers). --referenced uses the
             scopes of the configured providers named by configuration in the last run.
             The providers are a floor: --record stores what the agent read elsewhere
             (any service, cluster or host) with its command, so facts and claims use it.
  report     [--repo NAME]   Last run summary, or one repository's facts.
  check      --note PATH [--note PATH ...] [--repo NAME] [--semantic]
             Gates for a repository note: G1 source anchors resolve at their commit and
             the identifiers they name are in the cited lines; G2 every connector and
             configured resource is evidenced or addressed; freshness of cited files;
             --semantic asks Jev whether each cited sentence is supported (review aid).
             A candidate outside the vault is matched to its repository by its aliases.
  corrections  Note relations the last run could not support, as correction tasks.
  claims     --file DRAFT|-   Check a draft answer before delivery: resource names that no
             fact, platform snapshot or note knows, and relations discovery contradicts.
Facts and questions are local (.agents/state/discovery). Judgments and platform snapshots
are versioned under 90-Meta/discovery/. All output is JSON.`

const stateRel = ".agents/state/discovery"

type options struct {
	vault, at, classify, kind, file string
	provider, record                string
	repos, scopes, notes            []string
	limit                           int
	referenced, dryRun, semantic    bool
}

func parse(args []string) (string, options, error) {
	o := options{at: "head", classify: "auto", limit: 200}
	if len(args) == 0 {
		return "", o, errors.New("discover requires a command; use --help")
	}
	cmd := args[0]
	for i := 1; i < len(args); i++ {
		a := args[i]
		val := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", a)
			}
			i++
			return args[i], nil
		}
		var e error
		var v string
		switch a {
		case "--vault":
			o.vault, e = val()
		case "--repo":
			v, e = val()
			o.repos = append(o.repos, v)
		case "--note":
			v, e = val()
			o.notes = append(o.notes, v)
		case "--semantic":
			o.semantic = true
		case "--scope":
			v, e = val()
			o.scopes = append(o.scopes, v)
		case "--provider":
			o.provider, e = val()
		case "--record":
			o.record, e = val()
		case "--at":
			o.at, e = val()
		case "--classify":
			o.classify, e = val()
		case "--kind":
			o.kind, e = val()
		case "--file":
			o.file, e = val()
		case "--limit":
			v, e = val()
			if e == nil {
				_, e = fmt.Sscanf(v, "%d", &o.limit)
			}
		case "--referenced":
			o.referenced = true
		case "--dry-run":
			o.dryRun = true
		default:
			return "", o, fmt.Errorf("unknown option %s", a)
		}
		if e != nil {
			return "", o, e
		}
	}
	if o.vault == "" {
		return "", o, errors.New("--vault is required")
	}
	r, e := config.Resolve(o.vault)
	if e != nil {
		return "", o, e
	}
	o.vault, _ = r["vault_root"].(string)
	if o.at != "head" && o.at != "note" {
		return "", o, errors.New("--at must be head or note")
	}
	return cmd, o, nil
}

func emit(out io.Writer, v any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(v)
}

func writeState(vault, name string, v any) error {
	p := filepath.Join(vault, stateRel, name)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", " ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

func readState(vault, name string, v any) error {
	b, e := os.ReadFile(filepath.Join(vault, stateRel, name))
	if os.IsNotExist(e) {
		return fmt.Errorf("no discovery run found; run `vaultctl discover run --vault %s` first", vault)
	}
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}

func Run(args []string, out io.Writer) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			_, e := io.WriteString(out, Help+"\n")
			return e
		}
	}
	cmd, o, e := parse(args)
	if e != nil {
		return e
	}
	switch cmd {
	case "run":
		return runDiscovery(o, out)
	case "questions":
		return listQuestions(o, out)
	case "answer":
		return answerQuestions(o, out)
	case "platform":
		if o.record != "" {
			return recordObservation(o, out)
		}
		return capturePlatform(o, out)
	case "report":
		return showReport(o, out)
	case "check":
		return runCheck(o, out, o.semantic)
	case "corrections":
		return listCorrections(o, out)
	case "claims":
		return runClaims(o, out)
	}
	return fmt.Errorf("unknown discover command %q", cmd)
}

type runReport struct {
	Vault            string            `json:"vault"`
	GeneratedAt      string            `json:"generated_at"`
	Repositories     int               `json:"repositories"`
	Scanned          []string          `json:"scanned"`
	Failed           []string          `json:"failed"`
	PendingQuestions map[string]int    `json:"pending_questions"`
	Acceleration     map[string]any    `json:"acceleration"`
	Classified       map[string]any    `json:"classified_this_run,omitempty"`
	PlatformScopes   []string          `json:"platform_scopes_referenced"`
	PlatformCaptured map[string]string `json:"platform_captured"`
	PlatformObserved int               `json:"platform_observed_names,omitempty"`
	// CredentialsInSources lists credentials versioned in a source (file and key; the value is never read
	// back): the source owners' to fix, reported to the user, never copied into the vault.
	CredentialsInSources []map[string]string `json:"credentials_in_sources,omitempty"`
	Pending              map[string]int      `json:"pending_items"`
	Comparison           map[string]int      `json:"comparison"`
	Duration             string              `json:"duration"`
}

func runDiscovery(o options, out io.Writer) error {
	start := time.Now()
	only := map[string]bool{}
	for _, r := range o.repos {
		only[r] = true
	}
	inputs, e := discoverRepositories(o.vault, only)
	if e != nil {
		return e
	}
	scans := []*repoScan{}
	failed := []string{}
	for _, in := range inputs {
		if c := noteCommit(o.vault, in.Note); o.at == "note" && c != "" {
			in.Ref, in.RefNote, in.RefErr = c, "", ""
		}
		if in.RefErr != "" {
			failed = append(failed, in.Name+": "+in.RefErr)
			continue
		}
		s, e := scanRepository(in)
		if e != nil {
			failed = append(failed, in.Name+": "+e.Error())
			continue
		}
		s.in.Commit, _ = resolveCommit(in.Path, in.Ref)
		scans = append(scans, s)
	}
	if len(o.repos) > 0 {
		resolveLibraries(append(scans, libraryContext(o.vault, scans)...))
	} else {
		resolveLibraries(scans)
	}
	st, e := loadStore(o.vault)
	if e != nil {
		return e
	}
	if st.dropped > 0 {
		if e := st.save(o.vault); e != nil {
			return e
		}
	}
	ix, e := loadPlatform(o.vault)
	if e != nil {
		return e
	}
	a := &assembly{scans: scans, st: st, platform: ix, providers: configuredProviders(o.vault)}
	classified := map[string]any{}
	if jev := newJev(); jev != nil && o.classify == "auto" {
		// Dependencies and keys first: entry questions depend on key judgments.
		for round := 0; round < 3; round++ {
			qs := a.pendingQuestions()
			if len(qs) == 0 {
				break
			}
			done, failures := jev.answerAll(context.Background(), st, qs)
			classified[fmt.Sprintf("round_%d", round+1)] = map[string]any{"asked": len(qs), "answered": done, "failures": len(failures)}
			if e := st.save(o.vault); e != nil {
				return e
			}
			if done == 0 {
				break
			}
		}
		classified["jev_calls"], classified["jev_input_tokens"] = jev.calls, jev.tokens
	}
	facts := a.facts()
	qs := a.pendingQuestions()
	if e := writeState(o.vault, "questions.json", qs); e != nil {
		return e
	}
	for _, f := range facts {
		if e := writeState(o.vault, filepath.Join("facts", f.Repo+".json"), f); e != nil {
			return e
		}
	}
	cmp, e := compareNotes(o.vault, facts)
	if e != nil {
		return e
	}
	if e := writeState(o.vault, "comparison.json", cmp); e != nil {
		return e
	}
	gaps, e := cellGaps(o.vault, facts)
	if e != nil {
		return e
	}
	if e := writeState(o.vault, "gaps.json", gaps); e != nil {
		return e
	}
	rep := runReport{Vault: o.vault, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Repositories: len(inputs), Failed: failed,
		PendingQuestions: map[string]int{}, Acceleration: accelerationStatus(), PlatformScopes: a.platformScopes(configuredProviders(o.vault)),
		PlatformCaptured: map[string]string{}, Pending: map[string]int{}, Comparison: map[string]int{}}
	if len(classified) > 0 {
		rep.Classified = classified
	}
	for _, s := range scans {
		rep.Scanned = append(rep.Scanned, s.in.Name)
	}
	for _, q := range qs {
		rep.PendingQuestions[q.Kind]++
	}
	for p, s := range a.platform.scopes {
		rep.PlatformCaptured[p] = s.Status
	}
	observed := map[string]bool{}
	for _, refs := range a.platform.observed {
		for _, r := range refs {
			observed[r.scope+" "+r.name] = true
		}
	}
	rep.PlatformObserved = len(observed)
	rep.CredentialsInSources = credentialsInSources(scans)
	for _, f := range facts {
		for _, p := range f.Pending {
			rep.Pending[p.Kind]++
		}
	}
	for _, c := range cmp {
		rep.Comparison["notes_compared"]++
		rep.Comparison["supported_relations"] += len(c.Supported)
		rep.Comparison["discrepancies"] += len(c.Discrepancies)
		rep.Comparison["undocumented_resources"] += len(c.Undocumented)
	}
	rep.Comparison["resources_without_topic_note"] = len(gaps)
	rep.Duration = time.Since(start).Round(time.Millisecond).String()
	if e := writeState(o.vault, "report.json", rep); e != nil {
		return e
	}
	return emit(out, rep)
}

func pendingMap(o options) (map[string]question, []question, error) {
	var qs []question
	if e := readState(o.vault, "questions.json", &qs); e != nil {
		return nil, nil, e
	}
	st, e := loadStore(o.vault)
	if e != nil {
		return nil, nil, e
	}
	m := map[string]question{}
	open := []question{}
	for _, q := range qs {
		if _, done := st.table(q.Kind)[q.ID]; done {
			continue
		}
		if o.kind != "" && q.Kind != o.kind {
			continue
		}
		m[q.ID] = q
		open = append(open, q)
	}
	return m, open, nil
}

func listQuestions(o options, out io.Writer) error {
	_, open, e := pendingMap(o)
	if e != nil {
		return e
	}
	total := len(open)
	if o.limit > 0 && len(open) > o.limit {
		open = open[:o.limit]
	}
	return emit(out, map[string]any{"pending": total, "returned": len(open), "answer_with": "vaultctl discover answer --vault <VAULT> --file answers.json", "answer_format": `[{"id":"<question id>","choice":"<one option>","confidence":0.0-1.0}]`, "questions": open, "acceleration": accelerationStatus()})
}

func answerQuestions(o options, out io.Writer) error {
	if o.file == "" {
		return errors.New("--file is required")
	}
	pending, _, e := pendingMap(o)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(o.file)
	if e != nil {
		return e
	}
	var answers []map[string]any
	if e := json.Unmarshal(b, &answers); e != nil {
		return fmt.Errorf("answers must be a JSON array: %w", e)
	}
	st, e := loadStore(o.vault)
	if e != nil {
		return e
	}
	n, e := recordAnswers(st, pending, answers, "agent")
	if e != nil {
		return e
	}
	if e := st.save(o.vault); e != nil {
		return e
	}
	return emit(out, map[string]any{"recorded": n, "remaining": len(pending) - n, "next": "vaultctl discover run --vault <VAULT> --classify off"})
}

// configuredProviders reads platform.providers from the cell's instance.yaml.
func configuredProviders(vault string) []string {
	inst, e := config.LoadInstance(vault)
	if e != nil {
		return nil
	}
	return config.PlatformProviders(inst)
}

func capturePlatform(o options, out io.Writer) error {
	configured := configuredProviders(o.vault)
	keys := []string{}
	if len(o.scopes) > 0 {
		provider := o.provider
		if provider == "" && len(configured) == 1 {
			provider = configured[0]
		}
		p := providers[provider]
		if p == nil {
			return fmt.Errorf("--provider must be one of %s", strings.Join(ProviderNames(), ", "))
		}
		for _, sc := range o.scopes {
			if !p.validScope(sc) {
				return fmt.Errorf("%q is not a %s scope (gcp: project id; aws: <account>/<region>; azure: subscription id)", sc, provider)
			}
			keys = append(keys, scopeKey(provider, sc))
		}
	}
	if o.referenced {
		if len(configured) == 0 {
			return errors.New("no platform provider configured: list the cell's clouds in instance.yaml platform.providers")
		}
		var rep runReport
		if e := readState(o.vault, "report.json", &rep); e != nil {
			return e
		}
		for _, k := range rep.PlatformScopes {
			if n, _, _ := strings.Cut(k, ":"); o.provider == "" || n == o.provider {
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	uniq := []string{}
	for i, k := range keys {
		if i == 0 || k != keys[i-1] {
			uniq = append(uniq, k)
		}
	}
	if len(uniq) == 0 {
		return errors.New("no scope selected; pass --scope or --referenced")
	}
	if o.dryRun {
		cmds := map[string]string{}
		for _, k := range uniq {
			n, _, _ := strings.Cut(k, ":")
			cmds[n] = providers[n].commands()
		}
		return emit(out, map[string]any{"would_capture": uniq, "commands": cmds})
	}
	result := map[string]string{}
	for _, k := range uniq {
		n, sc, _ := strings.Cut(k, ":")
		s := providers[n].capture(sc)
		if e := saveSnapshot(o.vault, s); e != nil {
			return e
		}
		result[k] = s.Status
		if s.Status != "ok" {
			if b, e := os.ReadFile(snapshotPath(o.vault, n, sc)); e == nil && strings.Contains(string(b), `"refresh_failed"`) {
				result[k] = s.Status + " (previous snapshot kept)"
			}
		}
	}
	return emit(out, map[string]any{"captured": result, "stored_in": platformRel, "next": "vaultctl discover run --vault <VAULT>"})
}

func showReport(o options, out io.Writer) error {
	if len(o.repos) == 1 {
		var f repoFacts
		if e := readState(o.vault, filepath.Join("facts", o.repos[0]+".json"), &f); e != nil {
			return e
		}
		var cmp []comparison
		_ = readState(o.vault, "comparison.json", &cmp)
		for _, c := range cmp {
			if strings.EqualFold(c.Repo, f.Repo) {
				return emit(out, map[string]any{"facts": f, "comparison": c})
			}
		}
		return emit(out, map[string]any{"facts": f})
	}
	var rep runReport
	if e := readState(o.vault, "report.json", &rep); e != nil {
		return e
	}
	return emit(out, rep)
}

// credentialsInSources lists each versioned credential once by repository, file and key.
func credentialsInSources(scans []*repoScan) []map[string]string {
	out, seen := []map[string]string{}, map[string]bool{}
	for _, s := range scans {
		for _, e := range s.entries {
			k := s.in.Name + "\x00" + e.File + "\x00" + e.KeyPath
			if e.Credential && !seen[k] {
				seen[k] = true
				out = append(out, map[string]string{"repo": s.in.Name, "file": e.File, "key": e.KeyPath})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["repo"]+out[i]["file"]+out[i]["key"] < out[j]["repo"]+out[j]["file"]+out[j]["key"]
	})
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}
