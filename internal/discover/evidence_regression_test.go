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
	"strconv"
	"strings"
	"testing"

	"knowledge-os/internal/config"
)

func TestPlatformCaptureRejectsUnnamedRows(t *testing.T) {
	for _, row := range []string{"[null]", "[{}]", `[{"name":" "}]`} {
		t.Run(row, func(t *testing.T) {
			fakeCLI(t, map[string]string{
				"gcloud pubsub topics list":        row,
				"gcloud pubsub subscriptions list": "[]",
				"gcloud firestore databases list":  "[]",
				"gcloud sql instances list":        "[]",
				"gcloud storage buckets list":      "[]",
				"bq ls":                            "[]",
			}, nil)
			s := (gcpProvider{}).capture("acme-orders-prd")
			if s.Status == "ok" || s.Kinds["messaging"] == "ok" || len(s.Topics) != 0 {
				t.Fatalf("unnamed rows are unreadable evidence, not a successful inventory: %+v", s)
			}
		})
	}
}

func TestFailedPlatformKindCannotConfirmPartialRows(t *testing.T) {
	s := captureKinds(newSnapshot("gcp", "acme-orders-prd"), map[string]kindCapture{
		"messaging": func(s *platformSnapshot) (string, error) {
			s.Topics = append(s.Topics, "projects/acme-orders-prd/topics/orders-in")
			s.Subscriptions = append(s.Subscriptions, platformSubscription{Name: "orders-sub"})
			s.add("messaging", "topic", "orders-in")
			return "incomplete messaging read", errors.New("incomplete")
		},
		"object_storage": func(s *platformSnapshot) (string, error) {
			s.add("object_storage", "bucket", "gs://orders-exports")
			return "", nil
		},
	}, []string{"messaging", "object_storage"})
	if s.Status != "ok" || s.Kinds["messaging"] == "ok" || len(s.Topics) != 0 || len(s.Subscriptions) != 0 || len(s.Resources) != 1 {
		t.Fatalf("a failed kind must not contribute confirmations; successful kinds remain available: %+v", s)
	}
	ix := buildPlatformIndex([]platformSnapshot{s})
	if len(ix.topics["orders-in"]) != 0 || len(ix.objectsNamed("orders-in")) != 0 || len(ix.objectsNamed("gs://orders-exports")) != 1 {
		t.Fatalf("only the completed capture may confirm a resource: %+v", ix)
	}
}

