package retrieval

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func askPack(t *testing.T, opt Options, query string) string {
	t.Helper()
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	var b bytes.Buffer
	if e := idx.Ask(ctx, query, AskOptions{Visibility: "all", Budget: DefaultBudget}, &b); e != nil {
		t.Fatal(e)
	}
	return b.String()
}

func TestContainsNameNeedsWholeName(t *testing.T) {
	for _, c := range []struct {
		text, name string
		want       bool
	}{
		{"qué publica orders?", "orders", true},
		{"qué publica orders.", "orders", true},
		{"qué publica orders-out", "orders", false},
		{"reorders", "orders", false},
	} {
		if got := containsName(c.text, c.name); got != c.want {
			t.Fatalf("containsName(%q, %q) = %v", c.text, c.name, got)
		}
	}
}

func TestCompactPermalinks(t *testing.T) {
	def := "[a.go](https://github.com/acme/APP1-x/blob/0123456789abcdef/a.go#L3) and [b.go](https://github.com/acme/APP2-y/blob/fedcba9876543210/b.go)"
	got := compactPermalinks(def, "APP1-x", "0123456789ab")
	if got != "a.go#L3 and APP2-y@fedcba987654:b.go" {
		t.Fatal(got)
	}
}

func TestAskTermsKeepIdentifiersAndPhrases(t *testing.T) {
	for w, want := range map[string]bool{"Deduplica": false, "deduplica": false, "BUSINESS_ID": true, "PMM": true, "gRPC": true, "S3": true, "Él": false} {
		if identifier(w) != want {
			t.Errorf("identifier(%q) = %v", w, !want)
		}
	}
	got := map[string]string{}
	for _, x := range askTerms(`Deduplica "registro de duplicados" transaction-acked el PMM`) {
		got[x.stem] = x.expr()
	}
	for stem, expr := range map[string]string{"dedupl": `"dedupl"*`, "registro de duplicados": `"registro de duplicados"`, "transaction-acked": `"transaction acked"`, "pmm": `"pmm"`} {
		if got[stem] != expr {
			t.Errorf("term %q = %q, want %q (all: %v)", stem, got[stem], expr, got)
		}
	}
	if _, ok := got["el"]; ok {
		t.Error("short words are not terms")
	}
}

func TestReadSectionWithItsSources(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/orders.md", "---\naliases: [\"APP1-orders\"]\n---\n# orders\n\n## Propósito\n\nPublica. [^e1]\n\n## Persistencia y datos\n\nGuarda en Firestore. [^e2]\n\n### Detalle\n\nPor business ID.\n\n## Límites\n\nNada.\n\n[^e1]: [a.go](https://github.com/acme/APP1-orders/blob/0123456789abcdef0123/a.go#L1)\n[^e2]: [b.go](https://github.com/acme/APP1-orders/blob/0123456789abcdef0123/b.go) — Save, líneas 3-4\n")
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	read := func(name string, o ReadOptions) (string, error) {
		var b bytes.Buffer
		o.Budget = DefaultBudget
		e := idx.Read(ctx, name, o, &b)
		return b.String(), e
	}
	out, e := read("APP1-orders", ReadOptions{Section: "persistencia"})
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"## L10-L17", "Guarda en Firestore.", "### Persistencia y datos > Detalle", "L16 Por business ID.", "[^e2] ? APP1-orders is not a tracked repository", "APP1-orders@0123456789ab:b.go — Save, líneas 3-4"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if text := out[strings.Index(out, "## L10"):]; strings.Contains(text, "Publica.") || strings.Contains(text, "Nada.") {
		t.Fatalf("a section stops at the next heading of its level:\n%s", out)
	}
	if out, _ = read("orders", ReadOptions{Lines: "8-8"}); !strings.Contains(out, "## L8-L8") || !strings.Contains(out, "[^e1]") {
		t.Fatalf("lines:\n%s", out)
	}
	if _, e = read("order", ReadOptions{}); e == nil || !strings.Contains(e.Error(), "did you mean: orders") {
		t.Fatalf("a near name is suggested: %v", e)
	}
	if _, e = read("orders", ReadOptions{Section: "zz"}); e == nil || !strings.Contains(e.Error(), "sections: orders · Propósito") {
		t.Fatalf("a missing section lists the sections: %v", e)
	}
}

func TestParagraphsAreWholeLinedAndFolded(t *testing.T) {
	raw := []byte("---\ntipo: api\n---\n# n\n\n## Qué hace\n\n- Uno deduplica por business ID.\n  sigue el uno\n- En development/accl se configura Firestore en acme-dev, base acme y registro de duplicados orders-transaction.\n- En development/acco se configura Firestore en acme-dev, base acme y registro de duplicados orders-transaction.\n- En development/acpe se configura Firestore en acme-dev, base acme y registro de duplicados orders-transaction.\n<!-- connection:x -->\nProsa sin coincidencia.\n\n## Datos\n\n| a | b |\n|---|---|\n| uno | nada |\n| dos | duplicados |\n| tres | nada |\n| cuatro | nada |\n\n[^e1]: [a](https://github.com/o/r/blob/abc1234/a.go) duplicados\n")
	ps := parseParagraphs(raw)
	if ps[0].line != 8 || ps[0].text != "- Uno deduplica por business ID.\n  sigue el uno" || ps[0].section != "Qué hace" {
		t.Fatalf("first paragraph %+v", ps[0])
	}
	for _, p := range ps {
		if strings.Contains(p.text, "connection:x") || strings.Contains(p.text, "[^e1]:") {
			t.Fatalf("markers and footnote definitions are not paragraphs: %+v", p)
		}
	}
	folded := ""
	for k, p := range ps {
		if k > 0 && jaccard(signature(p.text), signature(ps[k-1].text)) >= 0.6 {
			folded += fmt.Sprintf("L%d [%s] ", p.line, difference(p.text, ps[k-1].text))
		}
	}
	if !strings.Contains(folded, "L11 [development/acco]") || !strings.Contains(folded, "L12 [development/acpe]") {
		t.Fatalf("alike paragraphs are found with what differs: %q", folded)
	}
}

