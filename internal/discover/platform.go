package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Platform evidence: read-only listings of the messaging resources that exist and how they are wired
// (topics, and the subscriptions or queues that consume them). Each cloud is a provider with the same
// snapshot model; the core never names a cloud. A scope that cannot be read becomes a pending item with
// the exact command to confirm it later.

const platformRel = "90-Meta/discovery/platform"

// platformSubscription is a consumer of a topic: a Pub/Sub subscription, an SQS queue subscribed to an
// SNS topic, a Service Bus subscription. A standalone queue has no topic.
type platformSubscription struct {
	Name       string      `json:"name"`
	Topic      string      `json:"topic,omitempty"`
	Filter     string      `json:"filter,omitempty"`
	Attributes [][2]string `json:"attributes,omitempty"`
	Push       string      `json:"push_endpoint,omitempty"`
	Sink       string      `json:"sink,omitempty"`
	DeadLetter string      `json:"dead_letter,omitempty"`
	Delivery   string      `json:"delivery,omitempty"` // ack deadline, redelivery backoff, attempts, retention
}

// platformResource is a data service a provider lists: a database, collection, table, instance, bucket
// or dataset. Aliases are the other names configuration uses for it (short name, host, connection name).
type platformResource struct {
	Kind    string   `json:"kind"` // document_db | sql_db | object_storage | warehouse
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	State   string   `json:"state,omitempty"` // as the provider reports it (RUNNABLE, STOPPED, available…)
	Aliases []string `json:"aliases,omitempty"`
}

type platformSnapshot struct {
	Provider      string                 `json:"provider"`
	Scope         string                 `json:"scope"`
	CapturedAt    string                 `json:"captured_at"`
	Status        string                 `json:"status"` // ok | auth-required | denied | not-found | unavailable | error
	RefreshFailed map[string]string      `json:"refresh_failed,omitempty"`
	Detail        string                 `json:"detail,omitempty"`
	Confirm       string                 `json:"confirm_with"`
	Kinds         map[string]string      `json:"kinds,omitempty"` // status of each service read
	Topics        []string               `json:"topics"`
	Subscriptions []platformSubscription `json:"subscriptions"`
	Resources     []platformResource     `json:"resources,omitempty"`
}

// kindCapture reads one kind of service into the snapshot; on failure it returns the CLI's message.
type kindCapture func(*platformSnapshot) (string, error)

var notEnabled = []string{"has not been used", "is disabled", "service_disabled", "api not enabled", "not been enabled", "is not enabled"}

// captureKinds reads each kind independently: a service that is disabled or unreadable in the scope
// leaves the others captured. The scope fails only when no kind could be read.
func captureKinds(snap platformSnapshot, fns map[string]kindCapture, order []string) platformSnapshot {
	snap.Kinds = map[string]string{}
	first, listed := "", false
	size := func() int { return len(snap.Topics) + len(snap.Subscriptions) + len(snap.Resources) }
	for i, k := range order {
		before := size()
		topics, subscriptions, resources := len(snap.Topics), len(snap.Subscriptions), len(snap.Resources)
		msg, e := fns[k](&snap)
		if e == nil && (slices.ContainsFunc(snap.Topics[topics:], func(name string) bool { return strings.TrimSpace(name) == "" }) ||
			slices.ContainsFunc(snap.Subscriptions[subscriptions:], func(s platformSubscription) bool { return strings.TrimSpace(s.Name) == "" }) ||
			slices.ContainsFunc(snap.Resources[resources:], func(r platformResource) bool { return strings.TrimSpace(r.Name) == "" })) {
			msg = "unreadable " + k + " inventory: a resource has no name"
			e = errors.New(msg)
		}
		if e == nil {
			snap.Kinds[k] = "ok"
			listed = listed || size() > before
			continue
		}
		// A failed kind is incomplete evidence. Keep other completed kinds, but prevent
		// these rows from entering the confirmation index as a completed capture.
		snap.Topics, snap.Subscriptions, snap.Resources = snap.Topics[:topics], snap.Subscriptions[:subscriptions], snap.Resources[:resources]
		status := snap.failed(msg).Status
		low := strings.ToLower(msg)
		for _, m := range notEnabled {
			if strings.Contains(low, m) {
				status = "not-enabled"
			}
		}
		snap.Kinds[k] = status
		if first == "" && status != "not-enabled" {
			first = msg
		}
		if status == "auth-required" || status == "unavailable" {
			for _, rest := range order[i+1:] {
				snap.Kinds[rest] = status // the same login or tool fails every service
			}
			break
		}
	}
	// An empty listing proves access only when nothing failed: some CLIs answer an unreadable scope
	// with an empty success.
	if listed || first == "" && len(snap.Kinds) > 0 && !slices.ContainsFunc(order, func(k string) bool { return snap.Kinds[k] != "ok" && snap.Kinds[k] != "not-enabled" }) {
		return snap.sorted()
	}
	if first == "" {
		snap.Status, snap.Detail = "not-enabled", "none of the captured services is enabled in this scope"
		return snap
	}
	return snap.failed(first)
}

