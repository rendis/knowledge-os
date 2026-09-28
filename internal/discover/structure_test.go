package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeNote(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if e := os.MkdirAll(filepath.Dir(p), 0o755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0o644); e != nil {
		t.Fatal(e)
	}
	return p
}

func TestEmptyCoreSections(t *testing.T) {
	cited := strings.Repeat("The handler acknowledges a duplicate and stops. [^e1]\n\n", 6)
	misplaced := "## Qué hace\n\nNo observado en fuentes estáticas revisadas.\n\n## Infraestructura y scheduling\n\n" + cited + "## Limitaciones y desconocimientos\n\nNone.\n\n[^e1]: [a.go](https://github.com/o/r/blob/abcdef1/a.go#L1) — L1\n"
	is := emptyCoreSections(misplaced)
	if len(is) != 1 || is[0].Severity != "error" || is[0].Where != "Qué hace" || !strings.Contains(is[0].Detail, "6 cited claims sit outside") || !strings.Contains(is[0].Detail, "Infraestructura y scheduling") {
		t.Fatalf("an empty core section beside cited claims is reported: %+v", is)
	}
	placed := "## Qué hace\n\n" + cited + "## Gatillo\n\nNo observado: it is a library.\n"
	if is := emptyCoreSections(placed); len(is) != 0 {
		t.Fatalf("an absence is fine when the core sections carry the claims: %+v", is)
	}
	if is := emptyCoreSections("## Qué hace\n\nNo observado.\n\n## Infraestructura y scheduling\n\nOne claim. [^e1]\n"); len(is) != 0 {
		t.Fatalf("a nearly empty note is not misplaced: %+v", is)
	}
}

func TestUndeclaredRelations(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "25-Topics/orders-created.md", "---\ntipo: topic\nnombre-raw: \"orders-created\"\n---\n")
	writeNote(t, vault, "25-Topics/orders-created-deadletter.md", "---\ntipo: topic\nnombre-raw: \"orders-created-deadletter\"\n---\n")
	f := repoFacts{Languages: map[string]int{"go": 3}, Resources: []resource{
		{Type: "message_subscription", Name: "orders-created-cl-sub", Direction: "consume", Topic: "orders-created-cl", Evidence: []evidence{{Kind: "config", File: "k8s/configmap"}}},
		{Type: "message_topic", Name: "only-in-platform", Direction: "publish", Evidence: []evidence{{Kind: "platform"}}},
	}}
	got := undeclaredRelations(vault, map[string]string{"gatillado-por": "[]"}, "## Relaciones\n\nNone.\n", f)
	if len(got) != 1 || got[0].Where != "[[orders-created]]" || !strings.Contains(got[0].Detail, "`gatillado-por`") {
		t.Fatalf("the consumed topic needs gatillado-por, and its -deadletter variant is not matched: %+v", got)
	}
	if got := undeclaredRelations(vault, map[string]string{"gatillado-por": `["[[orders-created]]"]`}, "", f); len(got) != 0 {
		t.Fatalf("a declared relation is not reported: %+v", got)
	}
	limits := "## Limitaciones y desconocimientos\n\nThe subscription to [[orders-created]] is configured but its handler is disabled.\n"
	if got := undeclaredRelations(vault, map[string]string{}, limits, f); len(got) != 0 {
		t.Fatalf("an explained non-relation is not reported: %+v", got)
	}
	f.Languages = nil
	if got := undeclaredRelations(vault, map[string]string{}, "", f); len(got) != 0 {
		t.Fatalf("infrastructure repositories are judged cell-wide: %+v", got)
	}
}

func TestDuplicateParagraphs(t *testing.T) {
	vault := t.TempDir()
	fact := "The handler removes leading zeros from store, terminal and folio before it composes the identifier, and falls back to the event id when the store or the terminal is missing."
	note := writeNote(t, vault, "20-Repos/demo/orders-command.md", "---\ncommit-analizado: \"abcdef123456\"\n---\n\n## Qué hace\n\n"+fact+" [^e1]\n")
	writeNote(t, vault, "25-Topics/orders-created.md", "---\ntipo: topic\n---\n\n## Contrato\n\n- "+fact+" [^e3]\n")
	writeNote(t, vault, "20-Repos/demo/orders-query.md", "---\n---\n\n"+fact+"\n")
	writeNote(t, vault, "investigations/case/investigation.md", fact+"\n")
	b, _ := os.ReadFile(note)
	got := duplicateParagraphs(vault, note, b)
	if len(got) != 1 || got[0].Where != "[[orders-created]]" || got[0].Severity != "error" {
		t.Fatalf("only the topic copy is reported; repository notes and cases are not compared: %+v", got)
	}
}

func TestCopiesIntroduced(t *testing.T) {
	vault := t.TempDir()
	fact := "The handler removes leading zeros from store, terminal and folio before it composes the identifier, and falls back to the event id when the store or the terminal is missing."
	old := "Payments are retried three times with a fixed delay of five seconds before the message is acknowledged and moved to the error collection for manual review."
	writeNote(t, vault, "20-Repos/demo/orders-command.md", "---\n---\n\n"+fact+" [^e1]\n\n"+old+"\n")
	topic := "---\ntipo: topic\n---\n\n## Contrato\n\n" + old + "\n\n- " + fact + " [^e3]\n"
	writeNote(t, vault, "25-Topics/orders-created.md", topic)
	got, e := CopiesIntroduced(vault, "25-Topics/orders-created.md", []byte("---\n---\n\n"+old+"\n"))
	if e != nil || len(got) != 1 || !strings.Contains(got[0], "[[orders-command]]") || !strings.Contains(got[0], "the handler removes") {
		t.Fatalf("only the copy the branch added is reported: %v %v", got, e)
	}
	if got, _ := CopiesIntroduced(vault, "25-Topics/orders-created.md", nil); len(got) != 2 {
		t.Fatalf("a new note reports every copy: %v", got)
	}
}

