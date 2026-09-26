package discover

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0o644); e != nil {
		t.Fatal(e)
	}
}

func gitRepo(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	if e := os.MkdirAll(root, 0o755); e != nil {
		t.Fatal(e)
	}
	for rel, c := range files {
		write(t, root, rel, c)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s", args, b)
		}
	}
	return root
}

func scan(t *testing.T, name, root string) *repoScan {
	t.Helper()
	s, e := scanRepository(repoInput{Name: name, Path: root, Ref: "HEAD"})
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestImportsAreLanguageRulesOnly(t *testing.T) {
	cases := map[string][]string{
		"go":   imports("go", "package x\nimport (\n\t\"fmt\"\n\tps \"cloud.google.com/go/pubsub\"\n)\n"),
		"java": imports("java", "package a.b;\nimport org.springframework.web.bind.annotation.GetMapping;\nimport static java.util.List.of;\n"),
		"js":   imports("js", "import axios from 'axios';\nconst f = require(\"fs\");\nexport * from '@scope/pkg/sub';\nimport('lazy')\n"),
		"py":   imports("py", "from google.cloud import pubsub_v1\nimport os, json as j\n"),
	}
	want := map[string]string{"go": "cloud.google.com/go/pubsub", "java": "org.springframework.web.bind.annotation.GetMapping", "js": "@scope/pkg/sub", "py": "google.cloud"}
	for lang, got := range cases {
		if !strings.Contains(strings.Join(got, " "), want[lang]) {
			t.Errorf("%s imports %v missing %s", lang, got, want[lang])
		}
	}
	c := langContext{m: manifest{goModules: []string{"example.com/svc"}, goRequires: []string{"cloud.google.com/go/pubsub"}, npm: map[string]bool{"axios": true}}, pyLocal: map[string]bool{"app": true}}
	for _, x := range []struct{ lang, spec, kind, fam string }{
		{"go", "example.com/svc/internal/x", "own", ""},
		{"go", "net/http", "std", "net/http"},
		{"go", "cloud.google.com/go/pubsub/v2/apiv1", "ext", "cloud.google.com/go/pubsub"},
		{"js", "http", "std", "http"},
		{"js", "axios", "ext", "axios"},
		{"js", "@alias/thing", "own", ""},
		{"py", "app.models", "own", ""},
		{"py", "socket", "std", "socket"},
		{"java", "java.net.http.HttpClient", "std", "java.net.http"},
		{"java", "lombok.Data", "ext", "lombok"},
	} {
		got := classifyImport(x.lang, x.spec, c)
		if got.kind != x.kind || got.family != x.fam {
			t.Errorf("%s %s = %+v, want %s %s", x.lang, x.spec, got, x.kind, x.fam)
		}
	}
}

func TestConfigParsingByFormat(t *testing.T) {
	yaml := "apiVersion: v1\nkind: ConfigMap\ndata:\n  GCP_PUBSUB_TOPIC_OUT: projects/p-prd/topics/orders-cl-outbound\n  PORT: \"8080\"\nspec:\n  env:\n    - name: SUB\n      value: orders-cl-inbound-sub\n"
	got := parseConfig("k8s/configmap.yaml", []byte(yaml))
	found := map[string]string{}
	for _, e := range got {
		found[e.KeyPath] = e.Value + "|" + e.Context
	}
	if found["data.GCP_PUBSUB_TOPIC_OUT"] != "projects/p-prd/topics/orders-cl-outbound|ConfigMap" || found["spec.env.SUB"] != "orders-cl-inbound-sub|ConfigMap" {
		t.Fatalf("yaml entries %v", found)
	}
	if _, ok := found["data.PORT"]; ok {
		t.Fatal("scalar noise must be dropped")
	}
	hcl := "module \"sub-cl\" {\n  source = \"./modules/pubsub/subscription\"\n  name_subscription = \"orders.confirmed\"\n  dead_letter_topic = var.env == \"prd\" ? \"orders.dlq\" : \"orders.dlq-dev\"\n  microservice = \"orders-command\"\n}\n"
	tf := map[string]bool{}
	for _, e := range parseConfig("pubsub/subs.tf", []byte(hcl)) {
		tf[e.Key+"="+e.Value] = true
		if !strings.Contains(e.Context, "source=./modules/pubsub/subscription") {
			t.Errorf("hcl context lost: %+v", e)
		}
	}
	for _, w := range []string{"name_subscription=orders.confirmed", "dead_letter_topic=orders.dlq", "dead_letter_topic=orders.dlq-dev", "microservice=orders-command"} {
		if !tf[w] {
			t.Errorf("missing %s in %v", w, tf)
		}
	}
	if tf["dead_letter_topic=prd"] || tf["source=./modules/pubsub/subscription"] {
		t.Errorf("hcl condition or meta-argument leaked: %v", tf)
	}
	flags := parseConfig("cloudfunctions/production/flags", []byte("  --trigger-topic=picking-topic \\\n  --region=us-east4 \\\n"))
	if len(flags) == 0 || flags[0].Key != "trigger-topic" || flags[0].Value != "picking-topic" {
		t.Fatalf("flags %v", flags)
	}
	env := parseConfig("kustomization/production/env-cl", []byte("TOPIC=orders-cl-outbound\n# comment=x\n"))
	if len(env) != 1 || env[0].Value != "orders-cl-outbound" {
		t.Fatalf("dotenv without extension %v", env)
	}
}

func TestScanAtCommitAndLibraries(t *testing.T) {
	dir := t.TempDir()
	lib := gitRepo(t, filepath.Join(dir, "LIB-common"), map[string]string{
		"go.mod":           "module example.com/common\n\nrequire cloud.google.com/go/pubsub v1.2.3\n",
		"publisher/pub.go": "package publisher\nimport \"cloud.google.com/go/pubsub\"\nvar _ = pubsub.NewClient\n",
		"log/log.go":       "package log\nimport \"fmt\"\n",
	})
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod":                 "module example.com/orders\n\nrequire (\n\texample.com/common v0.1.0\n\tcloud.google.com/go/firestore v1.0.0\n)\n",
		"cmd/main.go":            "package main\nimport (\n\t\"example.com/common/publisher\"\n\t\"net/http\"\n)\nconst topic = \"orders-cl-outbound\"\n",
		"k8s/prod/env-cl":        "TOPIC=orders-cl-outbound\nSUB=projects/p-prd/subscriptions/orders-cl-inbound-sub\n",
		"k8s/prod/secret.env-cl": "API_KEY=supersecret\n",
		"internal/x_test.go":     "package x\nimport \"github.com/stretchr/testify\"\n",
	})
	// A later commit must not leak into the analyzed snapshot.
	write(t, svc, "late.go", "package main\nimport \"github.com/late/dep\"\n")
	s := scan(t, "SVC-orders", svc)
	l := scan(t, "LIB-common", lib)
	resolveLibraries([]*repoScan{s, l})
	if s.deps["go:cloud.google.com/go/pubsub"] == nil || s.deps["go:cloud.google.com/go/pubsub"].Via != "LIB-common" {
		t.Fatalf("library pubsub not resolved: %+v", s.deps)
	}
	if s.deps["go:log"] != nil || s.deps["go:fmt"] != nil {
		t.Fatal("unimported library packages must not leak")
	}
	if s.deps["go:github.com/late/dep"] != nil || s.deps["go:github.com/stretchr/testify"] != nil {
		t.Fatal("working tree changes and tests must not be scanned")
	}
	for _, e := range s.entries {
		if e.Key == "API_KEY" && (!e.Secret || e.Value != "<redacted>") {
			t.Fatal("secret file values must be redacted")
		}
	}
	declared := strings.Join(s.code.Manifest.declared, " ")
	if !strings.Contains(declared, "go:cloud.google.com/go/firestore") {
		t.Fatalf("manifest declarations %v", declared)
	}
}

