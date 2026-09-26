package cell

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knowledge-os/internal/config"
	"knowledge-os/internal/kernel"
)

func run(t *testing.T, stdin string, args ...string) (map[string]any, string, error) {
	t.Helper()
	t.Setenv("KOS_VAULTS_FILE", filepath.Join(t.TempDir(), "vaults.json"))
	var out, questions bytes.Buffer
	e := Run(context.Background(), args, strings.NewReader(stdin), &out, &questions)
	m := map[string]any{}
	_ = json.Unmarshal(out.Bytes(), &m)
	return m, questions.String(), e
}

func TestInitWithFlagsWritesAValidCell(t *testing.T) {
	v := filepath.Join(t.TempDir(), "vault")
	res, questions, e := run(t, "", "init", "--vault", v, "--yes", "--cell-name", "Commerce", "--purpose", "Orders.",
		"--system", "orders:Orders", "--system", "Order Returns", "--tracker", "https://acme.atlassian.net/jira/ORD/",
		"--platform", "gcp", "--reference-branch", "develop", "--locale", "en", "--disable-topics")
	if e != nil || res["status"] != "initialized" || questions != "" {
		t.Fatalf("init: %v %v %q", e, res, questions)
	}
	inst, e := config.LoadInstance(v)
	if e != nil {
		t.Fatal(e)
	}
	systems := inst["systems"].([]any)
	if len(systems) != 2 || systems[1].(map[string]any)["id"] != "order-returns" {
		t.Fatalf("systems %v", systems)
	}
	tracker := inst["trackers"].([]any)[0].(map[string]any)
	if tracker["provider"] != "jira" || tracker["url"] != "https://acme.atlassian.net/jira/ORD" {
		t.Fatalf("tracker %v", tracker)
	}
	for _, p := range []string{"00-Home.md", "10-Sistemas/Order Returns.md", "20-Repos/orders/.gitkeep", "30-Flujos/Flujos.md", "AGENTS.md", kernel.LockName} {
		if _, e := os.Stat(filepath.Join(v, p)); e != nil {
			t.Fatalf("missing %s", p)
		}
	}
	if _, e := os.Stat(filepath.Join(v, "25-Topics")); !os.IsNotExist(e) {
		t.Fatal("topics disabled: no 25-Topics folder")
	}
	if o := config.Orientation(v, inst); o["ready"] != true {
		t.Fatalf("orientation %v", o)
	}
	if _, _, e := run(t, "", "init", "--vault", v, "--yes", "--system", "X"); e == nil || !strings.Contains(e.Error(), "already installed") {
		t.Fatalf("an installed vault is refused: %v", e)
	}
}

func TestInitAsksOnStderrAndCancelsWithoutWriting(t *testing.T) {
	answers := []string{"Commerce", "Order fulfillment.", "Orders, order-returns:Returns", "acme", "APP01-, APP02-",
		"develop,main", "gcp, aws", "https://acme.atlassian.net/jira/software/projects/ORD", "en"}
	for i := range answers {
		v := filepath.Join(t.TempDir(), "vault")
		_, _, e := run(t, strings.Join(answers[:i], "\n"), "init", "--vault", v)
		if e != errCancelled {
			t.Fatalf("end of input after %d answers: %v", i, e)
		}
		if _, e := os.Stat(v); !os.IsNotExist(e) {
			t.Fatalf("a cancelled init writes nothing (after %d answers)", i)
		}
	}
	v := filepath.Join(t.TempDir(), "vault")
	res, questions, e := run(t, strings.Join(answers, "\n")+"\n", "init", "--vault", v)
	if e != nil || res["status"] != "initialized" {
		t.Fatalf("init: %v %v", e, res)
	}
	last := -1
	for _, label := range []string{"Cell name", "What the cell owns", "Systems it owns", "GitHub organization", "Repository name prefixes",
		"Reference branches", "Clouds the systems", "Issue tracker URLs", "Language of the notes"} {
		at := strings.Index(questions, label)
		if at <= last {
			t.Fatalf("question %q missing or out of order:\n%s", label, questions)
		}
		last = at
	}
	inst, _ := config.LoadInstance(v)
	src := inst["sources"].(map[string]any)
	if src["github_org"] != "acme" || len(src["repo_prefixes"].([]any)) != 2 || src["reference_branch_order"].([]any)[0] != "develop" {
		t.Fatalf("sources %v", src)
	}
	if p := inst["platform"].(map[string]any)["providers"].([]any); len(p) != 2 || p[1] != "aws" {
		t.Fatalf("providers %v", p)
	}
}

