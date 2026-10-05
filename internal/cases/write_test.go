package cases

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConcurrentMutationsKeepEveryRecord(t *testing.T) {
	v := vault(t)
	res, e := run(t, "new", "--vault", v, "--title", "Concurrent records", "--type", "understanding", "--objective", "Keep every successful record.")
	if e != nil {
		t.Fatal(e)
	}
	const writers = 16
	start := make(chan struct{})
	errs := make(chan error, writers)
	for i := range writers {
		go func() {
			<-start
			o := options{vault: v, id: res["id"].(string), recKind: "question", text: fmt.Sprintf("Concurrent worker %02d", i), resolveBy: "Read the synthetic fixture"}
			var out bytes.Buffer
			errs <- mutate(o, &out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
				next, id, log, e := buildRecord(o, c, text, loc, nil)
				// A slow change lets competing writers read the same version without a lock.
				time.Sleep(20 * time.Millisecond)
				return next, log, map[string]any{"record": id}, e
			}, nil)
		}()
	}
	close(start)
	for range writers {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	b, e := os.ReadFile(filepath.Join(v, res["path"].(string)))
	if e != nil {
		t.Fatal(e)
	}
	if got := len(definedRecords(string(b))); got != writers {
		t.Fatalf("%d successful mutations retained %d records", writers, got)
	}
	for i := range writers {
		if !strings.Contains(string(b), fmt.Sprintf("Concurrent worker %02d", i)) {
			t.Fatalf("successful worker %d was lost", i)
		}
	}
	if checked, e := Check(v, res["path"].(string)); e != nil || !checked.OK {
		t.Fatalf("concurrent records must pass the gate: %+v %v", checked, e)
	}
}

func TestMutationRefusesToOverwriteAnExternalEdit(t *testing.T) {
	v := vault(t)
	res, e := run(t, "new", "--vault", v, "--title", "External edit", "--type", "understanding", "--objective", "Keep manual edits.")
	if e != nil {
		t.Fatal(e)
	}
	full := filepath.Join(v, res["path"].(string))
	var manual string
	var copied bool
	var out bytes.Buffer
	e = mutate(options{vault: v, id: res["id"].(string)}, &out, func(c Case, text, loc string) (string, []string, map[string]any, error) {
		manual = text + "\nManual edit made during the command.\n"
		if e := os.WriteFile(full, []byte(manual), 0o640); e != nil {
			return "", nil, nil, e
		}
		return replaceSection(text, "state", loc, "Updated by the command."), nil, nil, nil
	}, func(Case) error {
		copied = true
		return nil
	})
	if e == nil || !strings.Contains(e.Error(), "changed") {
		t.Fatalf("an external edit must stop the replacement: %v", e)
	}
	if copied {
		t.Fatal("attached files must not be copied for a case that will not be replaced")
	}
	if b, _ := os.ReadFile(full); string(b) != manual {
		t.Fatal("the external edit was overwritten")
	}
}