// key identifies a scope across providers: "<provider>:<scope>".
func (s platformSnapshot) key() string { return scopeKey(s.Provider, s.Scope) }

func scopeKey(provider, scope string) string { return provider + ":" + scope }

// platformProvider is one cloud's messaging platform.
type platformProvider interface {
	// parse splits a canonical resource identifier (path, ARN, resource id, URL) into its scope, kind
	// (topic or subscription) and short name.
	parse(id string) (scope, kind, short string, ok bool)
	// scopesIn lists the scopes a configuration value names through canonical identifiers.
	scopesIn(value string) []string
	// isScope reports whether a value judged a cloud project or region is one of this provider's scopes.
	isScope(value string) bool
	// validScope reports whether a scope given on the command line has this provider's format.
	validScope(scope string) bool
	capture(scope string) platformSnapshot
	// kinds lists the dependency categories whose platform service this provider reads.
	kinds() []string
	confirm(scope string) string
	commands() string
}

var providers = map[string]platformProvider{"gcp": gcpProvider{}, "aws": awsProvider{}, "azure": azureProvider{}}

// ProviderNames lists the platform providers this CLI can capture.
func ProviderNames() []string {
	out := []string{}
	for n := range providers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// runCLI runs a provider's own command-line tool, so the developer's existing login and permissions
// apply. Tests replace it.
var runCLI = func(name string, args ...string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return runCLIContext(ctx, name, args...)
}

func runCLIContext(ctx context.Context, name string, args ...string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if errors.Is(e, exec.ErrNotFound) {
		return nil, name + " is not installed or not on PATH", e
	}
	if ctx.Err() != nil {
		return nil, name + " read timed out or was cancelled", ctx.Err()
	}
	return b, strings.TrimSpace(stderr.String()), e
}

func newSnapshot(provider, scope string) platformSnapshot {
	return platformSnapshot{Provider: provider, Scope: scope, CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Confirm: providers[provider].confirm(scope), Topics: []string{}, Subscriptions: []platformSubscription{}}
}

// failed records why a scope could not be read, from the provider CLI's own message.
func (s platformSnapshot) failed(stderr string) platformSnapshot {
	low := strings.ToLower(stderr)
	switch {
	case strings.Contains(low, "not installed or not on path"):
		s.Status = "unavailable"
	case strings.Contains(low, "belong to account"):
		s.Status = "denied" // logged in, but to another account
	case strings.Contains(low, "auth login") || strings.Contains(low, "az login") || strings.Contains(low, "reauthentication") ||
		strings.Contains(low, "unable to locate credentials") || strings.Contains(low, "expiredtoken") || strings.Contains(low, "token has expired") ||
		strings.Contains(low, "sso session") || strings.Contains(low, "credentials"):
		s.Status = "auth-required"
	case strings.Contains(low, "permission") || strings.Contains(low, "denied") || strings.Contains(low, "authorizationfailed") ||
		strings.Contains(low, "not authorized") || strings.Contains(low, "forbidden") || strings.Contains(low, "does not have"):
		s.Status = "denied"
	case strings.Contains(low, "not found") || strings.Contains(low, "not exist") || strings.Contains(low, "notfound") || strings.Contains(low, "nonexistent"):
		s.Status = "not-found"
	default:
		s.Status = "error"
	}
	if len(stderr) > 300 {
		stderr = stderr[:300]
	}
	s.Detail = stderr
	return s
}

// jsonCLI requires observed JSON output. Empty stdout and null do not prove an empty listing.
func jsonCLI(v any, name string, args ...string) (string, error) {
	b, stderr, e := runCLI(name, args...)
	if e != nil {
		if stderr == "" {
			stderr = e.Error()
		}
		return stderr, e
	}
	if trimmed := bytes.TrimSpace(b); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "missing JSON output of " + name, errors.New("provider returned no observable JSON result")
	}
	if e := json.Unmarshal(b, v); e != nil {
		return "unreadable output of " + name + " " + strings.Join(args[:min(len(args), 3)], " "), e
	}
	return "", nil
}

