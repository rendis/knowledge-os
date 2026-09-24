package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T) (Options, func(string, string)) {
	t.Helper()
	root := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		full := filepath.Join(root, p)
		if e := os.MkdirAll(filepath.Dir(full), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(full, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("instance.yaml", "cell:\n  name: Test\n")
	write("00-Home.md", "# Test\n")
	return Options{Vault: root, Cache: t.TempDir()}, write
}
func TestRefreshLifecycleAndRanking(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/Reader.md", "---\ntitle: Reader IoT\naliases: [antena]\n---\n# Reader\n## Reintentos\nTimeout de recepción persistente.\n")
	ctx := context.Background()
	idx, e := Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	r, e := idx.Search(ctx, "recepcion", 5, "all")
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Cards) != 1 || r.Cards[0].Section != "Reader > Reintentos" || !strings.Contains(r.Cards[0].Excerpt, "recepción") {
		t.Fatalf("%+v", r)
	}
	old := r.Cards[0].SHA256
	idx.Close()
	idx, e = Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	if idx.Stats.Updated != 0 || idx.Stats.Unchanged != 2 {
		t.Fatal(idx.Stats)
	}
	idx.Close()
	write("20-Repos/Reader.md", "# Reader\nNuevo firmware Zebra\n")
	idx, e = Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	r, e = idx.Search(ctx, "recepcion", 5, "all")
	if e != nil || len(r.Cards) != 0 {
		t.Fatal(r, e)
	}
	r, e = idx.Search(ctx, "Zebra", 5, "all")
	if e != nil || len(r.Cards) != 1 || r.Cards[0].SHA256 == old {
		t.Fatal(r, e)
	}
	idx.Close()
	if e = os.Rename(filepath.Join(opt.Vault, "20-Repos/Reader.md"), filepath.Join(opt.Vault, "20-Repos/New.md")); e != nil {
		t.Fatal(e)
	}
	idx, e = Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	if idx.Stats.Deleted != 1 || idx.Stats.Updated != 1 {
		t.Fatal(idx.Stats)
	}
	idx.Close()
	if e = os.Remove(filepath.Join(opt.Vault, "20-Repos/New.md")); e != nil {
		t.Fatal(e)
	}
	idx, e = Open(ctx, opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	r, e = idx.Search(ctx, "Zebra", 5, "all")
	if e != nil || len(r.Cards) != 0 {
		t.Fatal(r, e)
	}
}
func TestPrivateScopeLiteralQueryAndBoundedCards(t *testing.T) {
	opt, write := fixture(t)
	write(".investigations-private/case/investigation.md", "# Secreto\nTokenPrivado "+strings.Repeat("contenido ", 2000))
	write(".scratch/task/secret.md", "TokenPrivado")
	write("AGENTS.personal.md", "TokenPrivado")
	idx, e := Open(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	r, e := idx.Search(context.Background(), "TokenPrivado", 5, "all")
	if e != nil || len(r.Cards) != 1 || r.Cards[0].Visibility != "private" || len([]rune(r.Cards[0].Excerpt)) > 361 {
		t.Fatal(r, e)
	}
	r, e = idx.Search(context.Background(), "TokenPrivado", 5, "public")
	if e != nil || len(r.Cards) != 0 {
		t.Fatal(r, e)
	}
	for _, q := range []string{"\" OR * NOT ()", "---", ""} {
		if _, e = idx.Search(context.Background(), q, 5, "all"); e != nil {
			t.Fatalf("query %q: %v", q, e)
		}
	}
}
func TestAtomicRollbackAndRebuild(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/a.md", "# Original\noriginaltoken")
	idx, e := Open(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	idx.Close()
	write("20-Repos/a.md", "# New\nnewtoken")
	write("20-Repos/z.md", "---\ninvalid: [\n---\n")
	if idx, e = Open(context.Background(), opt); e == nil {
		idx.Close()
		t.Fatal("invalid source accepted")
	}
	// Recovery after failed refresh must neither duplicate nor retain stale rows.
	if e = os.Remove(filepath.Join(opt.Vault, "20-Repos/z.md")); e != nil {
		t.Fatal(e)
	}
	idx, e = Open(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	if idx.Stats.Updated != 1 {
		t.Fatal(idx.Stats)
	}
	idx.Close()
	opt.Rebuild = true
	idx, e = Open(context.Background(), opt)
	if e != nil {
		t.Fatal(e)
	}
	defer idx.Close()
	r, e := idx.Search(context.Background(), "newtoken", 10, "all")
	if e != nil || len(r.Cards) != 1 {
		t.Fatal(r, e)
	}
}
func TestConcurrentRefresh(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/a.md", "# Concurrent\nconcurrenttoken")
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx, e := Open(context.Background(), opt)
			if e != nil {
				errors <- e
				return
			}
			defer idx.Close()
			_, e = idx.Search(context.Background(), "concurrenttoken", 5, "all")
			if e != nil {
				errors <- e
			}
		}()
	}
	wg.Wait()
	close(errors)
	for e := range errors {
		t.Error(e)
	}
}
func TestRejectSymlinkAndVaultCache(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/a.md", "# a")
	bad := opt
	bad.Cache = filepath.Join(opt.Vault, "cache")
	if idx, e := Open(context.Background(), bad); e == nil {
		idx.Close()
		t.Fatal("vault cache accepted")
	}
	if e := os.Symlink(filepath.Join(opt.Vault, "20-Repos/a.md"), filepath.Join(opt.Vault, "20-Repos/b.md")); e != nil {
		t.Skip(e)
	}
	if idx, e := Open(context.Background(), opt); e == nil {
		idx.Close()
		t.Fatal("symlink accepted")
	}
}
func TestMarkdownHeadingsDoNotUseCodeFence(t *testing.T) {
	d, e := parseDocument("20-Repos/test.md", []byte("# Real\n```text\n# Fake\n```\ncontenido\n"))
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range d.Passages {
		if strings.Contains(p.Section, "Fake") {
			t.Fatal(d)
		}
	}
}

func TestAllKnowledgeDirectoriesAreSearchable(t *testing.T) {
	opt, write := fixture(t)
	for _, dir := range []string{"10-Sistemas", "15-Arquitectura", "20-Repos", "25-Topics", "30-Flujos", "40-Integraciones", "50-Glosario", "60-Operacion", "70-Aprendizajes"} {
		write(dir+"/Reference.md", "# Reference\nScopeCoverageToken\n")
	}
	idx, err := Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	result, err := idx.Search(context.Background(), "ScopeCoverageToken", 10, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cards) != 9 {
		t.Fatalf("missing knowledge directory: %#v", result)
	}
}

func TestSQLiteFileURLWindowsDrive(t *testing.T) {
	for _, path := range []string{"C:/Users/Test User/cache#1/db.sqlite", "/tmp/Test User/cache#1/db.sqlite"} {
		u := sqliteFileURL(path)
		if u.Host != "" || !strings.HasPrefix(u.String(), "file:///") || strings.Contains(u.String(), "cache#") {
			t.Fatalf("invalid file URI for %q: %s", path, u.String())
		}
		if strings.HasPrefix(path, "C:") && !strings.HasPrefix(u.String(), "file:///C:/") {
			t.Fatal(u.String())
		}
	}
}

func TestCLIRejectsInvalidInstanceBeforeIndexing(t *testing.T) {
	opt, _ := fixture(t)
	err := Run(context.Background(), "search", []string{"--vault", opt.Vault, "--cache", opt.Cache, "--query", "anything"}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "invalid vault configuration") {
		t.Fatalf("invalid identity accepted: %v", err)
	}
	entries, err := os.ReadDir(opt.Cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid vault created index: %v %v", entries, err)
	}
}
