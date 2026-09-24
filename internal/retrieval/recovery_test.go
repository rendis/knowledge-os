package retrieval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKilledWriter(t *testing.T) {
	if os.Getenv("VAULTCTL_CRASH_HELPER") == "1" {
		idx, err := Open(context.Background(), Options{Vault: os.Getenv("VAULTCTL_TEST_VAULT"), Cache: os.Getenv("VAULTCTL_TEST_CACHE")})
		if err != nil {
			panic(err)
		}
		tx, err := idx.db.Begin()
		if err != nil {
			panic(err)
		}
		if _, err = tx.Exec(`DELETE FROM passages`); err != nil {
			panic(err)
		}
		fmt.Println("transaction-open")
		time.Sleep(time.Minute)
		os.Exit(9)
	}
	opt, write := fixture(t)
	write("20-Repos/a.md", "# Durable\ndurabletoken")
	idx, err := Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	idx.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestKilledWriter$")
	cmd.Env = append(os.Environ(), "VAULTCTL_CRASH_HELPER=1", "VAULTCTL_TEST_VAULT="+opt.Vault, "VAULTCTL_TEST_CACHE="+opt.Cache)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if strings.TrimSpace(line) != "transaction-open" {
			t.Fatalf("helper not ready: %q", line)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("helper timed out")
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	idx, err = Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	r, err := idx.Search(context.Background(), "durabletoken", 5, "all")
	if err != nil || len(r.Cards) != 1 {
		t.Fatal(r, err)
	}
}
func TestGroupAndLinksRefresh(t *testing.T) {
	opt, write := fixture(t)
	write("20-Repos/a.md", "---\nconsume-de: ['[[b]]']\n---\n# a\n[[b]]\n")
	write("20-Repos/b.md", "# b\n")
	write("investigations/case/investigation.md", "# antenna\nantenna")
	write("investigations/case/artifacts/report.md", "# antenna\nantenna")
	write(".investigations-private/case/investigation.md", "# antenna\nantenna")
	idx, err := Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	r, err := idx.Search(context.Background(), "antenna", 10, "all")
	if err != nil || len(r.Cards) != 1 {
		t.Fatal(r, err)
	}
	links, err := idx.Neighbors(context.Background(), "b")
	if err != nil || len(links.Incoming) != 1 || links.Incoming[0].Field != "consume-de" {
		t.Fatal(links, err)
	}
	idx.Close()
	if err = os.Remove(filepath.Join(opt.Vault, "20-Repos/a.md")); err != nil {
		t.Fatal(err)
	}
	idx, err = Open(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	links, err = idx.Neighbors(context.Background(), "b")
	if err != nil || len(links.Incoming) != 0 {
		t.Fatal(links, err)
	}
}