func TestAssemblyPlatformPendingAndComparison(t *testing.T) {
	dir := t.TempDir()
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod":          "module example.com/orders\n\nrequire cloud.google.com/go/pubsub v1.0.0\n",
		"main.go":         "package main\nimport \"cloud.google.com/go/pubsub\"\nvar _ = pubsub.NewClient\nconst ev = \"orderConfirmed\"\n",
		"k8s/prod/env-cl": "SUB=orders-cl-inbound-sub\nOUT=projects/p-prd/topics/orders-cl-outbound\nGHOST=projects/p-prd/topics/ghost-topic\n",
	})
	iac := gitRepo(t, filepath.Join(dir, "IAC-platform"), map[string]string{
		"pubsub/subs.tf": "module \"x\" {\n  source = \"./modules/pubsub/subscription\"\n  name_subscription = \"orders.audit-sub\"\n  microservice = \"svc-orders\"\n}\n",
	})
	st := &store{Dependencies: map[string]judgment{"go:cloud.google.com/go/pubsub": {Choice: "messaging", Confidence: 1}},
		ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	scans := []*repoScan{scan(t, "SVC-orders", svc), scan(t, "IAC-platform", iac)}
	for _, s := range scans {
		for _, e := range s.entries {
			st.ConfigKeys[keySignature(e)] = judgment{Choice: "other", Confidence: 0.9}
			if e.Key == "SUB" || e.Key == "name_subscription" {
				st.ConfigKeys[keySignature(e)] = judgment{Choice: "message_subscription", Confidence: 0.95}
				st.ConfigValues[entryID(e)] = judgment{Choice: "message_subscription", Confidence: 0.9}
			}
		}
	}
	snap := platformSnapshot{Provider: "gcp", Scope: "p-prd", Status: "ok", Topics: []string{"projects/p-prd/topics/orders-cl-outbound", "projects/p-prd/topics/orders-in"},
		Subscriptions: []platformSubscription{{Name: "projects/p-prd/subscriptions/orders-cl-inbound-sub", Topic: "projects/p-prd/topics/orders-in", Attributes: filterAttributes(`attributes.eventType="orderConfirmed" AND attributes.country="CL"`)}}}
	a := &assembly{scans: scans, st: st, platform: buildPlatformIndex([]platformSnapshot{snap})}
	if len(a.pendingQuestions()) != 0 {
		t.Fatalf("unexpected questions: %+v", a.pendingQuestions())
	}
	facts := a.facts()
	var f repoFacts
	for _, x := range facts {
		if x.Repo == "SVC-orders" {
			f = x
		}
	}
	got := map[string]resource{}
	for _, r := range f.Resources {
		got[normalizeResource(r.Name)] = r
	}
	sub := got["orders-cl-inbound-sub"]
	if sub.Direction != "consume" || sub.Topic != "projects/p-prd/topics/orders-in" || len(sub.Events) != 1 || sub.Events[0] != "orderConfirmed" {
		t.Fatalf("platform wiring %+v", sub)
	}
	if _, ok := got["orders.audit-sub"]; !ok {
		t.Fatalf("IaC block naming the service not attributed: %v", got)
	}
	kinds := map[string]string{}
	for _, p := range f.Pending {
		kinds[normalizeResource(p.Subject)] = p.Kind
	}
	if kinds["ghost-topic"] != "not-in-platform" || kinds["orders.audit-sub"] == "" {
		t.Fatalf("pending %v", f.Pending)
	}
	publish := false
	for _, ev := range f.Events {
		publish = publish || ev.Name == "orderConfirmed" && ev.Role == "publish-candidate"
	}
	if !publish {
		t.Fatalf("event literal not detected: %+v", f.Events)
	}
	vault := t.TempDir()
	write(t, vault, "20-Repos/orders.md", "---\naliases: [\"SVC-orders\"]\ngatillado-por: [\"[[orderConfirmed]]\"]\npublica-en: [\"[[orders-outbound]]\", \"[[legacy-topic]]\"]\n---\n")
	write(t, vault, "25-Topics/orderConfirmed.md", "---\ntipo: evento\n---\n")
	write(t, vault, "25-Topics/orders-outbound.md", "---\ntipo: topic\n---\n")
	write(t, vault, "25-Topics/legacy-topic.md", "---\ntipo: topic\n---\n")
	f.Note = "20-Repos/orders.md"
	cmp, e := compareNotes(vault, []repoFacts{f})
	if e != nil {
		t.Fatal(e)
	}
	if len(cmp) != 1 || len(cmp[0].Supported) != 2 || len(cmp[0].Discrepancies) != 1 || cmp[0].Discrepancies[0].Target != "legacy-topic" {
		t.Fatalf("comparison %+v", cmp)
	}
}

