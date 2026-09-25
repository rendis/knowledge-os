package discover

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Platform evidence: read-only listings of what exists and how it is wired. A project that
// cannot be read becomes a pending item with the exact command to confirm it later.

const platformRel = "90-Meta/discovery/platform"

type pubsubSubscription struct {
	Name       string      `json:"name"`
	Topic      string      `json:"topic"`
	Filter     string      `json:"filter,omitempty"`
	Attributes [][2]string `json:"attributes,omitempty"`
	Push       string      `json:"push_endpoint,omitempty"`
	BigQuery   string      `json:"bigquery_table,omitempty"`
	DeadLetter string      `json:"dead_letter_topic,omitempty"`
}

type platformSnapshot struct {
	Provider      string               `json:"provider"`
	Project       string               `json:"project"`
	CapturedAt    string               `json:"captured_at"`
	Status        string               `json:"status"` // ok | auth-required | denied | not-found | error
	RefreshFailed map[string]string    `json:"refresh_failed,omitempty"`
	Detail        string               `json:"detail,omitempty"`
	Confirm       string               `json:"confirm_with"`
	Topics        []string             `json:"topics"`
	Subscriptions []pubsubSubscription `json:"subscriptions"`
}

var (
	projectID  = regexp.MustCompile(`^[a-z][a-z0-9\-]{4,28}[a-z0-9]$`)
	filterAttr = regexp.MustCompile(`attributes\.([\w.\-]+)\s*=\s*"([^"]+)"|hasPrefix\(\s*attributes\.([\w.\-]+)\s*,\s*"([^"]+)"\s*\)`)
)

func filterAttributes(f string) [][2]string {
	out := [][2]string{}
	for _, m := range filterAttr.FindAllStringSubmatch(f, -1) {
		if m[1] != "" {
			out = append(out, [2]string{m[1], m[2]})
		} else {
			out = append(out, [2]string{m[3], m[4]})
		}
	}
	return out
}