func (s platformSnapshot) sorted() platformSnapshot {
	sort.Strings(s.Topics)
	sort.Slice(s.Resources, func(i, j int) bool { return s.Resources[i].Name < s.Resources[j].Name })
	sort.Slice(s.Subscriptions, func(i, j int) bool {
		a, b := s.Subscriptions[i], s.Subscriptions[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Topic < b.Topic
	})
	s.Status = "ok"
	return s
}

var unsafeFile = regexp.MustCompile(`[^a-z0-9._-]+`)

func snapshotPath(vault, provider, scope string) string {
	return filepath.Join(vault, platformRel, provider+"-"+strings.Trim(unsafeFile.ReplaceAllString(strings.ToLower(scope), "-"), "-")+".json")
}

func saveSnapshot(vault string, s platformSnapshot) error {
	p := snapshotPath(vault, s.Provider, s.Scope)
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
	matches, e := filepath.Glob(filepath.Join(vault, platformRel, "*.json"))
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
		if providers[s.Provider] == nil || s.Scope == "" {
			return nil, fmt.Errorf("%s: unknown platform provider %q or missing scope", m, s.Provider)
		}
		out = append(out, s)
	}
	return out, nil
}

// parseResource tries every provider's canonical format.
func parseResource(id string) (provider, scope, kind, short string, ok bool) {
	for _, n := range ProviderNames() {
		if s, k, sh, ok := providers[n].parse(id); ok {
			return n, s, k, sh, true
		}
	}
	return "", "", "", "", false
}

// scopeOf returns the scope key a canonical resource identifier belongs to.
func scopeOf(id string) string {
	if p, s, _, _, ok := parseResource(id); ok {
		return scopeKey(p, s)
	}
	return ""
}

// canonicalKind types a value written in a provider's canonical format as a messaging resource.
func canonicalKind(v string) string {
	if _, _, k, _, ok := parseResource(strings.TrimSpace(v)); ok {
		return "message_" + k
	}
	return ""
}

func normalizeResource(v string) string {
	v = strings.TrimSpace(v)
	if _, _, _, short, ok := parseResource(v); ok {
		return strings.ToLower(short)
	}
	return strings.ToLower(v)
}

func shortName(full string) string {
	if _, _, _, short, ok := parseResource(full); ok {
		return strings.ToLower(short)
	}
	if i := strings.LastIndexAny(full, "/:"); i >= 0 {
		return strings.ToLower(full[i+1:])
	}
	return strings.ToLower(full)
}