func TestInitEnterKeepsDefaults(t *testing.T) {
	v := filepath.Join(t.TempDir(), "vault")
	if _, _, e := run(t, strings.Repeat("\n", 9), "init", "--vault", v); e != nil {
		t.Fatal(e)
	}
	inst, _ := config.LoadInstance(v)
	if inst["cell"].(map[string]any)["name"] != "Cell" || inst["locale"].(map[string]any)["notes"] != "es" ||
		inst["systems"].([]any)[0].(map[string]any)["id"] != "platform" {
		t.Fatalf("defaults %v", inst)
	}
	b, _ := os.ReadFile(filepath.Join(v, "00-Home.md"))
	if !strings.Contains(string(b), "kos config status") {
		t.Fatal("Home carries the configuration guidance in the cell's language")
	}
}

func TestInitRejectsInvalidAnswersBeforeWriting(t *testing.T) {
	v := filepath.Join(t.TempDir(), "vault")
	if _, _, e := run(t, "", "init", "--vault", v, "--yes"); e == nil || !strings.Contains(e.Error(), "--system") {
		t.Fatalf("--yes needs a system: %v", e)
	}
	res, _, e := run(t, "", "init", "--vault", v, "--yes", "--system", "S", "--platform", "oracle")
	if e == nil || res["status"] != "invalid-instance" {
		t.Fatalf("unknown cloud: %v %v", e, res)
	}
	if _, e := os.Stat(v); !os.IsNotExist(e) {
		t.Fatal("an invalid answer writes nothing")
	}
}

func TestAdoptKeepsNotesAndRefusesChangedKernelFiles(t *testing.T) {
	v := t.TempDir()
	if _, _, e := run(t, "", "adopt", "--vault", v); e == nil {
		t.Fatal("an empty directory is not adoptable")
	}
	os.MkdirAll(filepath.Join(v, "10-Sistemas"), 0o755)
	os.WriteFile(filepath.Join(v, "00-Home.md"), []byte("# Our home\n"), 0o644)
	os.WriteFile(filepath.Join(v, "10-Sistemas", "S.md"), []byte("# S\n"), 0o644)
	if _, _, e := run(t, "", "adopt", "--vault", v); e == nil || !strings.Contains(e.Error(), "instance.yaml") {
		t.Fatalf("adopt needs the identity: %v", e)
	}
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\ncell:\n  name: C\n  purpose: p\nsystems:\n  - id: s\n    name: S\n"), 0o644)
	os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("# our own router\n"), 0o644)
	res, _, e := run(t, "", "adopt", "--vault", v)
	if e == nil || res["status"] != "ownership-conflict" {
		t.Fatalf("a changed kernel file is a conflict: %v %v", e, res)
	}
	if res, _, e = run(t, "", "adopt", "--vault", v, "--force"); e != nil || res["status"] != "adopted" {
		t.Fatalf("adopt --force: %v %v", e, res)
	}
	if b, _ := os.ReadFile(filepath.Join(v, "00-Home.md")); string(b) != "# Our home\n" {
		t.Fatal("adopt never rewrites Home")
	}
}

func TestDoctorReportsDriftAndStrictFails(t *testing.T) {
	v := filepath.Join(t.TempDir(), "vault")
	if _, _, e := run(t, "", "init", "--vault", v, "--yes", "--system", "S", "--locale", "en"); e != nil {
		t.Fatal(e)
	}
	res, _, e := run(t, "", "doctor", "--vault", v)
	if e != nil || res["state"] != "installed" || res["kernel_current"] != true || res["instance"].(map[string]any)["status"] != "valid" {
		t.Fatalf("doctor: %v %v", e, res)
	}
	if res["start_here"].([]any)[0] != "00-Home.md" || res["personal_instructions"].(map[string]any)["ignored"] != true {
		t.Fatalf("orientation and personal file: %v", res)
	}
	os.WriteFile(filepath.Join(v, "AGENTS.md"), []byte("# edited\n"), 0o644)
	res, _, e = run(t, "", "doctor", "--vault", v)
	if e != nil || len(res["drift"].([]any)) != 1 {
		t.Fatalf("drift is reported, not an error without --strict: %v %v", e, res)
	}
	if _, _, e = run(t, "", "doctor", "--vault", v, "--strict"); e == nil || !strings.Contains(e.Error(), "changed locally") {
		t.Fatalf("--strict fails on drift: %v", e)
	}
	os.WriteFile(filepath.Join(v, "instance.yaml"), []byte("version: 1\n"), 0o644)
	if res, _, e = run(t, "", "doctor", "--vault", v); e == nil || res["instance"].(map[string]any)["status"] != "invalid" {
		t.Fatalf("an invalid identity fails: %v %v", e, res)
	}
}
