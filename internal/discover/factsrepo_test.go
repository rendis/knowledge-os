package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactsRepoResolvesTheNamesAReaderHas(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, stateRel, "facts")
	if e := os.MkdirAll(dir, 0o755); e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"APP1-orders", "APP1-billing", "APP2-billing"} {
		if e := os.WriteFile(filepath.Join(dir, n+".json"), []byte(`{"repo":"`+n+`"}`), 0o644); e != nil {
			t.Fatal(e)
		}
	}
	cmp := `[{"repo":"APP2-billing","note":"20-Repos/facturacion.md"}]`
	if e := os.WriteFile(filepath.Join(vault, stateRel, "comparison.json"), []byte(cmp), 0o644); e != nil {
		t.Fatal(e)
	}
	for name, want := range map[string]string{"app1-orders": "APP1-orders", "orders": "APP1-orders", "facturacion": "APP2-billing"} {
		if got, e := factsRepo(vault, name); e != nil || got != want {
			t.Fatalf("factsRepo(%q) = %q, %v; want %q", name, got, e, want)
		}
	}
	if _, e := factsRepo(vault, "billing"); e == nil || !strings.Contains(e.Error(), "known repositories: APP1-billing, APP1-orders, APP2-billing") {
		t.Fatalf("an ambiguous name lists the known repositories: %v", e)
	}
}
