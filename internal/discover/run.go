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
  platform   [--project ID ...] [--referenced] [--dry-run]
             Capture read-only Pub/Sub listings with gcloud (requires authorization).
             --referenced uses the projects named by configuration in the last run.
  report     [--repo NAME]   Last run summary, or one repository's facts.
  check      --note PATH [--note PATH ...] [--repo NAME] [--semantic]
             Gates for a repository note: G1 source anchors resolve at their commit and
             the identifiers they name are in the cited lines; G2 every connector and
             configured resource is evidenced or addressed; freshness of cited files;
             --semantic asks Jev whether each cited sentence is supported (review aid).
             A candidate outside the vault is matched to its repository by its aliases.
Facts and questions are local (.agents/state/discovery). Judgments and platform snapshots
are versioned under 90-Meta/discovery/. All output is JSON.`

const stateRel = ".agents/state/discovery"

type options struct {
	vault, at, classify, kind, file string
	repos, projects, notes          []string
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
		case "--project":
			v, e = val()
			o.projects = append(o.projects, v)
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
		return capturePlatform(o, out)
	case "report":
		return showReport(o, out)
	case "check":
		return runCheck(o, out, o.semantic)
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
	PlatformProjects []string          `json:"platform_projects_referenced"`
	PlatformCaptured map[string]string `json:"platform_captured"`
	Pending          map[string]int    `json:"pending_items"`
	Comparison       map[string]int    `json:"comparison"`
	Duration         string            `json:"duration"`
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
		in.Ref = defaultRef(in.Path)
		if o.at == "note" {
			if c := noteCommit(o.vault, in.Note); c != "" {
				in.Ref = c
			}
		}
		s, e := scanRepository(in)
		if e != nil {
			failed = append(failed, in.Name+": "+e.Error())
			continue
		}
		s.in.Commit, _ = resolveCommit(in.Path, in.Ref)
		scans = append(scans, s)
	}
	resolveLibraries(scans)
	st, e := loadStore(o.vault)
	if e != nil {
		return e
	}
	snaps, e := loadSnapshots(o.vault)
	if e != nil {
		return e
	}
	a := &assembly{scans: scans, st: st, platform: buildPlatformIndex(snaps)}
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
		PendingQuestions: map[string]int{}, Acceleration: accelerationStatus(), PlatformProjects: a.platformProjects(),
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
	for p, s := range a.platform.projects {
		rep.PlatformCaptured[p] = s.Status
	}
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

func capturePlatform(o options, out io.Writer) error {
	projects := o.projects
	if o.referenced {
		var rep runReport
		if e := readState(o.vault, "report.json", &rep); e != nil {
			return e
		}
		projects = append(projects, rep.PlatformProjects...)
	}
	sort.Strings(projects)
	uniq := []string{}
	for i, p := range projects {
		if (i == 0 || p != projects[i-1]) && projectID.MatchString(p) {
			uniq = append(uniq, p)
		}
	}
	if len(uniq) == 0 {
		return errors.New("no project selected; pass --project or --referenced")
	}
	if o.dryRun {
		return emit(out, map[string]any{"would_capture": uniq, "commands": "gcloud pubsub topics list / subscriptions list --format=json (read-only)"})
	}
	result := map[string]string{}
	for _, p := range uniq {
		s := captureGCP(p)
		if e := saveSnapshot(o.vault, s); e != nil {
			return e
		}
		result[p] = s.Status
		if s.Status != "ok" {
			if b, e := os.ReadFile(snapshotPath(o.vault, p)); e == nil && strings.Contains(string(b), `"refresh_failed"`) {
				result[p] = s.Status + " (previous snapshot kept)"
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
