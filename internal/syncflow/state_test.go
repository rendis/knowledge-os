package syncflow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func call(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var b bytes.Buffer
	if e := Run(args, &b); e != nil {
		t.Fatal(e)
	}
	v, e := decode(b.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	return obj(v)
}
func fixture(t *testing.T, p string) map[string]any {
	t.Helper()
	v, e := readJSON(filepath.Join("testdata", p))
	if e != nil {
		t.Fatal(e)
	}
	return obj(v)
}
func saveJSON(t *testing.T, p string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	write(t, p, string(b))
}
func TestNativeSyncBeginCheckpointSealStatus(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	args := []string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", "APP00000-source-adapter", strings.Repeat("a", 40), "--package", "APP00000-target-service", strings.Repeat("b", 40)}
	start := call(t, args...)
	id := str(start["run_id"])
	if start["next_command"] != "checkpoint-package" || start["reused"] != false {
		t.Fatal(start)
	}
	if call(t, args...)["reused"] != true {
		t.Fatal("begin not idempotent")
	}
	for _, name := range []string{"APP00000-source-adapter", "APP00000-target-service"} {
		a := filepath.Join("testdata", name+".json")
		out := call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", a)
		if out["status"] != "pass" {
			t.Fatal(out)
		}
		if call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", a)["reused"] != true {
			t.Fatal("checkpoint not idempotent")
		}
	}
	status := call(t, "status", "--state-root", root, "--run-id", id)
	if status["next_command"] != "seal-gate" {
		t.Fatal(status)
	}
	sealed := call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", "testdata/gate.json")
	if sealed["code"] != "gate-sealed" || sealed["next_command"] != "validate-unit" {
		t.Fatal(sealed)
	}
	replay := call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", "testdata/gate.json")
	if replay["reused"] != true {
		t.Fatal(replay)
	}
	if call(t, "status", "--state-root", root, "--run-id", id)["receipt_digest"] != sealed["receipt_digest"] {
		t.Fatal("status unstable")
	}
	p := filepath.Join(root, "active", id, "packages", "APP00000-source-adapter", "artifact.json")
	write(t, p, "{}")
	var out bytes.Buffer
	if e := Run([]string{"status", "--state-root", root, "--run-id", id}, &out); e == nil {
		t.Fatal("accepted tampered checkpoint")
	}
}
func TestPackageLineageAndGateAuthority(t *testing.T) {
	p := fixture(t, "APP00000-source-adapter.json")
	if e := validatePackage(p); e != nil {
		t.Fatal(e)
	}
	obj(p["analysis"])["claims"] = []any{}
	d, _ := digest(p["analysis"])
	obj(p["validation"])["analysis_digest"] = d
	obj(p["review"])["analysis_digest"] = d
	d, _ = digest(p["review"])
	obj(p["validation"])["review_digest"] = d
	if e := validatePackage(p); e == nil {
		t.Fatal("accepted claims not bound to analysis")
	}
	g := fixture(t, "gate.json")
	gr := obj(arr(g["write_groups"])[0])
	obj(arr(gr["grants"])[0])["claim_id"] = "unreviewed"
	if e := validateGate(g); e == nil {
		t.Fatal("accepted unreviewed grant")
	}
}
func TestSyncLockAndInvalidState(t *testing.T) {
	root := t.TempDir()
	l := flock.New(filepath.Join(root, ".native.lock"))
	if e := l.Lock(); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	e := Run([]string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo=" + strings.Repeat("b", 40)}, &b)
	l.Unlock()
	if e == nil || e.Error() != "sync-state-busy" {
		t.Fatalf("lock: %v", e)
	}
	out := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo="+strings.Repeat("b", 40))
	id := str(out["run_id"])
	p := filepath.Join(root, "active", id, "run.json")
	v, e := readJSON(p)
	if e != nil {
		t.Fatal(e)
	}
	obj(v)["inventory_digest"] = strings.Repeat("c", 64)
	saveJSON(t, p, v)
	if e = Run([]string{"status", "--state-root", root, "--run-id", id}, &b); e == nil {
		t.Fatal("accepted fingerprint drift")
	}
}
func TestStateSymlinkRefused(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()
	if e := os.Symlink(elsewhere, filepath.Join(root, "active")); e != nil {
		t.Skip(e)
	}
	var b bytes.Buffer
	if e := Run([]string{"begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo=" + strings.Repeat("b", 40)}, &b); e == nil {
		t.Fatal("followed state symlink")
	}
	entries, e := os.ReadDir(elsewhere)
	if e != nil || len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}

func TestPublicDispatchReturnsMutationErrors(t *testing.T) {
	root := t.TempDir()
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("a", 64), "--package", "repo="+strings.Repeat("b", 40))
	id := str(start["run_id"])
	for _, cmd := range []string{"validate-unit", "review-unit", "apply-unit", "close"} {
		var b bytes.Buffer
		e := Run([]string{cmd, "--state-root", root, "--run-id", id}, &b)
		if e == nil {
			t.Fatalf("%s swallowed mutation error: %s", cmd, b.String())
		}
	}
}