func TestParentKeys(t *testing.T) {
	yml := "   1│ publishers:\n   2│   fenix:\n   3│     topic-id: x\n   4│   acknowledge:\n   5│     project-id: p\n   6│     topic-id: '${TOPIC_ACK}'"
	if got := parentKeys(yml); got != "acknowledge: publishers:" {
		t.Fatalf("parentKeys = %q", got)
	}
}

func TestReadRangesSectionsAndSingleLines(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/orders.md", "# orders\n\n## A\n\n- uno\n\n- dos\n\n## B\n\n- tres\n\n## C\n\n- cuatro\n")
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	read := func(o ReadOptions) string {
		var b bytes.Buffer
		o.Budget = ReadBudget
		if e := idx.Read(ctx, "orders", o, &b); e != nil {
			t.Fatal(e)
		}
		return b.String()
	}
	if out := read(ReadOptions{Lines: "5-5,15-15"}); !strings.Contains(out, "L5 - uno") || !strings.Contains(out, "L15 - cuatro") || strings.Contains(out, "dos") {
		t.Fatalf("two ranges:\n%s", out)
	}
	if out := read(ReadOptions{Lines: "6"}); !strings.Contains(out, "uno") {
		t.Fatalf("a single line on a blank takes the nearest paragraph:\n%s", out)
	}
	if out := read(ReadOptions{Section: "A|C"}); !strings.Contains(out, "uno") || !strings.Contains(out, "cuatro") || strings.Contains(out, "tres") {
		t.Fatalf("two sections:\n%s", out)
	}
}

func TestStemsStartWords(t *testing.T) {
	terms := askTerms("firma")
	if len(terms) != 1 || !terms[0].in(fold("el token firmado")) || terms[0].in(fold("hay que confirmar")) {
		t.Fatalf("a stem matches at a word start only: %+v", terms)
	}
}

func TestAskMapsEveryMatchingNote(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/orders.md", "---\naliases: [\"APP1-orders\"]\ntipo: api\n---\n# orders\n\n## Qué hace\n\n- Deduplicación por business ID antes de publicar. [^e1]\n\n- Publica el cierre con deduplicación previa.\n\n- Registra métricas de latencia.\n\n[^e1]: [src/dedup.go](https://github.com/acme/APP1-orders/blob/0123456789abcdef0123/src/dedup.go#L10-L20) — `Seen`\n")
	write("20-Repos/listener.md", "---\ntipo: api\n---\n# listener\n\nIntro.\n\nTambién publica el cierre por otra ruta.\n")
	write("20-Repos/billing.md", "---\ntipo: api\n---\n# billing\n\nFactura; deduplicación propia por folio al cierre.\n")
	write("90-Meta/Convenciones.md", "# Convenciones\n\nDeduplicación de notas al cierre.\n")
	write("investigations/20260101-x/investigation.md", "# Caso\n\nEl caso revisa la deduplicación del cierre.\n")
	out := askPack(t, opt, "¿Cómo deduplica APP1-orders el cierre?")
	for _, want := range []string{
		"# Vault map",
		"## Notes holding the question's terms (3, all of them; most terms first)",
		"- `20-Repos/orders.md` (named)",
		"`dedupl*` L9,11",
		"Paragraphs with these terms: ",
		"1 cite nothing",
		"- `20-Repos/listener.md`",
		"`cierr*` L8",
		"Sources: the note cites no footnotes",
		"investigations/20260101-x",
		"## Reading this map",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "90-Meta/") || strings.Contains(out, "métricas de latencia") {
		t.Fatalf("vault guidance is not knowledge, and the map copies no paragraph:\n%s", out)
	}
	// Named first, then the note holding more terms before the one holding one.
	if a, b, c := strings.Index(out, "`20-Repos/orders.md`"), strings.Index(out, "`20-Repos/billing.md`"), strings.Index(out, "`20-Repos/listener.md`"); !(a < b && b < c) {
		t.Fatalf("order named, more terms, fewer terms broken:\n%s", out)
	}
	if out = askPack(t, opt, "zanahoria morada"); !strings.Contains(out, "No note contains: zanahoria, morada") {
		t.Fatalf("unmatched terms not reported:\n%s", out)
	}
}

func TestAskMapStaysWithinItsBudget(t *testing.T) {
	opt, write := fixture(t)
	for k := range 60 {
		write(fmt.Sprintf("20-Repos/n%02d.md", k), fmt.Sprintf("---\ntipo: api\n---\n# n%02d\n\nEl cierre deduplica ventas %s.\n", k, strings.Repeat("x", k)))
	}
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	var b bytes.Buffer
	if e := idx.Ask(ctx, "cómo deduplica el cierre las ventas", AskOptions{Visibility: "all", Budget: 6000}, &b); e != nil {
		t.Fatal(e)
	}
	out := b.String()
	if n := len([]rune(out)); n > 6000 {
		t.Fatalf("map of %d characters exceeds its budget", n)
	}
	for _, want := range []string{"(60, all of them", "more, fewer terms each: 20-Repos/", "## Reading this map"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
}
