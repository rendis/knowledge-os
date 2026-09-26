package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Wiring is what the discovery store and the platform snapshots know about one messaging topic: the
// repositories whose configuration names it, the subscriptions each captured scope has on it and the
// repositories that name those subscriptions. It answers "who publishes, who consumes" from evidence
// instead of from the notes' relations alone.
type Wiring struct {
	Topic         string
	Repos         []WiringRepo
	Subscriptions []WiringSubscription
	Scopes        []WiringScope // the captured scopes the absence of a subscription is bounded to
	Confirm       string        // the provider command that lists the topic's subscriptions everywhere
	NoSubs        []string      // captured scopes that list no subscription at all
	Similar       []string      // other topics with the same last segment, easy to confuse with this one
}

// WiringScope is one captured platform scope: how many subscriptions it listed and when.
type WiringScope struct {
	Scope, Captured string
	Subscriptions   int
}

// WiringRepo is a repository whose configuration or code names the topic or one of its subscriptions.
type WiringRepo struct {
	Repo, Direction, Via, File, Key string
}

// WiringSubscription is a subscription a platform snapshot lists on the topic.
type WiringSubscription struct {
	Scope, Captured, Name, Topic, Filter, Push, DeadLetter string
	ConfiguredBy                                           []string
}

// Subscription finds a subscription by its short name in the captured platform scopes: the topic it
// reads, its filter, push endpoint and dead letter, per scope.
func Subscription(vault, name string) []WiringSubscription {
	short := func(full string) string { return full[strings.LastIndex(full, "/")+1:] }
	snaps, _ := loadSnapshots(vault)
	out := []WiringSubscription{}
	for _, s := range snaps {
		for _, sub := range s.Subscriptions {
			if short(sub.Name) == name {
				out = append(out, WiringSubscription{Scope: s.Provider + ":" + s.Scope, Captured: s.CapturedAt, Name: name, Topic: short(sub.Topic), Filter: sub.Filter, Push: sub.Push, DeadLetter: short(sub.DeadLetter)})
			}
		}
	}
	return out
}

// TopicWiring returns the wiring of a topic by its short name, or false when neither the facts nor
// the snapshots know it.
func TopicWiring(vault, name string) (Wiring, bool) {
	w := Wiring{Topic: name}
	short := func(full string) string { return full[strings.LastIndex(full, "/")+1:] }
	snaps, _ := loadSnapshots(vault)
	subNames := map[string][]int{} // subscription name → its indexes in w.Subscriptions
	similar := map[string]bool{}
	known := false
	for _, s := range snaps {
		if s.Status != "ok" {
			continue
		}
		scope := s.Provider + ":" + s.Scope
		if len(s.Subscriptions) == 0 {
			w.NoSubs = append(w.NoSubs, scope)
			continue
		}
		w.Scopes = append(w.Scopes, WiringScope{scope, s.CapturedAt, len(s.Subscriptions)})
		for _, t := range s.Topics {
			switch st := short(t); {
			case st == name:
				known = true
				if s.Provider == "gcp" && (w.Confirm == "" || strings.Contains(s.Scope, "prod") && !strings.Contains(w.Confirm, "prod")) {
					w.Confirm = "gcloud pubsub topics list-subscriptions " + t
				}
			case lastSegment(st) == lastSegment(name) && strings.Contains(name, ".") && !similar[st]:
				similar[st] = true
				w.Similar = append(w.Similar, st)
			}
		}
		for _, sub := range s.Subscriptions {
			if short(sub.Topic) != name {
				continue
			}
			ws := WiringSubscription{Scope: scope, Captured: s.CapturedAt, Name: short(sub.Name), Filter: sub.Filter, Push: sub.Push, DeadLetter: short(sub.DeadLetter)}
			subNames[ws.Name] = append(subNames[ws.Name], len(w.Subscriptions))
			w.Subscriptions = append(w.Subscriptions, ws)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(vault, stateRel, "facts", "*.json"))
	sort.Strings(paths)
	seen := map[string]bool{}
	configured := map[string][]string{}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var f repoFacts
		if json.Unmarshal(b, &f) != nil {
			continue
		}
		for _, r := range f.Resources {
			via := ""
			switch {
			case r.Name == name && strings.Contains(r.Type, "topic"):
			case strings.Contains(r.Type, "subscription") && (short(r.Topic) == name || subNames[r.Name] != nil):
				via = r.Name
				configured[r.Name] = append(configured[r.Name], f.Repo)
			default:
				continue
			}
			k := f.Repo + "\x00" + via
			if seen[k] {
				continue
			}
			seen[k] = true
			known = true
			wr := WiringRepo{Repo: f.Repo, Direction: r.Direction, Via: via}
			for k, ev := range r.Evidence {
				// Production configuration says where it runs; development may differ.
				if k == 0 || strings.Contains(ev.File, "prod") && !strings.Contains(wr.File, "prod") {
					wr.File, wr.Key = ev.File, ev.Key
				}
			}
			w.Repos = append(w.Repos, wr)
		}
	}
	for n, repos := range configured {
		for _, k := range subNames[n] {
			w.Subscriptions[k].ConfiguredBy = uniqueStrings(repos)
		}
	}
	sort.Strings(w.Similar)
	if w.Confirm == "" {
		w.Confirm = "the provider's command that lists the topic's subscriptions"
	}
	return w, known
}

func lastSegment(name string) string {
	return name[strings.LastIndex(name, ".")+1:]
}
