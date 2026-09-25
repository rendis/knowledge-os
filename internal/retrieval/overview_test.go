package retrieval

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestOverviewOneLinePerNote(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if e := os.WriteFile(p, []byte(s), 0o644); e != nil {
			t.Fatal(e)
		}
	}
	write("20-Repos/orders.md", "---\ntipo: api\nsistema: \"[[Sales]]\"\npublica-en: [\"[[orders-out]]\"]\n---\n# orders\n\n## Propósito\n\nPublica órdenes confirmadas hacia [[orders-out]] con reintentos. Segunda frase.\n")
	write("30-Flujos/long.md", "---\ntipo: flujo\n---\n# long\n\n"+strings.Repeat("a", 219)+"ó sin punto final y con más texto\n")
	write("25-Topics/orders-out.md", "---\ntipo: topic\n---\n# orders-out\n\nTopic de órdenes. [^e1]\n")
	write(".investigations/x/investigation.md", "hidden\n")
	write("AGENTS.md", "# Router\n\nInstructions, not knowledge.\n")
	write("90-Meta/Convenciones.md", "meta\n")
	write(".agents/state/discovery/comparison.json", `[{"note":"20-Repos/orders.md","discrepancies":[{"field":"publica-en","target":"orders-legacy","discovered":["orders-out"]}]}]`)
	var b bytes.Buffer
	if e := WriteOverview(root, "", &b); e != nil {
		t.Fatal(e)
	}
	out := b.String()
	for _, want := range []string{"[[orders]] (api) — Publica órdenes confirmadas hacia orders-out con reintentos.", "publica-en: orders-out", "[[orders-out]] (topic) — Topic de órdenes."} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "[[AGENTS]]") {
		t.Fatal("root instruction files are not knowledge notes")
	}
	if strings.Contains(out, "Beyond the vault") {
		t.Fatal("without reachable sources the overview adds no source section")
	}
	write(".agents/state/discovery/facts/orders.json", "{}")
	write("instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\ntrackers:\n  - id: \"acme-jira\"\n    provider: \"jira\"\n    url: \"https://acme.example/jira\"\ncapabilities:\n  database-inspection: [\"DB access\"]\n")
	b.Reset()
	_ = WriteOverview(root, "", &b)
	for _, want := range []string{"## Beyond the vault", "Repositories: 1 with discovery facts", "Trackers: acme-jira jira", "Capabilities: database-inspection"} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("the overview lists the sources reachable beyond the notes: missing %q in\n%s", want, b.String())
		}
	}
	if !strings.Contains(out, "⚠ unsupported by the last discover run, verify before use: publica-en: orders-legacy (repository evidence names orders-out)") {
		t.Fatalf("a discovery discrepancy must be flagged on its note:\n%s", out)
	}
	if !utf8.ValidString(out) {
		t.Fatal("a truncated summary must stay valid UTF-8")
	}
	if strings.Contains(out, "hidden") || strings.Contains(out, "Convenciones") {
		t.Fatalf("overview must list only knowledge notes:\n%s", out)
	}
}
