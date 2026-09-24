package handoff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	family := "example-123--repository"
	dir := filepath.Join(root, storeName, family)
	if err := os.MkdirAll(filepath.Join(dir, "history"), 0700); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	event := historyEvent{Schema: 1, ID: id, Revision: "v0001", CreatedAt: "2026-09-24T00:00:00Z", Reason: "initial", Changed: []any{"context.md", "work-item.md", "scope.md"}, Unchanged: []string{}}
	eventYAML, err := yaml.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	eventRaw := append([]byte("---\n"), eventYAML...)
	eventRaw = append(eventRaw, []byte("---\n\nInitial export.\n")...)
	write(t, filepath.Join(dir, "history", "v0001.md"), eventRaw)
	m := manifest{Schema: 2, ID: id, Family: family, Revision: "v0001", Files: map[string]fileRecord{}}
	m.Source.Investigation = "investigation-example"
	m.Source.UpdatedAt = "2026-09-24T00:00:00Z"
	m.Source.Story = "S-001"
	m.WorkItem = map[string]any{"tracker-id": "tracker", "provider": "jira", "tracker-url": "https://tracker.invalid", "reference": "EX-123", "url": "https://tracker.invalid/browse/EX-123", "updated-at": "2026-09-24T00:00:00Z", "captured-at": "2026-09-24T00:00:00Z", "freshness": "current", "snapshot-source": "user-supplied-export"}
	m.Repository.Remote = "example.test/team/repository"
	for _, name := range []string{"START.md", "context.md", "work-item.md", "scope.md"} {
		raw := []byte("# " + name + "\n")
		write(t, filepath.Join(dir, name), raw)
		m.Files[name] = fileRecord{SHA: digest(raw), SemanticSHA: digest(raw)}
	}
	m.History.Path = "history/v0001.md"
	m.History.SHA = digest(eventRaw)
	manifestRaw, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "handoff.yaml"), manifestRaw)
	active := map[string]any{"schema-version": 2, "investigation-id": m.Source.Investigation, "handoffs": []Entry{{ID: id, Family: family, Revision: "v0001", Manifest: family + "/handoff.yaml", ActivatedAt: "2026-09-24T00:00:00Z", State: "active"}}}
	activeRaw, err := yaml.Marshal(active)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, storeName, "ACTIVE.yaml"), activeRaw)
	return root, dir
}

func write(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestInspectStoredIntegrity(t *testing.T) {
	root, _ := fixture(t)
	result, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "integrity-checked" || len(result.Handoffs) != 1 || result.Scope != "stored-byte-integrity" || len(result.Unchecked) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRejectCorruptedFile(t *testing.T) {
	root, dir := fixture(t)
	write(t, filepath.Join(dir, "context.md"), []byte("changed\n"))
	if _, err := Inspect(root); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("expected integrity error, got %v", err)
	}
}

func TestRejectCorruptedHistory(t *testing.T) {
	root, dir := fixture(t)
	write(t, filepath.Join(dir, "history", "v0001.md"), []byte("changed\n"))
	if _, err := Inspect(root); err == nil || !strings.Contains(err.Error(), "history hash") {
		t.Fatalf("expected history error, got %v", err)
	}
}

func TestRejectPendingTransaction(t *testing.T) {
	for _, name := range []string{".APPLY.transaction", ".APPLY.transaction.prepare"} {
		t.Run(name, func(t *testing.T) {
			root, _ := fixture(t)
			if err := os.Mkdir(filepath.Join(root, storeName, name), 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := Inspect(root); err == nil || !strings.Contains(err.Error(), "pending") {
				t.Fatalf("expected pending error, got %v", err)
			}
		})
	}
}

func TestRejectSymlinkDocument(t *testing.T) {
	root, dir := fixture(t)
	path := filepath.Join(dir, "context.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "scope.md"), path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Inspect(root); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestRejectTraversal(t *testing.T) {
	root, _ := fixture(t)
	raw, err := os.ReadFile(filepath.Join(root, storeName, "ACTIVE.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, storeName, "ACTIVE.yaml"), bytes.ReplaceAll(raw, []byte("example-123--repository"), []byte("../escape")))
	if _, err := Inspect(root); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestRejectExtraHistory(t *testing.T) {
	root, dir := fixture(t)
	write(t, filepath.Join(dir, "history", "extra.md"), []byte("extra"))
	if _, err := Inspect(root); err == nil {
		t.Fatal("accepted extra history")
	}
}

func TestLegacyRegistryReadOnly(t *testing.T) {
	root, dir := fixture(t)
	family := filepath.Base(dir)
	legacy := map[string]any{"schema-version": 1, "handoff-id": strings.Repeat("a", 64), "family": family, "revision": "v0001", "manifest": family + "/handoff.yaml", "activated-at": "2026-09-24T00:00:00Z"}
	raw, err := yaml.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, storeName, "ACTIVE.yaml")
	write(t, path, raw)
	result, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Investigation != "investigation-example" {
		t.Fatal(result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, after) {
		t.Fatal("inspection mutated legacy registry")
	}
}

func TestInactive(t *testing.T) {
	result, err := Inspect(t.TempDir())
	if err != nil || result.Status != "inactive" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestLifecycleRequiresArguments(t *testing.T) {
	for _, verb := range []string{"apply", "set-state", "create-worktree", "validate"} {
		var out bytes.Buffer
		if err := Run([]string{verb}, &out); err == nil {
			t.Fatalf("accepted %s", verb)
		}
	}
}

func TestRejectDuplicateYAMLKeys(t *testing.T) {
	var entry Entry
	if err := decode([]byte("family: one\nfamily: two\n"), &entry); err == nil {
		t.Fatal("accepted duplicate keys")
	}
}

func TestRejectMultipleYAMLDocuments(t *testing.T) {
	var entry Entry
	if err := decode([]byte("family: one\n---\nfamily: two\n"), &entry); err == nil {
		t.Fatal("accepted multiple documents")
	}
}