func TestAgentAnswersAreValidated(t *testing.T) {
	st := &store{Dependencies: map[string]judgment{}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	q := newQuestion("dependency", "go:x", map[string]any{})
	pending := map[string]question{"go:x": q}
	if _, e := recordAnswers(st, pending, []map[string]any{{"id": "go:x", "choice": "invented"}}, "agent"); e == nil {
		t.Fatal("choices outside the option set must be rejected")
	}
	if _, e := recordAnswers(st, pending, []map[string]any{{"id": "go:y", "choice": "messaging"}}, "agent"); e == nil {
		t.Fatal("unknown ids must be rejected")
	}
	n, e := recordAnswers(st, pending, []map[string]any{{"id": "go:x", "choice": "messaging", "confidence": 0.7}}, "agent")
	if e != nil || n != 1 || st.Dependencies["go:x"].Source != "agent" {
		t.Fatalf("record %v %v %+v", n, e, st.Dependencies)
	}
	// A recorded judgment is corrected without being pending, within its kind's options.
	if n, e := recordAnswers(st, map[string]question{}, []map[string]any{{"id": "go:x", "choice": "document_db", "confidence": 0.9}}, "agent"); e != nil || n != 1 || st.Dependencies["go:x"].Choice != "document_db" {
		t.Fatalf("correction %v %v %+v", n, e, st.Dependencies)
	}
	if _, e := recordAnswers(st, map[string]question{}, []map[string]any{{"id": "go:x", "choice": "message_topic"}}, "agent"); e == nil {
		t.Fatal("a correction must use its own kind's options")
	}
}

func TestNameMatchAndResourceShape(t *testing.T) {
	if !nameMatch("acme-stock-adjustment-inbound", "projects/p/topics/acme-stock-adjustment-cl-inbound") {
		t.Fatal("country variant must match the logical name")
	}
	if nameMatch("orders-outbound", "inbound") || nameMatch("acme-stock-soh-inbound", "stock-inbound-topic-cl") {
		t.Fatal("unrelated names must not match")
	}
	if resourceShaped("test") || resourceShaped("a b-c") || !resourceShaped("orders.confirmed") {
		t.Fatal("resource shape rule")
	}
	if logicalName("acme-store-reception-cl-inbound") != logicalName("acme-store-reception-pe-inbound") {
		t.Fatal("variants must share a logical name")
	}
}

func TestNoteGates(t *testing.T) {
	dir := t.TempDir()
	repo := gitRepo(t, filepath.Join(dir, "repos", "SVC-orders"), map[string]string{
		"go.mod":          "module example.com/orders\n\nrequire cloud.google.com/go/pubsub v1.0.0\n",
		"pub/pub.go":      "package pub\n\nimport \"cloud.google.com/go/pubsub\"\n\nfunc Publish() {\n\ttopic := os.Getenv(\"TOPIC_OUT\")\n\t_ = pubsub.NewClient\n}\n",
		"k8s/prod/env-cl": "TOPIC_OUT=orders-cl-outbound\n",
	})
	sha, _ := resolveCommit(repo, "HEAD")
	cmd := exec.Command("git", "-C", repo, "remote", "add", "origin", "https://github.com/acme/SVC-orders.git")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(string(b))
	}
	vault := t.TempDir()
	for _, m := range []string{"AGENTS.md", "00-Home.md", "90-Meta/Convenciones.md", "90-Meta/Auditoria - Framework.md"} {
		write(t, vault, m, "x\n")
	}
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+filepath.Join(dir, "repos")+"\"\n")
	st := &store{Dependencies: map[string]judgment{"go:cloud.google.com/go/pubsub": {Choice: "messaging", Confidence: 1}}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	s := scan(t, "SVC-orders", repo)
	for _, e := range s.entries {
		st.ConfigKeys[keySignature(e)] = judgment{Choice: "message_topic", Confidence: 0.95}
		st.ConfigValues[entryID(e)] = judgment{Choice: "message_topic", Confidence: 0.95}
	}
	if e := st.save(vault); e != nil {
		t.Fatal(e)
	}
	link := "https://github.com/acme/SVC-orders/blob/" + sha + "/pub/pub.go#L5-L8"
	good := "---\naliases: [\"SVC-orders\"]\ncommit-analizado: \"" + sha[:12] + "\"\n---\n# orders\n\nPublica en el topic `orders-cl-outbound` usando `TOPIC_OUT`. [^e1]\n\n[^e1]: [pub/pub.go](" + link + ") — L5-L8: `os.Getenv(\"TOPIC_OUT\")` y `pubsub.NewClient`\n"
	write(t, vault, "20-Repos/orders.md", good)
	r, e := checkNote(vault, "20-Repos/orders.md", "", false)
	if e != nil {
		t.Fatal(e)
	}
	if !r.OK || r.Anchors["verified"] != 1 || r.Coverage["connector_categories_evidenced"] != 1 || r.Coverage["resource_groups_addressed"] != 1 {
		t.Fatalf("good note must pass: %+v", r)
	}
	write(t, vault, "25-Topics/orders-legacy.md", "---\ntipo: topic\n---\n# orders-legacy\n")
	write(t, vault, "20-Repos/orders.md", strings.Replace(good, "---\n# orders", "publica-en: [\"[[orders-legacy]]\"]\n---\n# orders", 1))
	r, _ = checkNote(vault, "20-Repos/orders.md", "", false)
	flagged := false
	for _, i := range r.Issues {
		flagged = flagged || i.Gate == "G3-relation" && i.Severity == "review" && i.Where == "orders-legacy"
	}
	if !r.OK || !flagged {
		t.Fatalf("a relation the evidence does not support goes to review without failing the gate: %+v", r.Issues)
	}
	multi := strings.Replace(good, "[pub/pub.go]("+link+")", "[go.mod](https://github.com/acme/SVC-orders/blob/"+sha+"/go.mod), [pub/pub.go]("+link+")", 1)
	write(t, vault, "20-Repos/orders.md", multi)
	if r, _ = checkNote(vault, "20-Repos/orders.md", "", false); !r.OK {
		t.Fatalf("an identifier in one of a footnote's cited files satisfies the footnote: %+v", r.Issues)
	}
	bad := strings.Replace(good, "`pubsub.NewClient`", "`kafka.NewWriter`", 1)
	bad = strings.Replace(bad, "#L5-L8", "#L5-L40", 1)
	bad = strings.Replace(bad, "Publica en el topic `orders-cl-outbound` usando `TOPIC_OUT`.", "Publica eventos.", 1)
	bad = strings.Replace(bad, "pub/pub.go](", "pub/pub.go]("+"", 1)
	write(t, vault, "20-Repos/orders.md", bad)
	r, _ = checkNote(vault, "20-Repos/orders.md", "", false)
	gates := map[string]int{}
	for _, i := range r.Issues {
		if i.Severity == "error" {
			gates[i.Gate]++
		}
	}
	if r.OK || gates["G1-anchor"] == 0 {
		t.Fatalf("invented identifier or impossible range must fail G1: %+v", r.Issues)
	}
	if ok, _, pre, _ := CheckNoteIntroduced(vault, "20-Repos/orders.md", []byte(bad)); !ok || pre == 0 {
		t.Fatal("a touched note keeping its pre-existing errors must not block")
	}
	if ok, introduced, _, _ := CheckNoteIntroduced(vault, "20-Repos/orders.md", []byte(good)); ok || len(introduced) == 0 {
		t.Fatal("errors added to a touched note must block")
	}
	if ok, _, _, _ := CheckNoteIntroduced(vault, "20-Repos/orders.md", nil); ok {
		t.Fatal("the synced note must pass every gate")
	}
	if claim := claimFor("Hace A. Luego publica B. [^e2] [^e3]\n", "e3"); claim != "Luego publica B." {
		t.Fatalf("claim %q", claim)
	}
	if !tokenPresent("health.port", "health:\n  port: 8086\n") || tokenPresent("health.host", "health:\n  port: 1\n") {
		t.Fatal("dotted key paths follow YAML nesting")
	}
}

