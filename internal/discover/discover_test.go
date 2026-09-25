package discover

import (
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
				st.ConfigKeys[keySignature(e)] = judgment{Choice: "pubsub_subscription", Confidence: 0.95}
				st.ConfigValues[entryID(e)] = judgment{Choice: "pubsub_subscription", Confidence: 0.9}
			}
		}
	}
	snap := platformSnapshot{Project: "p-prd", Status: "ok", Topics: []string{"projects/p-prd/topics/orders-cl-outbound", "projects/p-prd/topics/orders-in"},
		Subscriptions: []pubsubSubscription{{Name: "projects/p-prd/subscriptions/orders-cl-inbound-sub", Topic: "projects/p-prd/topics/orders-in", Attributes: filterAttributes(`attributes.eventType="orderConfirmed" AND attributes.country="CL"`)}}}
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
		st.ConfigKeys[keySignature(e)] = judgment{Choice: "pubsub_topic", Confidence: 0.95}
		st.ConfigValues[entryID(e)] = judgment{Choice: "pubsub_topic", Confidence: 0.95}
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
	if claim := claimFor("Hace A. Luego publica B. [^e2] [^e3]\n", "e3"); claim != "Luego publica B." {
		t.Fatalf("claim %q", claim)
	}
	if !tokenPresent("health.port", "health:\n  port: 8086\n") || tokenPresent("health.host", "health:\n  port: 1\n") {
		t.Fatal("dotted key paths follow YAML nesting")
	}
}

func TestFailedRefreshKeepsPreviousSnapshot(t *testing.T) {
	v := t.TempDir()
	good := platformSnapshot{Provider: "gcp-pubsub", Project: "p-prd", Status: "ok", Topics: []string{"projects/p-prd/topics/t"}}
	if e := saveSnapshot(v, good); e != nil {
		t.Fatal(e)
	}
	if e := saveSnapshot(v, platformSnapshot{Project: "p-prd", Status: "auth-required", Detail: "reauthentication failed", CapturedAt: "later"}); e != nil {
		t.Fatal(e)
	}
	snaps, _ := loadSnapshots(v)
	if len(snaps) != 1 || snaps[0].Status != "ok" || len(snaps[0].Topics) != 1 || snaps[0].RefreshFailed["status"] != "auth-required" {
		t.Fatalf("previous evidence lost: %+v", snaps)
	}
}