func TestPermissionIsNotCredentialButNestedAssignmentIs(t *testing.T) {
	for _, s := range []string{"id-token: write", "permissions: {id-token: none}", "id-token:\tread"} {
		if sensitive(s) {
			t.Fatalf("permission falsely classified: %s", s)
		}
	}
	for _, s := range []string{"id-token: write,token: secretvalue", "xid-token: write", "id-token: writer", "token: secretvalue"} {
		if !sensitive(s) {
			t.Fatalf("credential assignment missed: %s", s)
		}
	}
}

func TestCheckpointDistinguishesRepositoryPathFromLocalAbsolutePath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		path      string
		wantError bool
	}{
		{name: "git-relative-mobile-path", path: "Mobile/Users/Session/UserSession.swift"},
		{name: "local-macos-path", path: "/Users/alice/source/UserSession.swift", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			name := "APP00000-source-adapter"
			start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", name, strings.Repeat("a", 40))
			packageValue := fixture(t, name+".json")
			obj(arr(obj(packageValue["analysis"])["claims"])[0])["statement"] = "Changed source: " + tc.path
			analysisDigest, _ := digest(packageValue["analysis"])
			obj(packageValue["review"])["analysis_digest"] = analysisDigest
			obj(packageValue["validation"])["analysis_digest"] = analysisDigest
			reviewDigest, _ := digest(packageValue["review"])
			obj(packageValue["validation"])["review_digest"] = reviewDigest
			artifact := filepath.Join(t.TempDir(), "package.json")
			saveJSON(t, artifact, packageValue)

			var output bytes.Buffer
			err := Run([]string{"checkpoint-package", "--state-root", root, "--run-id", str(start["run_id"]), "--repository", name, "--artifact", artifact}, &output)
			if tc.wantError {
				if err == nil || err.Error() != "checkpoint-sensitive-content" {
					t.Fatalf("accepted local absolute path: %v (%s)", err, output.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("rejected Git-relative path: %v (%s)", err, output.String())
			}
		})
	}
}

func TestAbandonEmptyArchivesExactRunAndAllowsNewBegin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	name := "APP00000-source-adapter"
	inventory := strings.Repeat("c", 64)
	oid := strings.Repeat("a", 40)
	start := call(t, "begin", "--state-root", root, "--inventory-digest", inventory, "--package", name, oid)
	id := str(start["run_id"])
	runPath := filepath.Join(root, "active", id, "run.json")
	runValue, err := readJSON(runPath)
	if err != nil {
		t.Fatal(err)
	}
	run := obj(runValue)
	run["tool_digest"] = strings.Repeat("f", 64)
	run["fingerprint"], err = fingerprint(str(run["tool_digest"]), str(run["inventory_digest"]), arr(run["packages"]))
	if err != nil {
		t.Fatal(err)
	}
	saveJSON(t, runPath, run)
	original, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err = Run([]string{"abandon-empty", "--state-root", root, "--run-id", id}, &output); err != nil {
		t.Fatal(err)
	}
	resultValue, err := decode(output.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	result := obj(resultValue)
	if result["code"] != "run-abandoned" || result["reused"] != false {
		t.Fatal(result)
	}
	if _, err = os.Stat(filepath.Join(root, "active", id)); !os.IsNotExist(err) {
		t.Fatal("active run was not retired")
	}
	archived := filepath.Join(root, "retired", id+"-abandoned")
	preserved, err := os.ReadFile(filepath.Join(archived, "run.json"))
	if err != nil || !bytes.Equal(preserved, original) {
		t.Fatal("archived run changed")
	}
	if _, found, err := archivedAbandonment(root, id); err != nil || !found {
		t.Fatalf("abandonment receipt invalid: found=%v err=%v", found, err)
	}

	output.Reset()
	if err = Run([]string{"abandon-empty", "--state-root", root, "--run-id", id}, &output); err != nil {
		t.Fatal(err)
	}
	retryValue, err := decode(output.Bytes())
	if err != nil || obj(retryValue)["reused"] != true {
		t.Fatalf("abandonment retry was not idempotent: %v %v", retryValue, err)
	}
	newRun := call(t, "begin", "--state-root", root, "--inventory-digest", inventory, "--package", name, oid)
	if newRun["run_id"] == id || newRun["reused"] != false {
		t.Fatalf("new begin reused abandoned run: %v", newRun)
	}
}