// platformIndex answers deterministic questions about platform resources by short name.
type platformIndex struct {
	topics map[string][]string               // short name -> full names
	subs   map[string][]platformSubscription // short name -> subscriptions
	scopes map[string]platformSnapshot       // scope key -> snapshot
	// objects holds data services by every name configuration may use (lower case).
	objects map[string][]objectRef
	// observed holds names the agent recorded outside the built-in providers (short name -> refs).
	observed map[string][]observedRef
}

type objectRef struct {
	scope string
	res   platformResource
}

// objectsNamed returns the data services a configuration value or code literal names.
func (ix platformIndex) objectsNamed(v string) []objectRef {
	seen, out := map[string]bool{}, []objectRef{}
	for _, k := range []string{strings.ToLower(strings.TrimSpace(v)), normalizeResource(v), shortName(v), hostOf(v)} {
		for _, o := range ix.objects[k] {
			if !seen[o.scope+o.res.Name] {
				seen[o.scope+o.res.Name] = true
				out = append(out, o)
			}
		}
	}
	return out
}

func buildPlatformIndex(snaps []platformSnapshot) platformIndex {
	ix := platformIndex{topics: map[string][]string{}, subs: map[string][]platformSubscription{}, scopes: map[string]platformSnapshot{}, objects: map[string][]objectRef{}}
	for _, s := range snaps {
		if len(s.RefreshFailed) != 0 {
			// Retained rows describe the last successful capture, not the failed current read.
			// Keep the historical snapshot on disk, but exclude its resource confirmations.
			s.Status = "refresh-failed"
			s.Detail = s.RefreshFailed["status"] + ": " + s.RefreshFailed["detail"]
			if confirm := s.RefreshFailed["confirm_with"]; confirm != "" {
				s.Confirm = confirm
			}
			ix.scopes[s.key()] = s
			continue
		}
		ix.scopes[s.key()] = s
		for _, r := range s.Resources {
			ref := objectRef{scope: s.key(), res: r}
			keys := map[string]bool{strings.ToLower(r.Name): true}
			for _, a := range r.Aliases {
				keys[strings.ToLower(a)] = true
			}
			for k := range keys {
				ix.objects[k] = append(ix.objects[k], ref)
			}
		}
		for _, t := range s.Topics {
			ix.topics[shortName(t)] = appendUnique(ix.topics[shortName(t)], t)
		}
		for _, sub := range s.Subscriptions {
			ix.subs[shortName(sub.Name)] = append(ix.subs[shortName(sub.Name)], sub)
			if sub.Topic != "" && scopeOf(sub.Topic) != "" {
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
	case s:
		return "message_subscription"
	case t:
		return "message_topic"
	}
	return ""
}

// filterAttributes extracts attribute equalities from a subscription filter written as
// attributes.key = "value" or hasPrefix(attributes.key, "value") (Pub/Sub) or key = 'value' (SQL rules).
var (
	filterAttr = regexp.MustCompile(`attributes\.([\w.\-]+)\s*=\s*"([^"]+)"|hasPrefix\(\s*attributes\.([\w.\-]+)\s*,\s*"([^"]+)"\s*\)`)
	sqlAttr    = regexp.MustCompile(`(?i)(?:user\.|sys\.)?([a-z_][\w.\-]*)\s*=\s*'([^']*)'`)
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

// policyAttributes flattens a JSON filter policy ({"eventType": ["a", {"prefix": "b"}]}) into pairs.
func policyAttributes(policy string) [][2]string {
	var m map[string]any
	if json.Unmarshal([]byte(policy), &m) != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := [][2]string{}
	for _, k := range keys {
		vals, ok := m[k].([]any)
		if !ok {
			continue // nested (message body) policies are kept only in the raw filter
		}
		for _, v := range vals {
			switch x := v.(type) {
			case string:
				out = append(out, [2]string{k, x})
			case map[string]any:
				if p, ok := x["prefix"].(string); ok {
					out = append(out, [2]string{k, p})
				}
			}
		}
	}
	return out
}