func TestRelativeAnchors(t *testing.T) {
	note := "Text. [^e1] [^e2] [^e3]\n\n[^e1]: src/publisher.go#L27-L48, cmd/main.go#L9 — `publishWithRetry`\n" +
		"[^e2]: [a.go](https://github.com/o/r/blob/abcdef1/a.go#L1) — long form\n[^e3]: see the README — prose, not a path\n"
	got := relativeAnchors(note, "o/r", "abcdef123456")
	if len(got) != 2 || got[0].Path != "src/publisher.go" || got[0].From != 27 || got[0].To != 48 || got[1].Path != "cmd/main.go" || got[1].From != 9 || got[1].To != 9 {
		t.Fatalf("short citations resolve against the note's repository and commit: %+v", got)
	}
	if got[0].Repo != "o/r" || got[0].Commit != "abcdef123456" || got[0].Text != "`publishWithRetry`" || got[0].Footnote != "e1" {
		t.Fatalf("anchor fields: %+v", got[0])
	}
	if got := relativeAnchors("[^e1]: Dockerfile — `FROM`\n[^e2]: Dockerfile#L1-L9 — x\n[^e3]: README — prose\n", "o/r", "abc"); len(got) != 2 {
		t.Fatalf("conventional extensionless files are paths, a bare word is not: %+v", got)
	}
	if got := relativeAnchors("[^e1]: builder.go — SetAuditOpt; [a.go](https://github.com/o/r/blob/abc1234/a.go) — Audit\n", "o/r", "abc"); len(got) != 1 || got[0].Path != "builder.go" {
		t.Fatalf("a short citation followed by a permalink in the text is still read: %+v", got)
	}
	if got := relativeAnchors(note, "", "abc"); len(got) != 0 {
		t.Fatal("without the note's repository there is no short form")
	}
}

func TestShortenCitationsKeepsMixedFootnotes(t *testing.T) {
	own := "https://github.com/acme/SVC-orders/blob/abcdef1234567890/pub/pub.go#L5-L8"
	other := "https://github.com/acme/SVC-lib/blob/1111111111111111/lib.go#L3"
	note := "---\naliases: [\"SVC-orders\"]\ncommit-analizado: \"abcdef123456\"\n---\n\nText. [^e1] [^e2] [^e3]\n\n" +
		"[^e1]: [pub/pub.go](" + own + "), [cmd/main.go](https://github.com/acme/SVC-orders/blob/abcdef1234567890/cmd/main.go) — `Publish`\n" +
		"[^e2]: [pub/pub.go](" + own + "), [lib.go](" + other + ") — mixed\n" +
		"[^e3]: [pub/pub.go](https://github.com/acme/SVC-orders/blob/9999999999999999/pub/pub.go#L1) — older commit\n"
	fm := frontmatterOf([]byte(note))
	got, n := shortenCitations(note, "/v/20-Repos/orders.md", fm)
	if n != 1 || !strings.Contains(got, "[^e1]: pub/pub.go#L5-L8, cmd/main.go — `Publish`") || !strings.Contains(got, "[^e2]: [pub/pub.go]("+own+")") || !strings.Contains(got, "9999999999999999") {
		t.Fatalf("only footnotes citing the own repository at commit-analizado become short: %d\n%s", n, got)
	}
}

func TestUndeclaredCalls(t *testing.T) {
	vault := t.TempDir()
	writeNote(t, vault, "20-Repos/demo/product-query.md", "---\naliases: [\"APP01-product-query\"]\n---\n")
	writeNote(t, vault, "40-Integraciones/Provider.md", "---\ntipo: integracion-externa\naliases: [\"sftp.provider.example\"]\n---\n")
	f := repoFacts{Languages: map[string]int{"go": 3}, Resources: []resource{
		{Type: "http_endpoint", Name: "http://product-query-service/api/v1/categories", Evidence: []evidence{{Kind: "config"}}},
		{Type: "http_endpoint", Name: "product-query-svc.ns.svc.cluster.local:50051", Evidence: []evidence{{Kind: "config"}}},
		{Type: "storage_bucket", Name: "sftp://sftp.provider.example/inbox", Evidence: []evidence{{Kind: "config"}}},
		{Type: "storage_bucket", Name: "product-query", Evidence: []evidence{{Kind: "config"}}},
		{Type: "http_endpoint", Name: "https://api.unknown.example/v1", Evidence: []evidence{{Kind: "config"}}},
		{Type: "http_endpoint", Name: "http://orders-command-service/x", Evidence: []evidence{{Kind: "config"}}},
	}}
	fm := map[string]string{"aliases": `["APP01-orders-command"]`}
	got := undeclaredCalls(vault, "/v/20-Repos/demo/orders-command.md", fm, "", f)
	if len(got) != 2 || got[0].Where != "[[Provider]]" || !strings.Contains(got[0].Detail, "`lee-de` or `escribe-en`") ||
		got[1].Where != "[[product-query]]" || !strings.Contains(got[1].Detail, "`consume-de`") || !strings.Contains(got[1].Detail, "product-query-svc.ns.svc.cluster.local") {
		t.Fatalf("HTTP and gRPC hosts resolve to the repository note, an SFTP host to its integration; a bucket name, an unknown host and the note itself do not count: %+v", got)
	}
	if got := undeclaredCalls(vault, "/v/20-Repos/demo/orders-command.md", map[string]string{"consume-de": `["[[product-query]]"]`, "lee-de": `["[[Provider]]"]`}, "", f); len(got) != 0 {
		t.Fatalf("declared calls are not reported: %+v", got)
	}
}