func gcloudJSON(args ...string) ([]byte, string, error) {
	cmd := exec.Command("gcloud", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	return b, strings.TrimSpace(stderr.String()), e
}

func captureGCP(project string) platformSnapshot {
	snap := platformSnapshot{Provider: "gcp-pubsub", Project: project, CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Confirm: fmt.Sprintf("gcloud pubsub topics list --project %s && gcloud pubsub subscriptions list --project %s", project, project),
		Topics:  []string{}, Subscriptions: []pubsubSubscription{}}
	fail := func(stderr string) platformSnapshot {
		low := strings.ToLower(stderr)
		switch {
		case strings.Contains(low, "auth login") || strings.Contains(low, "reauthentication") || strings.Contains(low, "credentials"):
			snap.Status = "auth-required"
			snap.Confirm = "gcloud auth login, then " + snap.Confirm
		case strings.Contains(low, "permission") || strings.Contains(low, "denied"):
			snap.Status = "denied"
		case strings.Contains(low, "not found") || strings.Contains(low, "not exist"):
			snap.Status = "not-found"
		default:
			snap.Status = "error"
		}
		if len(stderr) > 300 {
			stderr = stderr[:300]
		}
		snap.Detail = stderr
		return snap
	}
	b, stderr, e := gcloudJSON("pubsub", "topics", "list", "--project", project, "--format=json")
	if e != nil {
		return fail(stderr)
	}
	var topics []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(b, &topics) != nil {
		return fail("unreadable topics listing")
	}
	for _, t := range topics {
		snap.Topics = append(snap.Topics, t.Name)
	}
	b, stderr, e = gcloudJSON("pubsub", "subscriptions", "list", "--project", project, "--format=json")
	if e != nil {
		return fail(stderr)
	}
	var subs []struct {
		Name       string `json:"name"`
		Topic      string `json:"topic"`
		Filter     string `json:"filter"`
		PushConfig struct {
			PushEndpoint string `json:"pushEndpoint"`
		} `json:"pushConfig"`
		BigqueryConfig struct {
			Table string `json:"table"`
		} `json:"bigqueryConfig"`
		DeadLetterPolicy struct {
			DeadLetterTopic string `json:"deadLetterTopic"`
		} `json:"deadLetterPolicy"`
	}
	if json.Unmarshal(b, &subs) != nil {
		return fail("unreadable subscriptions listing")
	}
	for _, s := range subs {
		snap.Subscriptions = append(snap.Subscriptions, pubsubSubscription{Name: s.Name, Topic: s.Topic, Filter: s.Filter,
			Attributes: filterAttributes(s.Filter), Push: s.PushConfig.PushEndpoint, BigQuery: s.BigqueryConfig.Table, DeadLetter: s.DeadLetterPolicy.DeadLetterTopic})
	}
	sort.Strings(snap.Topics)
	sort.Slice(snap.Subscriptions, func(i, j int) bool { return snap.Subscriptions[i].Name < snap.Subscriptions[j].Name })
	snap.Status = "ok"
	return snap
}

func snapshotPath(vault, project string) string {
	return filepath.Join(vault, platformRel, "gcp-pubsub-"+project+".json")
}

func saveSnapshot(vault string, s platformSnapshot) error {
	p := snapshotPath(vault, s.Project)
	// A failed refresh never discards evidence captured before: keep it and record the attempt.
	if s.Status != "ok" {
		if b, e := os.ReadFile(p); e == nil {
			var prev platformSnapshot
			if json.Unmarshal(b, &prev) == nil && prev.Status == "ok" {
				prev.RefreshFailed = map[string]string{"at": s.CapturedAt, "status": s.Status, "detail": s.Detail, "confirm_with": s.Confirm}
				s = prev
			}
		}
	}
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", " ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

func loadSnapshots(vault string) ([]platformSnapshot, error) {
	matches, e := filepath.Glob(filepath.Join(vault, platformRel, "gcp-pubsub-*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(matches)
	out := []platformSnapshot{}
	for _, m := range matches {
		b, e := os.ReadFile(m)
		if e != nil {
			return nil, e
		}
		var s platformSnapshot
		if e := json.Unmarshal(b, &s); e != nil {
			return nil, fmt.Errorf("%s: %w", m, e)
		}
		out = append(out, s)
	}
	return out, nil
}

// platformIndex answers deterministic questions about platform resources by short name.
type platformIndex struct {
	topics   map[string][]string             // short name -> full names
	subs     map[string][]pubsubSubscription // short name -> subscriptions
	byTopic  map[string][]pubsubSubscription // full topic -> subscriptions
	projects map[string]platformSnapshot
}

func shortName(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return strings.ToLower(full[i+1:])
	}
	return strings.ToLower(full)
}

func buildPlatformIndex(snaps []platformSnapshot) platformIndex {
	ix := platformIndex{topics: map[string][]string{}, subs: map[string][]pubsubSubscription{}, byTopic: map[string][]pubsubSubscription{}, projects: map[string]platformSnapshot{}}
	for _, s := range snaps {
		ix.projects[s.Project] = s
		for _, t := range s.Topics {
			ix.topics[shortName(t)] = appendUnique(ix.topics[shortName(t)], t)
		}
		for _, sub := range s.Subscriptions {
			ix.subs[shortName(sub.Name)] = append(ix.subs[shortName(sub.Name)], sub)
			ix.byTopic[sub.Topic] = append(ix.byTopic[sub.Topic], sub)
			if strings.HasPrefix(sub.Topic, "projects/") {
				ix.topics[shortName(sub.Topic)] = appendUnique(ix.topics[shortName(sub.Topic)], sub.Topic)
			}
		}
	}
	return ix
}

// variants returns platform names that extend a base name with suffix tokens (at most 5).
func (ix platformIndex) variants(base string) []string {
	out := []string{}
	for short, full := range ix.topics {
		if strings.HasPrefix(short, base+"-") {
			out = append(out, full...)
		}
	}
	for short, subs := range ix.subs {
		if strings.HasPrefix(short, base+"-") {
			for _, s := range subs {
				out = append(out, s.Name)
			}
		}
	}
	sort.Strings(out)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func (ix platformIndex) typeOf(value string) string {
	n := normalizeResource(value)
	_, t := ix.topics[n]
	_, s := ix.subs[n]
	switch {
	case t && !s:
		return "pubsub_topic"
	case s && !t:
		return "pubsub_subscription"
	case s && t:
		return "pubsub_subscription"
	}
	return ""
}

var canonicalPubSub = regexp.MustCompile(`projects/([a-z][a-z0-9\-]+)/(topics|subscriptions)/([\w.\-{}$~%+]+)`)

func normalizeResource(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if m := canonicalPubSub.FindStringSubmatch(v); m != nil {
		return m[3]
	}
	return v
}