func TestFailedRefreshKeepsPreviousSnapshot(t *testing.T) {
	v := t.TempDir()
	good := platformSnapshot{Provider: "gcp", Scope: "p-prd", Status: "ok", Topics: []string{"projects/p-prd/topics/t"}}
	if e := saveSnapshot(v, good); e != nil {
		t.Fatal(e)
	}
	if e := saveSnapshot(v, platformSnapshot{Provider: "gcp", Scope: "p-prd", Status: "auth-required", Detail: "reauthentication failed", CapturedAt: "later"}); e != nil {
		t.Fatal(e)
	}
	snaps, _ := loadSnapshots(v)
	if len(snaps) != 1 || snaps[0].Status != "ok" || len(snaps[0].Topics) != 1 || snaps[0].RefreshFailed["status"] != "auth-required" {
		t.Fatalf("previous evidence lost: %+v", snaps)
	}
}

func TestReferenceRefFollowsPolicy(t *testing.T) {
	g := func(root string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %s", args, b)
		}
	}
	remote := gitRepo(t, filepath.Join(t.TempDir(), "remote"), map[string]string{"a.txt": "1\n"})
	g(remote, "checkout", "-q", "-b", "trunk")
	clone := filepath.Join(t.TempDir(), "clone")
	if b, e := exec.Command("git", "clone", "-q", remote, clone).CombinedOutput(); e != nil {
		t.Fatalf("clone: %s", b)
	}
	// The remote moves ahead after the clone's local main was checked out.
	g(remote, "checkout", "-q", "main")
	write(t, remote, "a.txt", "2\n")
	g(remote, "commit", "-qam", "ahead")
	g(clone, "checkout", "-q", "main")
	g(clone, "fetch", "-q", "origin")
	g(clone, "remote", "set-head", "origin", "--delete")
	if ref, note, e := referenceRef(clone, "", nil); e != nil || note != "" || ref != "refs/remotes/origin/main" {
		t.Fatalf("unlisted repo must use origin/main over a lagging local branch: %q %q %v", ref, note, e)
	}
	if ref, _, e := referenceRef(clone, "trunk", nil); e != nil || ref != "refs/remotes/origin/trunk" {
		t.Fatalf("configured branch must be exact: %q %v", ref, e)
	}
	if _, _, e := referenceRef(clone, "release/x", nil); e == nil {
		t.Fatal("a missing configured branch must block, not fall back")
	}
	g(clone, "branch", "-q", "develop")
	if ref, _, e := referenceRef(clone, "", []string{"develop", "main"}); e != nil || ref != "refs/heads/develop" {
		t.Fatalf("the cell's branch order is tried first to last: %q %v", ref, e)
	}
	if ref, _, e := referenceRef(clone, "", []string{"release", "master", "main"}); e != nil || ref != "refs/remotes/origin/main" {
		t.Fatalf("absent branches of the order are skipped: %q %v", ref, e)
	}
	other := gitRepo(t, filepath.Join(t.TempDir(), "other"), map[string]string{"a.txt": "1\n"})
	g(other, "branch", "-m", "main", "develop")
	if ref, note, e := referenceRef(other, "", nil); e != nil || ref != "HEAD" || note == "" {
		t.Fatalf("no main/master must scan HEAD and report the fallback: %q %q %v", ref, note, e)
	}
}

