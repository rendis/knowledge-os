package discover

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Observations are platform evidence the agent captured outside the built-in providers: any cloud
// service, cluster, host or SaaS it could read. The built-in providers are the floor, not the ceiling;
// an observation is recorded with the command that produced it, so every reader reuses it and the same
// checks (facts, claims) know its names.

const observedRel = platformRel + "/observed"

type observedLink struct {
	Relation string `json:"relation"`
	Target   string `json:"target"`
}

type observedResource struct {
	Name  string         `json:"name"`
	Kind  string         `json:"kind,omitempty"`
	Links []observedLink `json:"links,omitempty"`
}

type observation struct {
	Provider   string             `json:"provider"`
	Scope      string             `json:"scope"`
	Service    string             `json:"service"`
	Command    string             `json:"command"`
	CapturedAt string             `json:"captured_at"`
	CapturedBy string             `json:"captured_by"`
	Resources  []observedResource `json:"resources"`
}

// observedRef is one observed name in the platform index.
type observedRef struct{ scope, service, name string }

var obsSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9._\-]*$`)

const observationFormat = `{"provider": "gcp", "scope": "<project, account/region, cluster, host>", "service": "cloud-scheduler",
 "command": "<the read-only command that produced it>",
 "captured_at": "<optional original RFC3339 capture time>", "captured_by": "<optional original collector>",
 "resources": [{"name": "<full or short name>", "kind": "<schedule, function, table...>", "links": [{"relation": "targets", "target": "<name>"}]}]}`

func recordObservation(o options, out io.Writer) error {
	b, e := os.ReadFile(o.record)
	if e != nil {
		return e
	}
	var ob observation
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&ob); e != nil {
		return fmt.Errorf("%s: %v; expected %s", o.record, e, observationFormat)
	}
	ob.Provider, ob.Service, ob.Scope = strings.ToLower(strings.TrimSpace(ob.Provider)), strings.ToLower(strings.TrimSpace(ob.Service)), strings.TrimSpace(ob.Scope)
	switch {
	case !obsSlug.MatchString(ob.Provider) || !obsSlug.MatchString(ob.Service):
		return errors.New("provider and service are lowercase names (gcp, aws, kubernetes, sftp…; cloud-scheduler, lambda…)")
	case ob.Scope == "" || strings.ContainsAny(ob.Scope, " \t\n"):
		return errors.New("scope names what was read: a project, account/region, subscription, cluster or host")
	case strings.TrimSpace(ob.Command) == "":
		return errors.New("command is required: the read-only command or query that produced the observation")
	case len(ob.Resources) == 0:
		return errors.New("resources lists at least one observed name")
	}
	if ob.CapturedAt != "" {
		if _, e := time.Parse(time.RFC3339, ob.CapturedAt); e != nil {
			return errors.New("captured_at must be an RFC3339 timestamp")
		}
	}
	values := []string{ob.Command, ob.Scope, ob.CapturedBy}
	for _, r := range ob.Resources {
		if strings.TrimSpace(r.Name) == "" {
			return errors.New("every resource has a name")
		}
		values = append(values, r.Name)
		for _, l := range r.Links {
			values = append(values, l.Target)
		}
	}
	for _, v := range values {
		if credentialEntry("", v) {
			return errors.New("the observation contains a credential: record names and relations only")
		}
	}
	sort.Slice(ob.Resources, func(i, j int) bool { return ob.Resources[i].Name < ob.Resources[j].Name })
	// Imported evidence keeps its provenance; omitted fields retain the live-capture defaults.
	if ob.CapturedAt == "" {
		ob.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if ob.CapturedBy == "" {
		ob.CapturedBy = "agent"
	}
	rel := filepath.Join(observedRel, ob.Provider+"-"+strings.Trim(unsafeFile.ReplaceAllString(strings.ToLower(ob.Scope), "-"), "-")+"-"+ob.Service+".json")
	p := filepath.Join(o.vault, rel)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	out2, _ := json.MarshalIndent(ob, "", " ")
	if e := os.WriteFile(p, append(out2, '\n'), 0o644); e != nil {
		return e
	}
	return emit(out, map[string]any{"recorded": filepath.ToSlash(rel), "resources": len(ob.Resources), "next": "kos discover run --vault <VAULT>; publish the file on a sync branch"})
}

func loadObservations(vault string) ([]observation, error) {
	matches, e := filepath.Glob(filepath.Join(vault, observedRel, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(matches)
	out := []observation{}
	for _, m := range matches {
		b, e := os.ReadFile(m)
		if e != nil {
			return nil, e
		}
		var ob observation
		if e := json.Unmarshal(b, &ob); e != nil {
			return nil, fmt.Errorf("%s: %w", m, e)
		}
		out = append(out, ob)
	}
	return out, nil
}

// loadPlatform builds the index from the providers' snapshots and the agent's observations.
func loadPlatform(vault string) (platformIndex, error) {
	snaps, e := loadSnapshots(vault)
	if e != nil {
		return platformIndex{}, e
	}
	obs, e := loadObservations(vault)
	if e != nil {
		return platformIndex{}, e
	}
	ix := buildPlatformIndex(snaps)
	ix.addObservations(obs)
	return ix, nil
}

func (ix *platformIndex) addObservations(obs []observation) {
	if ix.observed == nil {
		ix.observed = map[string][]observedRef{}
	}
	for _, ob := range obs {
		for _, r := range ob.Resources {
			ref := observedRef{scope: scopeKey(ob.Provider, ob.Scope), service: ob.Service, name: r.Name}
			full, short := normalizeResource(r.Name), shortName(r.Name)
			ix.observed[full] = append(ix.observed[full], ref)
			if short != full && short != "" {
				ix.observed[short] = append(ix.observed[short], ref) // configuration often names the short form
			}
		}
	}
}
