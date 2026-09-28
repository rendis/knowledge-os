package discover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Judgments that code cannot make are asked once, answered by the agent, stored in the versioned
// cell vault and reused. A stored judgment is data: code decides what to do with it.

const storeRel = "90-Meta/discovery/classifications.json"

type judgment struct {
	Choice     string  `json:"choice"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
	At         string  `json:"at"`
}

type store struct {
	Schema       int                 `json:"schema"`
	Dependencies map[string]judgment `json:"dependencies"`
	ConfigKeys   map[string]judgment `json:"config_keys"`
	ConfigValues map[string]judgment `json:"config_entries"`
	dropped      int
}

func loadStore(vault string) (*store, error) {
	s := &store{Schema: 1, Dependencies: map[string]judgment{}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	b, e := os.ReadFile(filepath.Join(vault, storeRel))
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return nil, e
	}
	if e := json.Unmarshal(b, s); e != nil {
		return nil, fmt.Errorf("%s: %w", storeRel, e)
	}
	for _, m := range []*map[string]judgment{&s.Dependencies, &s.ConfigKeys, &s.ConfigValues} {
		if *m == nil {
			*m = map[string]judgment{}
		}
	}
	s.dropCredentials()
	return s, nil
}

// dropCredentials removes judgments recorded for entries whose value is a credential, so a store written
// before a credential was recognized loses it on the next save.
func (s *store) dropCredentials() {
	for id := range s.ConfigValues {
		parts := strings.SplitN(id, "|", 5) // key|parent|context||value
		if len(parts) == 5 && credentialEntry(parts[0], parts[4]) {
			delete(s.ConfigValues, id)
			s.dropped++
		}
	}
}

func (s *store) save(vault string) error {
	p := filepath.Join(vault, storeRel)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", " ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e := os.WriteFile(tmp, append(b, '\n'), 0o644); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}

func (s *store) table(kind string) map[string]judgment {
	switch kind {
	case "dependency":
		return s.Dependencies
	case "config_key":
		return s.ConfigKeys
	default:
		return s.ConfigValues
	}
}

// Question kinds and their answer sets.

var dependencyOptions = map[string]string{
	"messaging":                 "Message broker or pub/sub client: publishes or consumes events.",
	"document_db":               "Document or key-value database client (Firestore, MongoDB, Redis...).",
	"sql_db":                    "Relational database driver, ORM or SQL access layer.",
	"warehouse":                 "Analytics warehouse client such as BigQuery.",
	"object_storage":            "Cloud object or blob storage client.",
	"file_transfer":             "SFTP, FTP or SSH file transfer.",
	"http_client":               "Makes outbound HTTP calls: HTTP client, fetch, REST client. Standard HTTP packages that can call count here.",
	"http_server":               "Web framework or HTTP server that receives requests.",
	"rpc":                       "gRPC or another RPC framework.",
	"secrets_or_identity":       "Secret manager, authentication or identity provider client.",
	"scheduler_or_queue_runner": "Cron or scheduling library, or task-queue runner.",
	"cloud_sdk_other":           "Other cloud SDK or auth helper.",
	"internal_no_io":            "No external communication: language helpers, logging, telemetry, validation, config, utilities, UI, build or test tooling.",
}

var resourceOptions = map[string]string{
	"message_topic":           "Holds the name, path, ARN or id of a topic or exchange messages are published to (any broker or cloud).",
	"message_subscription":    "Holds the name, path, ARN, URL or id of a subscription or queue a consumer reads from (any broker or cloud).",
	"database_object":         "Holds a database, schema, table, collection or dataset name, or a database host/connection.",
	"storage_bucket":          "Holds a cloud storage bucket or a remote file location (for example an SFTP path or host).",
	"http_endpoint":           "Holds a URL, host, base path or route of an HTTP or gRPC API of ANOTHER service that is called.",
	"service_identity":        "Holds the name of this deployable service itself: deployment, container, cloud function, microservice or application name.",
	"cloud_project_or_region": "Holds a cloud project, account or subscription id, region, cluster, namespace or environment name.",
	"secret_reference":        "Holds a reference to a secret, credential, API key or token.",
	"other":                   "Anything else: settings, ports, resource limits, labels, images, versions, logging, flags, build or package metadata.",
}

var entryOptions = func() map[string]string {
	m := map[string]string{}
	for k, v := range resourceOptions {
		m[k] = v
	}
	m["not_a_resource_name"] = "The value is not itself a resource name, path or URL (a country code, environment word, team name, template path, expression or description)."
	return m
}()

var questionText = map[string]string{
	"dependency":   "Classify what external system, if any, this dependency lets the code communicate with. If an imported subpackage performs communication, classify by that communication.",
	"config_key":   "These key/value entries come from configuration, deployment or infrastructure-as-code files of microservice repositories and share the same key. What does this key hold? Judge from the key path, its context and the example values.",
	"config_entry": "This entry comes from a configuration, deployment or infrastructure-as-code file. Using the key path, the block context and the file, decide what the `value` of this entry is.",
}

func optionsFor(kind string) map[string]string {
	switch kind {
	case "dependency":
		return dependencyOptions
	case "config_key":
		return resourceOptions
	default:
		return entryOptions
	}
}

type question struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	State        map[string]any    `json:"state"`
	Instructions string            `json:"instructions"`
	Options      map[string]string `json:"options"`
}

func newQuestion(kind, id string, state map[string]any) question {
	return question{ID: id, Kind: kind, State: state, Instructions: questionText[kind], Options: optionsFor(kind)}
}

func round3(f float64) float64 { return float64(int(f*1000+0.5)) / 1000 }

func today() string { return time.Now().UTC().Format("2006-01-02") }

// recordAnswers stores agent answers after validating them against the question set.
func recordAnswers(s *store, pending map[string]question, answers []map[string]any, source string) (int, error) {
	n := 0
	for _, a := range answers {
		id, _ := a["id"].(string)
		choice, _ := a["choice"].(string)
		q, ok := pending[id]
		if !ok { // a recorded judgment can be corrected, e.g. after a review finding
			for _, kind := range []string{"dependency", "config_key", "config_entry"} {
				if _, done := s.table(kind)[id]; done {
					q, ok = question{ID: id, Kind: kind, Options: optionsFor(kind)}, true
					break
				}
			}
		}
		if !ok {
			return n, fmt.Errorf("answer %q matches neither a pending question nor a recorded judgment", id)
		}
		if q.Options[choice] == "" {
			return n, fmt.Errorf("answer %q: choice %q is not one of the options", id, choice)
		}
		conf, _ := a["confidence"].(float64)
		if conf <= 0 || conf > 1 {
			conf = 0.8
		}
		s.table(q.Kind)[id] = judgment{Choice: choice, Confidence: round3(conf), Source: source, At: today()}
		n++
	}
	return n, nil
}

// ValidateState checks the versioned discovery state (classifications and platform snapshots):
// it parses, every judgment picks one of its kind's options with a confidence in [0,1] and a
// source. A branch that changes only this state publishes without a knowledge review.
func ValidateState(vault string) error {
	s, e := loadStore(vault)
	if e != nil {
		return e
	}
	for kind, t := range map[string]map[string]judgment{"dependency": s.Dependencies, "config_key": s.ConfigKeys, "config_entry": s.ConfigValues} {
		opts := optionsFor(kind)
		for id, j := range t {
			if opts[j.Choice] == "" || j.Confidence < 0 || j.Confidence > 1 || j.Source == "" {
				return fmt.Errorf("%s: %s %q has an invalid judgment", storeRel, kind, id)
			}
		}
	}
	_, e = loadSnapshots(vault)
	return e
}