func TestLibraryContextResolvesSelectedRepository(t *testing.T) {
	dir := t.TempDir()
	repos := filepath.Join(dir, "repos")
	gitRepo(t, filepath.Join(repos, "SVC-common"), map[string]string{
		"go.mod":           "module example.org/platform/common\n",
		"publisher/pub.go": "package publisher\nimport (\n\t\"cloud.google.com/go/pubsub\"\n\t\"example.org/platform/common/store\"\n)\n",
		"store/store.go":   "package store\nimport \"cloud.google.com/go/firestore\"\n",
	})
	svc := gitRepo(t, filepath.Join(repos, "SVC-orders"), map[string]string{
		"go.mod":      "module example.org/orders\n\nrequire example.org/platform/common v1.0.0\n",
		"cmd/main.go": "package main\nimport \"example.org/platform/common/publisher\"\n",
	})
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	s := scan(t, "SVC-orders", svc)
	libs := libraryContext(vault, []*repoScan{s})
	if len(libs) != 1 || libs[0].in.Name != "SVC-common" {
		t.Fatalf("the imported company library must be scanned: %v", libs)
	}
	resolveLibraries(append([]*repoScan{s}, libs...))
	if d := s.deps["go:cloud.google.com/go/pubsub"]; d == nil || d.Via != "SVC-common" {
		t.Fatalf("a --repo run must resolve library dependencies like a full run: %+v", s.deps)
	}
	if d := s.deps["go:cloud.google.com/go/firestore"]; d == nil || d.Via != "SVC-common" {
		t.Fatalf("a library's own imported packages must be followed: %+v", s.deps)
	}
}

