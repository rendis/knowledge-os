package discover

import "testing"

func TestClaimsChecksBareNames(t *testing.T) {
	vault := t.TempDir()
	write(t, vault, "instance.yaml", "version: 1\ncell:\n  name: \"C\"\n  purpose: \"p\"\nsystems:\n  - id: \"s\"\n    name: \"S\"\n")
	write(t, vault, "25-Topics/acme.sales.orders-acked.md", "# acme.sales.orders-acked\n")
	write(t, vault, stateRel+"/facts/SVC-x.json", `{"repo":"SVC-x"}`)
	write(t, vault, stateRel+"/comparison.json", `[]`)
	res, e := checkClaims(vault, "El servicio publica en acme.sales.orders-acked y en acme.sales.inventado-topic; es fire-and-forget.")
	if e != nil {
		t.Fatal(e)
	}
	if res["names_checked"] != 2 || res["ok"] != false {
		t.Fatalf("bare names: %v", res)
	}
	res, _ = checkClaims(vault, "Solo prosa.")
	if res["note"] == nil {
		t.Fatalf("nothing checked must say so: %v", res)
	}
}