func TestAbandonEmptyRefusesCheckpointedRunAndPreservesEvidence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	name := "APP00000-source-adapter"
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", name, strings.Repeat("a", 40))
	id := str(start["run_id"])
	call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", filepath.Join("testdata", name+".json"))
	artifact := filepath.Join(root, "active", id, "packages", name, "artifact.json")
	original, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = Run([]string{"abandon-empty", "--state-root", root, "--run-id", id}, &output)
	if err == nil || err.Error() != "run-not-abandonable" {
		t.Fatalf("checkpointed run abandonment: %v (%s)", err, output.String())
	}
	preserved, readErr := os.ReadFile(artifact)
	if readErr != nil || !bytes.Equal(preserved, original) {
		t.Fatal("checkpoint evidence was not preserved")
	}
	if _, statErr := os.Stat(filepath.Join(root, "retired", id+"-abandoned")); !os.IsNotExist(statErr) {
		t.Fatal("refused run was archived")
	}
}

func TestAbandonEmptyRefusesUnexpectedRunArtifact(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", "repo", strings.Repeat("a", 40))
	id := str(start["run_id"])
	evidence := filepath.Join(root, "active", id, "worker-evidence.json")
	write(t, evidence, "preserve me")
	var output bytes.Buffer
	err := Run([]string{"abandon-empty", "--state-root", root, "--run-id", id}, &output)
	if err == nil || err.Error() != "run-not-abandonable" {
		t.Fatalf("unexpected artifact abandonment: %v (%s)", err, output.String())
	}
	if body, readErr := os.ReadFile(evidence); readErr != nil || string(body) != "preserve me" {
		t.Fatal("unexpected artifact was not preserved")
	}
}

func TestMissingCheckpointMarksSourceStale(t *testing.T) {
	root := t.TempDir()
	name := "APP00000-source-adapter"
	start := call(t, "begin", "--state-root", root, "--inventory-digest", strings.Repeat("c", 64), "--package", name, strings.Repeat("a", 40))
	id := str(start["run_id"])
	call(t, "checkpoint-package", "--state-root", root, "--run-id", id, "--repository", name, "--artifact", "testdata/"+name+".json")
	p := fixture(t, name+".json")
	gp := filepath.Join(t.TempDir(), "gate.json")
	saveJSON(t, gp, p["gate"])
	call(t, "seal-gate", "--state-root", root, "--run-id", id, "--gate", gp)
	if e := os.Remove(filepath.Join(root, "active", id, "gate.json")); e != nil {
		t.Fatal(e)
	}
	call(t, "status", "--state-root", root, "--run-id", id)
	if _, e := os.Stat(filepath.Join(root, "active", id, "gate.json")); e != nil {
		t.Fatal("gate history was not restored", e)
	}
	if e := os.Remove(filepath.Join(root, "active", id, "packages", name, "artifact.json")); e != nil {
		t.Fatal(e)
	}
	result := call(t, "status", "--state-root", root, "--run-id", id)
	if result["next_command"] != "checkpoint-package" || obj(arr(result["units"])[0])["stale_reason"] != "source" {
		t.Fatal(result)
	}
}