func TestStaleNeighboursAfterSync(t *testing.T) {
	dir := t.TempDir()
	repos := filepath.Join(dir, "repos")
	repo := gitRepo(t, filepath.Join(repos, "SVC-orders"), map[string]string{"svc/save.go": "package svc\n// save then notify\n", "svc/other.go": "package svc\n"})
	old, _ := resolveCommit(repo, "HEAD")
	write(t, repo, "svc/save.go", "package svc\n// exists, enrich, save, notify\n")
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "change"}} {
		if b, e := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); e != nil {
			t.Fatal(string(b))
		}
	}
	head, _ := resolveCommit(repo, "HEAD")
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\nsources:\n  repo_prefixes: [\"SVC\"]\n")
	write(t, vault, ".knowledge-os-config.yaml", "version: 1\nworkspace:\n  repository_roots:\n    - \""+repos+"\"\n")
	link := func(sha, file string) string {
		return "https://github.com/acme/SVC-orders/blob/" + sha + "/" + file + "#L2"
	}
	write(t, vault, "20-Repos/orders.md", "---\naliases: [\"SVC-orders\"]\ncommit-analizado: \""+head[:12]+"\"\n---\n# orders\n")
	write(t, vault, "25-Topics/orders-out.md", "# orders-out\n\nGuarda y notifica. [^e1]\nOtro. [^e2]\n\n[^e1]: [svc/save.go]("+link(old, "svc/save.go")+")\n[^e2]: [svc/other.go]("+link(old, "svc/other.go")+")\n")
	write(t, vault, "30-Flujos/flow.md", "# flow\n\nYa actualizado. [^e1]\n\n[^e1]: [svc/save.go]("+link(head, "svc/save.go")+")\n")
	if !overlaps([][2]int{{22, 21}}, 12, 30) || overlaps([][2]int{{22, 21}}, 1, 21) || !overlaps([][2]int{{5, 7}}, 7, 9) || overlaps([][2]int{{5, 7}}, 8, 9) {
		t.Fatal("line overlap: an insertion counts only inside the cited block")
	}
	write(t, vault, "30-Flujos/package.md", "# package\n\nPaquete. [^e1]\n\n[^e1]: [svc/save.go]("+strings.Replace(link(old, "svc/save.go"), "#L2", "#L1", 1)+")\n")
	// A neighbour citing a commit newer than the synced note (a pinned later version) is not stale.
	write(t, vault, "20-Repos/orders-old.md", "---\naliases: [\"SVC-orders\"]\ncommit-analizado: \""+old[:12]+"\"\n---\n# orders\n")
	if s, _ := StaleNeighbours(vault, []string{"20-Repos/orders-old.md"}); len(s) != 0 {
		t.Fatalf("newer anchors are not stale: %v", s)
	}
	_ = os.Remove(filepath.Join(vault, "20-Repos/orders-old.md"))
	stale, e := StaleNeighbours(vault, []string{"20-Repos/orders.md"})
	if e != nil {
		t.Fatal(e)
	}
	if len(stale) != 1 || !strings.HasPrefix(stale[0], "25-Topics/orders-out.md") || !strings.Contains(stale[0], "svc/save.go") || strings.Contains(stale[0], "other.go") {
		t.Fatalf("only the neighbour citing a changed file at an older commit is stale: %v", stale)
	}
}

