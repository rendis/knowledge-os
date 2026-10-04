package discover

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestObservationKeepsCaptureProvenance(t *testing.T) {
	v := t.TempDir()
	source := filepath.Join(t.TempDir(), "observation.json")
	const captured = "2000-01-01T03:00:00.123+03:00"
	b := `{"provider":"local","scope":"probe","service":"fixture","command":"printf fixture","captured_at":"` + captured + `","captured_by":"original-collector","resources":[{"name":"probe-resource"}]}`
	if e := os.WriteFile(source, []byte(b), 0o644); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := recordObservation(options{vault: v, record: source}, &out); e != nil {
		t.Fatal(e)
	}
	observations, e := loadObservations(v)
	if e != nil || len(observations) != 1 {
		t.Fatalf("recorded observations: %+v %v", observations, e)
	}
	if ob := observations[0]; ob.CapturedAt != captured || ob.CapturedBy != "original-collector" {
		t.Fatalf("import must preserve the capture time and collector: %+v", ob)
	}
	// Preserving a collector must keep the observation's existing credential guard.
	b = strings.Replace(b, "original-collector", "https://collector.example.test/run?sig=AbCdEfGhIjKlMnOp", 1)
	if e := os.WriteFile(source, []byte(b), 0o644); e != nil {
		t.Fatal(e)
	}
	if e := recordObservation(options{vault: v, record: source}, &out); e == nil || !strings.Contains(e.Error(), "credential") {
		t.Fatalf("a collector with credentials must be refused: %v", e)
	}
	observations, e = loadObservations(v)
	if e != nil || len(observations) != 1 || observations[0].CapturedBy != "original-collector" {
		t.Fatalf("a rejected import must keep the previous observation: %+v %v", observations, e)
	}
}

func TestObservationDefaultsAndInvalidCaptureTime(t *testing.T) {
	for _, captured := range []string{"", "not-a-date"} {
		t.Run(captured, func(t *testing.T) {
			v := t.TempDir()
			source := filepath.Join(t.TempDir(), "observation.json")
			b, _ := json.Marshal(observation{Provider: "local", Scope: "probe", Service: "fixture", Command: "printf fixture", CapturedAt: captured, Resources: []observedResource{{Name: "probe-resource"}}})
			if e := os.WriteFile(source, b, 0o644); e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			before := time.Now().UTC().Truncate(time.Second)
			e := recordObservation(options{vault: v, record: source}, &out)
			if captured != "" {
				if e == nil || !strings.Contains(e.Error(), "captured_at") {
					t.Fatalf("invalid capture time must be refused: %v", e)
				}
				if observations, _ := loadObservations(v); len(observations) != 0 {
					t.Fatal("an invalid observation was written")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			observations, e := loadObservations(v)
			if e != nil || len(observations) != 1 {
				t.Fatalf("recorded observations: %+v %v", observations, e)
			}
			ob := observations[0]
			at, e := time.Parse(time.RFC3339, ob.CapturedAt)
			if e != nil || at.Before(before) || at.After(time.Now().UTC()) || ob.CapturedBy != "agent" {
				t.Fatalf("legacy inputs still get capture defaults: %+v %v", ob, e)
			}
		})
	}
}

func TestObservationsExtendThePlatformBeyondTheProviders(t *testing.T) {
	vault := t.TempDir()
	obs := filepath.Join(t.TempDir(), "obs.json")
	write := func(content string) { _ = os.WriteFile(obs, []byte(content), 0o644) }
	var out bytes.Buffer
	write(`{"provider": "sftp", "scope": "files.partner.example", "service": "sftp", "command": "sftp -b - partner <<< 'ls /inbound'", "resources": [{"name": "/inbound/sales", "kind": "directory"}]}`)
	if e := recordObservation(options{vault: vault, record: obs}, &out); e != nil {
		t.Fatal(e)
	}
	write(`{"provider": "gcp", "scope": "acme-orders-prd", "service": "cloud-scheduler", "command": "gcloud scheduler jobs list --project acme-orders-prd --location us-east4 --format=json",
		"resources": [{"name": "projects/acme-orders-prd/locations/us-east4/jobs/nightly-close", "kind": "schedule", "links": [{"relation": "targets", "target": "https://orders.example.test/close"}]}]}`)
	if e := recordObservation(options{vault: vault, record: obs}, &out); e != nil {
		t.Fatal(e)
	}
	write(`{"provider": "gcp", "scope": "acme-orders-prd", "service": "logic", "command": "x", "resources": [{"name": "https://prod.example.test/invoke?sp=run&sig=AbCdEfGhIjKlMnOp"}]}`)
	if e := recordObservation(options{vault: vault, record: obs}, &out); e == nil || !strings.Contains(e.Error(), "credential") {
		t.Fatalf("a credential is never recorded: %v", e)
	}
	write(`{"provider": "gcp", "scope": "acme-orders-prd", "service": "x", "resources": [{"name": "a"}]}`)
	if e := recordObservation(options{vault: vault, record: obs}, &out); e == nil {
		t.Fatal("the command that produced the observation is required")
	}
	ix, e := loadPlatform(vault)
	if e != nil || len(ix.observed["nightly-close"]) != 1 || ix.observed["nightly-close"][0].scope != "gcp:acme-orders-prd" {
		t.Fatalf("observed names are indexed by short name: %v %+v", e, ix.observed)
	}

	dir := t.TempDir()
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod":         "module example.com/orders\n",
		"main.go":        "package main\nimport (\n\t\"github.com/pkg/sftp\"\n\t\"cloud.google.com/go/pubsub\"\n)\nvar _, _ = sftp.NewClient, pubsub.NewClient\n",
		"config/app.env": "SFTP_DIR=/inbound/sales\nNOTIFY=https://hooks.example.test/x?sig=AbCdEfGhIjKlMnOpQr\n",
	})
	st := &store{Dependencies: map[string]judgment{"go:github.com/pkg/sftp": {Choice: "file_transfer"}, "go:cloud.google.com/go/pubsub": {Choice: "messaging"}},
		ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	s := scan(t, "SVC-orders", svc)
	for _, e := range s.entries {
		st.ConfigKeys[keySignature(e)] = judgment{Choice: "storage_bucket", Confidence: 0.9}
		st.ConfigValues[entryID(e)] = judgment{Choice: "storage_bucket", Confidence: 0.9}
	}
	a := &assembly{scans: []*repoScan{s}, st: st, platform: ix, providers: []string{"gcp"}}
	f := a.facts()[0]
	observed := false
	for _, r := range f.Resources {
		for _, ev := range r.Evidence {
			observed = observed || ev.Kind == "platform-observed" && r.Name == "/inbound/sales"
		}
	}
	if !observed {
		t.Fatalf("a recorded observation is evidence for the resource: %+v", f.Resources)
	}
	unmanaged := []string{}
	for _, p := range f.Pending {
		if p.Kind == "platform-unmanaged" {
			unmanaged = append(unmanaged, p.Subject)
		}
	}
	if !slices.Equal(unmanaged, []string{"file_transfer"}) {
		t.Fatalf("services no configured provider reads are pending, the rest are not: %v", unmanaged)
	}
	creds := credentialsInSources([]*repoScan{s})
	if len(creds) != 1 || creds[0]["file"] != "config/app.env" || creds[0]["key"] != "NOTIFY" {
		t.Fatalf("a versioned credential is reported by file and key: %v", creds)
	}
	for _, e := range s.entries {
		if e.Key == "NOTIFY" && (e.Value != "<redacted>" || !e.Credential) {
			t.Fatalf("its value is never kept: %+v", e)
		}
	}
}