func TestPlatformCaptureCannotSynthesizeNamesFromMissingIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name, provider, scope, kind string
		outputs                     map[string]string
	}{
		{"gcp bucket", "gcp", "acme-orders-prd", "object_storage", map[string]string{"gcloud storage buckets list": "[{}]"}},
		{"gcp dataset", "gcp", "acme-orders-prd", "warehouse", map[string]string{"bq ls": "[{}]"}},
		{"gcp table", "gcp", "acme-orders-prd", "warehouse", map[string]string{
			"bq ls": `[{"datasetReference":{"datasetId":"orders"}}]`,
			"bq ls --project_id=acme-orders-prd --format=json --max_results=1000 acme-orders-prd:orders": "[{}]",
		}},
		{"aws table", "aws", "123456789012/us-east-1", "document_db", map[string]string{"aws dynamodb list-tables": `{"TableNames":[""]}`}},
		{"aws bucket", "aws", "123456789012/us-east-1", "object_storage", map[string]string{"aws s3api list-buckets": `{"Buckets":[{}]}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputs := map[string]string{
				"gcloud pubsub topics list": "[]", "gcloud pubsub subscriptions list": "[]",
				"gcloud firestore databases list": "[]", "gcloud sql instances list": "[]",
				"gcloud storage buckets list": "[]", "bq ls": "[]",
				"aws sts get-caller-identity": `{"Account":"123456789012"}`,
				"aws sns list-topics":         "{}", "aws sns list-subscriptions": "{}", "aws sqs list-queues": "{}",
				"aws dynamodb list-tables": "{}", "aws rds describe-db-instances": "{}", "aws rds describe-db-clusters": "{}", "aws s3api list-buckets": "{}",
			}
			for cmd, output := range tc.outputs {
				outputs[cmd] = output
			}
			fakeCLI(t, outputs, nil)
			s := providers[tc.provider].capture(tc.scope)
			if s.Status == "ok" || s.Kinds[tc.kind] == "ok" || len(s.Resources) != 0 {
				t.Fatalf("a namespace prefix cannot replace an unobserved identifier: %+v", s)
			}
		})
	}
}

func evidenceFixture(t *testing.T, sourceRoot string) (string, string) {
	t.Helper()
	repo := gitRepo(t, filepath.Join(sourceRoot, "SVC-probe"), map[string]string{
		"go.mod":   "module example.com/probe\n",
		"probe.go": "package probe\nconst Value = 1\n",
	})
	if b, e := exec.Command("git", "-C", repo, "remote", "add", "origin", "https://github.com/acme/SVC-probe.git").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\nsources:\n  repo_prefixes: [SVC]\n")
	write(t, vault, ".knowledge-os-config.yaml", fmt.Sprintf("version: 1\nworkspace:\n  repository_roots:\n    - %q\n", sourceRoot))
	sha, _ := resolveCommit(repo, "HEAD")
	write(t, vault, "20-Repos/probe.md", "---\naliases: [SVC-probe]\ncommit-analizado: "+sha+"\n---\n# Probe\n\nThe implementation defines `Value`. [^e1]\n\n[^e1]: probe.go#L1-L2 — `Value`\n")
	return vault, repo
}

func TestDiscoveryIncludesConfiguredCheckoutRoot(t *testing.T) {
	vault, repo := evidenceFixture(t, t.TempDir())
	write(t, vault, ".knowledge-os-config.yaml", fmt.Sprintf("version: 1\nworkspace:\n  repository_roots:\n    - %q\n", repo))
	inputs, e := discoverRepositories(vault, map[string]bool{"SVC-probe": true})
	canonical, _ := filepath.EvalSymlinks(repo)
	if e != nil || len(inputs) != 1 || inputs[0].Path != canonical {
		t.Fatalf("configured checkout must be discoverable: %v %+v", e, inputs)
	}
	r, e := checkNote(vault, "20-Repos/probe.md", "")
	if e != nil || r.Anchors["verified"] != 1 {
		t.Fatalf("the same checkout must verify short citations: %v %+v", e, r)
	}
}

func TestDiscoveryRejectsAmbiguousTrackedRepositories(t *testing.T) {
	root := t.TempDir()
	vault, repo := evidenceFixture(t, root)
	copy := filepath.Join(root, "second-checkout")
	if b, e := exec.Command("git", "clone", "-q", "--shared", repo, copy).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	if _, e := gitOutput(copy, "remote", "set-url", "origin", "https://github.com/ACME/svc-PROBE.git"); e != nil {
		t.Fatal(e)
	}
	if inputs, e := discoverRepositories(vault, map[string]bool{"SVC-probe": true}); e == nil || !strings.Contains(e.Error(), "ambiguous checkout") {
		t.Fatalf("duplicate source identities must not overwrite the same facts: %v %+v", e, inputs)
	}
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, at: "head"}, &out); e == nil {
		t.Fatal("an ambiguous discovery must not persist arbitrary evidence")
	}
	if _, e := os.Stat(filepath.Join(vault, stateRel)); !os.IsNotExist(e) {
		t.Fatalf("ambiguous source evidence was persisted: %v", e)
	}
	r, e := checkNote(vault, "20-Repos/probe.md", "SVC-probe")
	if e != nil || r.Verified {
		t.Fatalf("an explicit repo must remain unverified when identity is ambiguous: %v %+v", e, r)
	}
}

func TestDiscoveryBindsCaseVariantToNoteAndReferenceBranch(t *testing.T) {
	vault, repo := evidenceFixture(t, t.TempDir())
	if _, e := gitOutput(repo, "remote", "set-url", "origin", "https://github.com/ACME/svc-PROBE.git"); e != nil {
		t.Fatal(e)
	}
	if _, e := gitOutput(repo, "branch", "reviewed"); e != nil {
		t.Fatal(e)
	}
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\nsources:\n  repo_prefixes: [SVC]\n  reference_branches:\n    SVC-probe: reviewed\n")
	inputs, e := discoverRepositories(vault, map[string]bool{"SVC-probe": true})
	if e != nil || len(inputs) != 1 || inputs[0].Note != "20-Repos/probe.md" || !strings.HasSuffix(inputs[0].Ref, "/reviewed") {
		t.Fatalf("repository spelling must preserve note and configured branch: %v %+v", e, inputs)
	}
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, repos: []string{"SVC-probe"}, at: "note", readOnly: true}, &out); e != nil {
		t.Fatal(e)
	}
	var got struct {
		Report runReport   `json:"report"`
		Facts  []repoFacts `json:"facts"`
	}
	if e := json.Unmarshal(out.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if len(got.Report.Failed) != 0 || got.Report.Comparison["notes_compared"] != 1 || len(got.Facts) != 1 || got.Facts[0].Note != "20-Repos/probe.md" || got.Facts[0].Ref != noteCommit(vault, "20-Repos/probe.md") {
		t.Fatalf("a bound case variant must compare its note at the recorded commit: %+v", got)
	}
	located, e := config.LocateRepositoryByName(vault, "SVC-probe")
	if e != nil || located["reference_branch"] != "reviewed" || located["note"] != inputs[0].Note {
		t.Fatalf("the reader and discovery must use the same branch and note: %v %+v", e, located)
	}
}

func TestUnboundRepositoryNoteHasExplicitPendingEvidence(t *testing.T) {
	vault, _ := evidenceFixture(t, t.TempDir())
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots: []\n")
	r, e := checkNote(vault, "20-Repos/probe.md", "SVC-probe")
	if e != nil {
		t.Fatal(e)
	}
	if r.OK || r.Verified {
		t.Fatalf("an explicit repo cannot bypass missing identity: %+v", r)
	}
	var out bytes.Buffer
	if e := runCheck(options{vault: vault, notes: []string{"20-Repos/probe.md"}, readOnly: true}, &out); e == nil {
		t.Fatal("an unbound source check cannot exit successfully")
	}
	for _, issue := range r.Issues {
		if issue.Severity == "pending" && strings.Contains(issue.Detail, "checkout") {
			return
		}
	}
	t.Fatalf("an unbound short citation must not silently pass: %+v", r)
}

func TestPublicationRejectsUnresolvedSourceEvidence(t *testing.T) {
	vault, _ := evidenceFixture(t, t.TempDir())
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots: []\n")
	base, e := os.ReadFile(filepath.Join(vault, "20-Repos/probe.md"))
	if e != nil {
		t.Fatal(e)
	}
	for _, content := range [][]byte{nil, base} {
		ok, issues, _, e := CheckNoteIntroduced(vault, "20-Repos/probe.md", content)
		if e != nil || ok || len(issues) == 0 {
			t.Fatalf("unavailable source evidence cannot pass publication, including pre-existing unknowns: %v %v %v", ok, issues, e)
		}
	}
}

func TestWorktreeCommonRefsInvalidateNoteCache(t *testing.T) {
	t.Setenv("KOS_NO_CACHE", "")
	t.Setenv("KOS_CACHE_DIR", t.TempDir())
	vault, repo := evidenceFixture(t, t.TempDir())
	root := t.TempDir()
	worktree := filepath.Join(root, "SVC-probe")
	if b, e := exec.Command("git", "-C", repo, "worktree", "add", "--detach", worktree, "main").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	write(t, vault, ".knowledge-os-config.yaml", fmt.Sprintf("version: 1\nworkspace:\n  repository_roots:\n    - %q\n", root))
	first, e := checkNote(vault, "20-Repos/probe.md", "")
	if e != nil || first.Freshness["changed_files"] != 0 {
		t.Fatalf("initial freshness: %v %+v", e, first)
	}
	write(t, repo, "probe.go", "package probe\nconst Value = 2\n")
	for _, args := range [][]string{{"add", "probe.go"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "change"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
	}
	// Simulate the next CLI process, whose fingerprint memo starts empty.
	memoMu.Lock()
	delete(fingerprintMemo, vault)
	memoMu.Unlock()
	again, e := checkNote(vault, "20-Repos/probe.md", "")
	if e != nil || again.Freshness["changed_files"] != 1 || again.Freshness["head"] == first.Freshness["head"] {
		t.Fatalf("a common reference change must invalidate the cache: %v %+v", e, again)
	}
}

func TestPlatformJSONRequiresObservedOutput(t *testing.T) {
	for _, output := range []string{"", " \n", "null", "{}", "invalid"} {
		t.Run(fmt.Sprintf("output_%q", output), func(t *testing.T) {
			fakeCLI(t, map[string]string{"probe list": output}, nil)
			var rows []map[string]any
			if _, e := jsonCLI(&rows, "probe", "list"); e == nil {
				t.Fatalf("%q must not establish a successful empty listing", output)
			}
		})
	}
	fakeCLI(t, map[string]string{"probe list": "[]"}, nil)
	var rows []map[string]any
	if _, e := jsonCLI(&rows, "probe", "list"); e != nil || rows == nil {
		t.Fatalf("an observed empty JSON list is valid: %v %+v", e, rows)
	}
}

func TestReadOnlyDiscoveryReturnsEvidenceWithoutWrites(t *testing.T) {
	vault, _ := evidenceFixture(t, t.TempDir())
	// Loading old classifications may drop credentials in memory, but a question
	// does not authorize rewriting the store, even to prune an old entry.
	store := `{"schema":1,"config_entries":{"API_KEY|data|ConfigMap||obsolete-secret":{"choice":"other"}}}`
	write(t, vault, storeRel, store)
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, at: "head", readOnly: true}, &out); e != nil {
		t.Fatal(e)
	}
	var result struct {
		Facts  []repoFacts `json:"facts"`
		Report runReport   `json:"report"`
	}
	if e := json.Unmarshal(out.Bytes(), &result); e != nil || len(result.Facts) != 1 || len(result.Report.Scanned) != 1 {
		t.Fatalf("fresh evidence must be returned: %v %s", e, out.String())
	}
	if _, e := os.Stat(filepath.Join(vault, stateRel)); !os.IsNotExist(e) {
		t.Fatalf("a read-only scan must not create discovery state: %v", e)
	}
	if b, e := os.ReadFile(filepath.Join(vault, storeRel)); e != nil || string(b) != store {
		t.Fatalf("a read-only scan must preserve classifications: %v %s", e, b)
	}
}

func TestReadOnlyDiscoveryFailsForMissingRequestedEvidence(t *testing.T) {
	for _, requested := range [][]string{{"SVC-missing"}, {"SVC-probe", "SVC-missing"}} {
		vault, _ := evidenceFixture(t, t.TempDir())
		var out bytes.Buffer
		e := runDiscovery(options{vault: vault, repos: requested, at: "head", readOnly: true}, &out)
		if e == nil {
			t.Fatalf("an incomplete requested discovery cannot exit successfully: %s", out.String())
		}
		var result struct {
			Facts  []repoFacts `json:"facts"`
			Report runReport   `json:"report"`
		}
		if e := json.Unmarshal(out.Bytes(), &result); e != nil || len(result.Report.Failed) != 1 || len(result.Facts) != len(requested)-1 {
			t.Fatalf("partial evidence and its missing scope must remain available: %v %s", e, out.String())
		}
		if _, e := os.Stat(filepath.Join(vault, stateRel)); !os.IsNotExist(e) {
			t.Fatalf("an incomplete read-only scan must not persist state: %v", e)
		}
	}
}

func TestReadOnlyPartialDiscoveryUsesOnlyFreshFacts(t *testing.T) {
	root := t.TempDir()
	vault, _ := evidenceFixture(t, root)
	other := gitRepo(t, filepath.Join(root, "SVC-other"), map[string]string{"other.go": "package other\nconst Value = 1\n"})
	if _, e := gitOutput(other, "remote", "add", "origin", "https://github.com/acme/SVC-other.git"); e != nil {
		t.Fatal(e)
	}
	write(t, vault, "20-Repos/other.md", "---\naliases: [SVC-other]\n---\n# Other\n")
	if e := writeState(vault, "facts/SVC-other.json", repoFacts{Repo: "SVC-other", Note: "20-Repos/other.md", Commit: "old"}); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, repos: []string{"SVC-probe"}, at: "head", readOnly: true}, &out); e != nil {
		t.Fatal(e)
	}
	var result struct {
		Facts      []repoFacts  `json:"facts"`
		Comparison []comparison `json:"comparison"`
		Report     runReport    `json:"report"`
	}
	if e := json.Unmarshal(out.Bytes(), &result); e != nil || len(result.Facts) != 1 || len(result.Comparison) != 1 || result.Report.Comparison["notes_compared"] != 1 {
		t.Fatalf("a current scoped read must not merge unrequested historical facts: %v %s", e, out.String())
	}
}

func TestReadOnlyRequestedEmptyCheckoutIsIncomplete(t *testing.T) {
	root := t.TempDir()
	vault, _ := evidenceFixture(t, root)
	empty := filepath.Join(root, "SVC-empty")
	if b, e := exec.Command("git", "init", "-q", empty).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	if _, e := gitOutput(empty, "remote", "add", "origin", "https://github.com/acme/SVC-empty.git"); e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, repos: []string{"SVC-empty"}, at: "head", readOnly: true}, &out); e == nil {
		t.Fatalf("a requested checkout without a commit cannot establish fresh evidence: %s", out.String())
	}
}

func TestReadOnlyNoteCheckDoesNotPopulateCache(t *testing.T) {
	t.Setenv("KOS_NO_CACHE", "")
	t.Setenv("KOS_CACHE_DIR", t.TempDir())
	vault, _ := evidenceFixture(t, t.TempDir())
	var out bytes.Buffer
	if e := runCheck(options{vault: vault, notes: []string{"20-Repos/probe.md"}, readOnly: true}, &out); e != nil {
		t.Fatal(e, out.String())
	}
	if _, e := os.Stat(checkCacheDir()); !os.IsNotExist(e) {
		t.Fatalf("a read-only check must not create a cache: %v", e)
	}
}

func TestReadOnlyNoteCheckRechecksAnUnreadableSource(t *testing.T) {
	t.Setenv("KOS_NO_CACHE", "")
	t.Setenv("KOS_CACHE_DIR", t.TempDir())
	vault, repo := evidenceFixture(t, t.TempDir())
	first, e := checkNote(vault, "20-Repos/probe.md", "")
	if e != nil || !first.OK || first.Anchors["verified"] != 1 {
		t.Fatalf("initial source validation: %v %+v", e, first)
	}
	blob, e := gitOutput(repo, "rev-parse", "HEAD:probe.go")
	if e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(repo, ".git", "objects", blob[:2], blob[2:])); e != nil {
		t.Fatal(e)
	}
	// Losing an object does not change the source refs or the existing cache key.
	memoMu.Lock()
	delete(fingerprintMemo, vault)
	memoMu.Unlock()
	again, e := checkNoteMode(vault, "20-Repos/probe.md", "", false)
	if e == nil && (again.OK || again.Anchors["verified"] != 0) {
		t.Fatalf("read-only verification must not reuse a cached pass when the cited source is unreadable: %+v", again)
	}
	for _, base := range [][]byte{nil, []byte("---\naliases: [SVC-probe]\ncommit-analizado: " + first.Commit + "\n---\n# Probe\n\nThe implementation defines `Value`. [^e1]\n\n[^e1]: probe.go#L1-L2 — `Value`\n")} {
		if ok, issues, _, e := CheckNoteIntroduced(vault, "20-Repos/probe.md", base); e == nil && ok {
			t.Fatalf("publication cannot reuse a cached pass after losing source evidence: base=%v issues=%v fresh=%+v", base != nil, issues, again)
		}
	}
}

func TestReadOnlyPlatformCaptureDoesNotPersist(t *testing.T) {
	vault, _ := evidenceFixture(t, t.TempDir())
	fakeCLI(t, map[string]string{"gcloud": "[]", "bq ls": "[]"}, nil)
	var out bytes.Buffer
	if e := capturePlatform(options{vault: vault, provider: "gcp", scopes: []string{"acme-evidence-prd"}, readOnly: true}, &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), `"snapshots"`) || !strings.Contains(out.String(), `"status": "ok"`) {
		t.Fatal(out.String())
	}
	if _, e := os.Stat(filepath.Join(vault, platformRel)); !os.IsNotExist(e) {
		t.Fatalf("a read-only capture must not store a snapshot: %v", e)
	}
}

func TestNoOutputCannotErasePreviousPlatformEvidence(t *testing.T) {
	vault := t.TempDir()
	old := newSnapshot("gcp", "acme-evidence-prd")
	old.Status = "ok"
	old.Topics = []string{"projects/acme-evidence-prd/topics/probe"}
	old.Subscriptions = []platformSubscription{{Name: "projects/acme-evidence-prd/subscriptions/probe-sub", Topic: old.Topics[0]}}
	old.Resources = []platformResource{{Kind: "sql_db", Name: "probe-db", Aliases: []string{"probe.example.org"}}}
	old.Kinds = map[string]string{"messaging": "ok"}
	if e := saveSnapshot(vault, old); e != nil {
		t.Fatal(e)
	}
	fakeCLI(t, map[string]string{"gcloud": "", "bq ls": ""}, nil)
	failed := providers["gcp"].capture(old.Scope)
	if failed.Status != "error" {
		t.Fatalf("blank output must fail: %+v", failed)
	}
	if e := saveSnapshot(vault, failed); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(snapshotPath(vault, "gcp", old.Scope))
	var retained platformSnapshot
	if e != nil || json.Unmarshal(b, &retained) != nil || len(retained.Topics) != 1 || len(retained.RefreshFailed) == 0 {
		t.Fatalf("previous evidence must survive with the refresh failure: %v %s", e, b)
	}
	ix := buildPlatformIndex([]platformSnapshot{retained})
	if ix.scopes[old.key()].Status == "ok" || ix.typeOf(old.Topics[0]) != "" || len(ix.objectsNamed("probe-db")) != 0 || len(ix.subs) != 0 {
		t.Fatalf("retained historical evidence cannot confirm current resources: %+v", ix)
	}
	a := assembly{platform: ix}
	pending := a.pendingFor(repoFacts{Resources: []resource{{Name: old.Topics[0], Type: "message_topic"}}})
	if len(pending) != 1 || pending[0].Kind != "platform-access" {
		t.Fatalf("a failed current refresh must remain explicit: %+v", pending)
	}
	if e := saveSnapshot(vault, old); e != nil {
		t.Fatal(e)
	}
	snaps, e := loadSnapshots(vault)
	if e != nil || buildPlatformIndex(snaps).typeOf(old.Topics[0]) != "message_topic" || len(snaps[0].RefreshFailed) != 0 {
		t.Fatalf("a successful fresh read must restore the indexed evidence: %v %+v", e, snaps)
	}
}

func TestPlatformReadRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, message, e := runCLIContext(ctx, "git", "--version"); e == nil || !strings.Contains(message, "cancelled") {
		t.Fatalf("cancelled reads must fail explicitly: %v %s", e, message)
	}
}

func TestDiscoveryKeepsTheCommitOfTheInspectedSnapshot(t *testing.T) {
	if _, e := exec.LookPath("sh"); e != nil {
		t.Skip("shell fixture unavailable")
	}
	vault, repo := evidenceFixture(t, t.TempDir())
	inspected, _ := resolveCommit(repo, "HEAD")
	write(t, repo, "later.go", "package probe\nconst Later = 2\n")
	for _, args := range [][]string{{"add", "later.go"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "later"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
	}
	newer, _ := resolveCommit(repo, "HEAD")
	if _, e := gitOutput(repo, "update-ref", "refs/heads/main", inspected); e != nil {
		t.Fatal(e)
	}
	realGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	bin := t.TempDir()
	// Advance the reference after cat-file has finished the immutable snapshot.
	// This models another process fetching or committing while discovery reads.
	shim := "#!/bin/sh\n" + strconv.Quote(realGit) + " \"$@\"\nrc=$?\nfor arg in \"$@\"; do\n  if [ \"$arg\" = cat-file ]; then\n    " + strconv.Quote(realGit) + " -C " + strconv.Quote(repo) + " update-ref refs/heads/main " + newer + "\n  fi\ndone\nexit $rc\n"
	if e := os.WriteFile(filepath.Join(bin, "git"), []byte(shim), 0o755); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out bytes.Buffer
	if e := runDiscovery(options{vault: vault, at: "head", readOnly: true}, &out); e != nil {
		t.Fatal(e)
	}
	var result struct {
		Facts []repoFacts `json:"facts"`
	}
	if e := json.Unmarshal(out.Bytes(), &result); e != nil || len(result.Facts) != 1 || result.Facts[0].Commit != inspected {
		t.Fatalf("facts must retain the inspected commit despite the moved reference: %v %s", e, out.String())
	}
}