func TestClaimsAndCorrections(t *testing.T) {
	vault := t.TempDir()
	facts := repoFacts{Repo: "SVC-orders", Resources: []resource{{Type: "message_subscription", Name: "projects/p-prd/subscriptions/orders-cl-inbound-sub", Topic: "projects/p-prd/topics/orders-inbound"}}}
	if e := writeState(vault, "facts/SVC-orders.json", facts); e != nil {
		t.Fatal(e)
	}
	cmp := []comparison{{Repo: "SVC-orders", Note: "20-Repos/orders.md", Discrepancies: []discrepancy{{Field: "gatillado-por", Target: "legacy-orders-topic", Found: []string{"orders-inbound"}}}}}
	if e := writeState(vault, "comparison.json", cmp); e != nil {
		t.Fatal(e)
	}
	write(t, vault, "25-Topics/stock-topic.md", "---\ntipo: topic\nnombre-raw: \"stock-inbound-{cl|pe}\"\n---\n# stock\n")
	write(t, vault, "90-Meta/discovery/platform/gcp-p-prd.json", `{"provider":"gcp","scope":"p-prd","status":"ok","topics":["projects/p-prd/topics/audit-events"],"subscriptions":[]}`)
	draft := "El servicio consume `orders-cl-inbound-sub` del topic `orders-inbound`, y audita en `audit-events` y `stock-inbound-pe`.\n\n" +
		"Según [[orders]], también lo dispara `legacy-orders-topic`.\n\nPublica además en `invented-orders-topic`; ver `cmd/main.go` y `pubsub.NewClient` con `GCP_PROJECT_ID`.\n" +
		"Los reembolsos llegan por `refund-orders-inbound` y `refund-orders-cl-inbound-sub`; el topic de producción es `orders-inbound-prd`.\n"
	r, e := checkClaims(vault, draft)
	if e != nil {
		t.Fatal(e)
	}
	unknown := fmt.Sprint(r["unknown_names"])
	for _, invented := range []string{"refund-orders-inbound", "refund-orders-cl-inbound-sub"} {
		if !strings.Contains(unknown, invented) {
			t.Fatalf("a known name plus a meaningful token is not known: %s missing from %v", invented, r)
		}
	}
	if strings.Contains(unknown, "orders-inbound-prd") {
		t.Fatalf("an environment variant of a known name is known: %v", r)
	}
	if r["ok"] != false || !strings.Contains(unknown, "invented-orders-topic") || strings.Contains(unknown, "stock-inbound-pe") || strings.Contains(unknown, "main.go") || strings.Contains(unknown, "GCP_PROJECT_ID") {
		t.Fatalf("only names no evidence knows are flagged: %v", r)
	}
	if !strings.Contains(fmt.Sprint(r["contradicted_relations"]), "legacy-orders-topic") {
		t.Fatalf("a relation discovery contradicts must be flagged: %v", r)
	}
	var b bytes.Buffer
	if e := listCorrections(options{vault: vault}, &b); e != nil || !strings.Contains(b.String(), `"target": "legacy-orders-topic"`) {
		t.Fatalf("corrections %v %s", e, b.String())
	}
}

