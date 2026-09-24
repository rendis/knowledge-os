package retrieval

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestNeighborsPublishedInvestigationPointers(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/Reader-X.md", "# Reader-X\n[[Database]]\n")
	write("20-Repos/Database.md", "# Database\n")
	write("investigations/one/investigation.md", "# Radio investigation\n[[Reader-X|reader]]\n## Additional\nReader-X is mentioned again.\n")
	write("investigations/two/investigation.md", "# "+strings.Repeat("á", 1000)+"\nFailure reported in READER-X.\n")
	write("investigations/three/investigation.md", "# Unrelated\nOther component.\n")
	write("investigations/one/artifacts/copy/investigation.md", "# Resource copy\nReader-X\n")
	write(".investigations/local/investigation.md", "# Local case\nReader-X\n")
	write(".investigations-private/one/investigation.md", "# Private overlay\nReader-X\n")
	idx, err := Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	r, err := idx.Neighbors(context.Background(), "Reader-X")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Investigations) != 2 || r.InvestigationsTruncated {
		t.Fatalf("%+v", r)
	}
	for n, path := range []string{"investigations/one/investigation.md", "investigations/two/investigation.md"} {
		p := r.Investigations[n]
		if p.Path != path || p.Title == "" || p.Origin != "published-investigation" {
			t.Fatalf("%+v", p)
		}
		if len([]rune(p.Title)) > 241 {
			t.Fatal("investigation title exceeds output bound")
		}
	}
	if len(r.Outgoing) != 1 || r.Outgoing[0].Target != "Database" {
		t.Fatalf("lost graph edges: %+v", r)
	}
	r, err = idx.Neighbors(context.Background(), "Database")
	if err != nil || len(r.Investigations) != 0 || r.InvestigationsTruncated {
		t.Fatal(r, err)
	}
}

func TestNeighborsInvestigationPointersBoundedAndLiteral(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/A_%.md", "# A_%\n")
	for n := 0; n < investigationPointerLimit+1; n++ {
		write(fmt.Sprintf("investigations/case-%02d/investigation.md", n), "# Case\nA_% appears here.\n")
	}
	write("investigations/nonliteral/investigation.md", "# Other\nAnything unrelated\n")
	idx, err := Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	r, err := idx.Neighbors(context.Background(), "A_%")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Investigations) != investigationPointerLimit || !r.InvestigationsTruncated {
		t.Fatalf("%+v", r)
	}
	if r.Investigations[0].Path != "investigations/case-00/investigation.md" || r.Investigations[19].Path != "investigations/case-19/investigation.md" {
		t.Fatalf("unstable ordering: %+v", r.Investigations)
	}
}
