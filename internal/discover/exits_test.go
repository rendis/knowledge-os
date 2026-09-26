package discover

import (
	"fmt"
	"strings"
	"testing"
)

func TestExitsOfAFunction(t *testing.T) {
	ts := strings.Split(`class Handler {
  process = async (id: string): Promise<boolean> => {
    if (isHealth(id)) {
      return true
    }
    try {
      items.forEach((x) => {
        return x
      })
      await save(id)
    } catch (e: any) {
      log.error('Error save data', e)
      await guard.rollback(id)
      return false
    }
    try { await notify(id) } catch (e) { log.warn('notify failed') }
    if (bad) {
      throw new OrderAlreadyExistsError('order already exists')
    }
    res.status(409).json({})
    process.exit(1)
    return true
  }
}`, "\n")
	if end := functionEnd(ts, 2, "h.ts"); end != 23 {
		t.Fatalf("function end = %d, want 23", end)
	}
	exits := exitsIn(ts, 2, 23, "h.ts")
	got := []string{}
	for _, e := range exits {
		got = append(got, e.Kind)
	}
	want := "returns true|returns false|catches and continues|throws|responds 409|exits the process|returns true"
	if strings.Join(got, "|") != want {
		t.Fatalf("exits = %v", got)
	}
	if exits[1].At != 11 || !strings.Contains(exits[1].When, "catch") {
		t.Fatalf("return false is under the catch: %+v", exits[1])
	}
	names := strings.Join(exits[1].Names, ",")
	if !strings.Contains(names, "Error save data") || !strings.Contains(names, "rollback") {
		t.Fatalf("the path is named by what it logs and calls: %s", names)
	}
	if n := strings.Join(exits[3].Names, ","); !strings.Contains(n, "OrderAlreadyExistsError") || !strings.Contains(n, "order already exists") {
		t.Fatalf("throw names: %s", n)
	}
	if ExitCall("    process.exit(1)") != "process.exit(" {
		t.Fatal("ExitCall")
	}
}

func TestExitsGoAndPython(t *testing.T) {
	g := strings.Split(`func Handle(m Msg) error {
	if err := decode(m); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if err := store(m); err != nil {
		return err
	}
	if m.Dup {
		m.Ack()
		return nil
	}
	return ErrNotSaved
}

func Other() {}`, "\n")
	if end := functionEnd(g, 1, "h.go"); end != 13 {
		t.Fatalf("go end = %d", end)
	}
	got := []string{}
	for _, e := range exitsIn(g, 1, 13, "h.go") {
		got = append(got, e.Kind)
	}
	if strings.Join(got, "|") != "returns an error|propagates|acks|returns nil|returns an error" {
		t.Fatalf("go exits = %v", got)
	}
	py := strings.Split(`def handle(m):
    try:
        save(m)
    except ValueError:
        log.warning("bad payload")
    if m.dup:
        raise DuplicateOrder("dup")
    return True

def other():
    pass`, "\n")
	if end := functionEnd(py, 1, "h.py"); end != 8 {
		t.Fatalf("python end = %d", end)
	}
	got = got[:0]
	for _, e := range exitsIn(py, 1, 8, "h.py") {
		got = append(got, e.Kind)
	}
	if strings.Join(got, "|") != "catches and continues|throws|returns True" {
		t.Fatalf("python exits = %v", got)
	}
}

func TestExitsOfARetryLoop(t *testing.T) {
	src := strings.Split(`func send(n int) error {
	for i := 0; i < n; i++ {
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Error sending request", err)
			continue
		}
		switch resp.StatusCode {
		case 200:
			break
		}
		if resp.StatusCode == 202 {
			break
		}
	}
	log.Println("Maximum retrying attempts reached")
	return nil
}`, "\n")
	got := []string{}
	for _, e := range exitsIn(src, 1, functionEnd(src, 1, "s.go"), "s.go") {
		got = append(got, fmt.Sprintf("L%d %s", e.Line, e.Kind))
	}
	if strings.Join(got, "|") != "L6 goes to the next iteration|L13 leaves the loop|L17 returns nil" {
		t.Fatalf("loop exits = %v", got)
	}
}

func TestWithinKeepsTheMatchingBlocks(t *testing.T) {
	src := []string{"func f() {"}
	for k := 2; k <= 40; k++ {
		src = append(src, fmt.Sprintf("\tstep%d()", k))
	}
	src = append(src, "}")
	f := Function{Start: 1, src: src, hits: []int{30, 31}}
	got := f.Within(6)
	if !strings.HasPrefix(got, "   1│ func f() {\n    │ …\n  28│") || !strings.Contains(got, "  31│ \tstep31()") || strings.Contains(got, "   2│") {
		t.Fatalf("within keeps the first line and the matching block:\n%s", got)
	}
}