func TestShortNamesAreCheckedInTheScopeTheirFileDeclares(t *testing.T) {
	dir := t.TempDir()
	svc := gitRepo(t, filepath.Join(dir, "SVC-stock"), map[string]string{
		"go.mod":         "module example.com/stock\n",
		"k8s/prd/env-cl": "PROJECT_ID=x-app-prd\nSUB=stock-cl-sub\nTOPIC=catalog-topic\n",
		"k8s/uat/env-cl": "PROJECT_ID=x-app-uat\nSUB=stock-cl-sub\nTOPIC=catalog-topic\n",
	})
	st := &store{Dependencies: map[string]judgment{}, ConfigKeys: map[string]judgment{}, ConfigValues: map[string]judgment{}}
	s := scan(t, "SVC-stock", svc)
	for _, e := range s.entries {
		choice := map[string]string{"PROJECT_ID": "cloud_project_or_region", "SUB": "message_subscription", "TOPIC": "message_topic"}[e.Key]
		st.ConfigKeys[keySignature(e)] = judgment{Choice: choice, Confidence: 0.95}
		st.ConfigValues[entryID(e)] = judgment{Choice: choice, Confidence: 0.95}
	}
	snaps := []platformSnapshot{
		{Provider: "gcp", Scope: "x-app-prd", Status: "ok", Subscriptions: []platformSubscription{{Name: "projects/x-app-prd/subscriptions/stock-cl-sub", Topic: "projects/other-sys-prd/topics/catalog-topic"}}},
		{Provider: "gcp", Scope: "x-app-uat", Status: "ok"},
		{Provider: "gcp", Scope: "other-sys-prd", Status: "ok", Topics: []string{"projects/other-sys-prd/topics/catalog-topic"}},
	}
	a := &assembly{scans: []*repoScan{s}, st: st, platform: buildPlatformIndex(snaps)}
	f := a.facts()[0]
	subjects := []string{}
	for _, p := range f.Pending {
		subjects = append(subjects, p.Subject)
	}
	joined := strings.Join(subjects, " | ")
	if !strings.Contains(joined, "stock-cl-sub @ gcp:x-app-uat") {
		t.Fatalf("a subscription present in prd but absent in the uat project its file declares is pending: %v", subjects)
	}
	if strings.Contains(joined, "catalog-topic @") {
		t.Fatalf("a topic owned by another system's project is not missing from the consumer's project: %v", subjects)
	}
}

func TestNoteNamesExpandBraceAlternatives(t *testing.T) {
	n := vaultNote{fm: map[string]string{"nombre-raw": `"inventory-inbound-topic-{cl|co|pe}"`, "aliases": `["stock-{a|b}"]`}}
	names := n.names("acme-scan-inventory-soh-inbound")
	hit := false
	for _, d := range names {
		hit = hit || nameMatch(d, "inventory-inbound-topic-cl")
	}
	if short := (vaultNote{fm: map[string]string{"nombre-raw": `"svc-{|v2}"`}}).names("svc-note"); len(short) != 2 || short[1] != "svc-v2" {
		t.Fatalf("a too-short variant is dropped: %v", short)
	}
	if !hit || len(names) != 6 {
		t.Fatalf("a per-country resource matches the braced raw name: %v", names)
	}
}

func TestPartialRunKeepsOtherRepositoriesFacts(t *testing.T) {
	v := t.TempDir()
	for _, r := range []string{"repo-a", "repo-b"} {
		if e := writeState(v, filepath.Join("facts", r+".json"), repoFacts{Repo: r, Note: "20-Repos/" + r + ".md"}); e != nil {
			t.Fatal(e)
		}
	}
	fresh := []repoFacts{{Repo: "repo-a", Note: "20-Repos/repo-a.md", Commit: "new"}}
	all := storedFacts(v, fresh, map[string]bool{"repo-a": true, "repo-b": true})
	if len(all) != 2 || all[0].Commit != "new" || all[1].Repo != "repo-b" {
		t.Fatalf("a partial run compares the scanned repository fresh and keeps the others: %+v", all)
	}
	if all := storedFacts(v, fresh, map[string]bool{"repo-a": true}); len(all) != 1 {
		t.Fatalf("a repository no longer tracked is left out: %+v", all)
	}
}

func TestLibraryDependenciesListEveryImportingFile(t *testing.T) {
	dir := t.TempDir()
	lib := gitRepo(t, filepath.Join(dir, "LIB-common"), map[string]string{
		"go.mod":           "module example.com/common\n",
		"publisher/pub.go": "package publisher\nimport \"cloud.google.com/go/pubsub\"\n",
	})
	svc := gitRepo(t, filepath.Join(dir, "SVC-orders"), map[string]string{
		"go.mod":        "module example.com/orders\n\nrequire example.com/common v0.1.0\n",
		"cmd/main.go":   "package main\nimport \"example.com/common/publisher\"\n",
		"cmd/worker.go": "package main\nimport \"example.com/common/publisher\"\n",
		"cmd/other.go":  "package main\nimport \"example.com/common/publisher\"\n",
	})
	for i := 0; i < 20; i++ { // map iteration order changes between runs
		s := scan(t, "SVC-orders", svc)
		resolveLibraries([]*repoScan{s, scan(t, "LIB-common", lib)})
		d := s.deps["go:cloud.google.com/go/pubsub"]
		if d == nil || strings.Join(d.Files, ",") != "cmd/main.go,cmd/other.go,cmd/worker.go" {
			t.Fatalf("a library dependency is evidenced by every file that imports the library: %+v", d)
		}
	}
}
