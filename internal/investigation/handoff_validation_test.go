package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func dhFixture() (string, dhEntry) {
	f := map[string]string{"Story ID": "S-001", "Tracker ID": "jira", "Provider": "jira", "Tracker URL": "https://example.com", "Work item reference": "ÉVIDENCE-ßﬀ-42", "Repository remote": "example.com/team/repo", "Branch": "feature/évidence", "Revision": "v0001", "Materialized at": "2026-09-24T12:00:00-03:00"}
	sum := sha256.Sum256([]byte(f["Tracker ID"] + "|" + f["Work item reference"] + "|" + f["Repository remote"]))
	f["Handoff ID"] = hex.EncodeToString(sum[:])
	f["Family"] = dhSlug(f["Tracker ID"]+"-"+f["Work item reference"]) + "-" + f["Handoff ID"][:10] + "--repo"
	e := dhEntry{"DH-001", "001", "jira:" + f["Work item reference"] + " / repo", f}
	var b strings.Builder
	b.WriteString("## Development handoffs\n\n### " + e.id + " — " + e.title + "\n")
	for _, field := range dhFields {
		b.WriteString("- " + field + ": " + f[field] + "\n")
	}
	b.WriteString("\n## History\n" + dhEvent(dhBinding(e)) + "\n  <!-- " + dhMarker + " " + dhJSON(dhBinding(e)) + " -->\n")
	return b.String(), e
}
func dhEvent(m map[string]string) string {
	return "- " + m["materialized-at"] + " — Vinculó development handoff `" + m["dh"] + "`; story `" + m["story-id"] + "`; work item `" + m["tracker-id"] + ":" + m["work-item-reference"] + "`; repository `" + m["repository-remote"] + "`; branch `" + m["branch"] + "`; handoff `" + m["handoff-id"] + "`; revision `" + m["revision"] + "`"
}
func TestHandoffValidation(t *testing.T) {
	text, _ := dhFixture()
	if errs := validateHandoffs(text); len(errs) > 0 {
		t.Fatal(errs)
	}
	cases := []struct{ name, old, new, want string }{
		{"duplicate key", `{"branch":`, `{"dh":"DH-001","branch":`, "duplicate keys: dh"},
		{"identity", "- Handoff ID: ", "- Handoff ID: f", "Handoff ID does not match its identity"},
		{"history missing", "## History", "## Other", "must appear only in History"},
		{"offset", "2026-09-24T12:00:00-03:00", "2026-09-24T12:00:00", "invalid Materialized at"},
		{"id gap", "DH-001", "DH-002", "contiguous from DH-001"},
		{"visible target", "; branch `feature/évidence`", "; branch `other`", "exact binding target"},
		{"revision coverage", "v0001", "v0002", "cover each Revision"},
		{"missing field", "- Story ID: S-001\n", "", "missing fields Story ID"},
		{"order", "- Story ID: S-001\n- Tracker ID: jira", "- Tracker ID: jira\n- Story ID: S-001", "fields are out of order"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateHandoffs(strings.ReplaceAll(text, c.old, c.new))
			if !strings.Contains(strings.Join(err, "\n"), c.want) {
				t.Fatalf("want %s; got %v", c.want, err)
			}
		})
	}
}
func TestHandoffHistoryChronology(t *testing.T) {
	text, e := dhFixture()
	e.fields["Revision"] = "v0002"
	e.fields["Materialized at"] = "2026-09-23T12:00:00-03:00"
	text = strings.Replace(text, "- Revision: v0001", "- Revision: v0002", 1)
	text = strings.Replace(text, "- Materialized at: 2026-09-24T12:00:00-03:00", "- Materialized at: "+e.fields["Materialized at"], 1)
	text += dhEvent(dhBinding(e)) + "\n  <!-- " + dhMarker + " " + dhJSON(dhBinding(e)) + " -->\n"
	if errs := validateHandoffs(text); !strings.Contains(strings.Join(errs, "\n"), "preserve Revision chronology") {
		t.Fatal(errs)
	}
}
func TestHandoffValidationPythonParity(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("Python reference unavailable")
	}
	script, err := filepath.Abs("../../kernel/.agents/skills/manage-investigation/scripts/investigation-case.py")
	if err != nil {
		t.Fatal(err)
	}
	text, _ := dhFixture()
	samples := []string{text, "", "## Development handoffs\n", "## Development handoffs\n## Development handoffs\n", strings.Replace(text, "## History", "## Historial", 1)}
	for _, pair := range [][2]string{{"v0001", "v0000"}, {"v0001", "v0002"}, {"DH-001", "DH-003"}, {"- Provider: jira", "- Provider: A"}, {"- Family: ", "- Family: bad"}, {"- Story ID: S-001\n", ""}, {"2026-09-24T12:00:00-03:00", "2026-09-24T12:00:00"}, {"https://example.com", "https://EXAMPLE.com/"}, {`{"branch":`, `{"dh":"DH-001","branch":`}, {"## History", "## Somewhere else"}, {"Vinculó development", "— development"}} {
		samples = append(samples, strings.ReplaceAll(text, pair[0], pair[1]))
	}
	payload, _ := json.Marshal(samples)
	cmd := exec.Command(python, "-c", `import sys,json,importlib.util
s=importlib.util.spec_from_file_location("investigation_ref",sys.argv[1]);m=importlib.util.module_from_spec(s);sys.modules[s.name]=m;s.loader.exec_module(m)
print(json.dumps([m.validate_development_handoffs(x) for x in json.load(sys.stdin)],ensure_ascii=False))`, script)
	cmd.Stdin = strings.NewReader(string(payload))
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("reference failed: %v %s", e, out)
	}
	var want [][]string
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	for i, s := range samples {
		got := validateHandoffs(s)
		if len(got) == 0 && len(want[i]) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want[i]) {
			t.Errorf("case %d\ngot %v\nwant %v", i, got, want[i])
		}
	}
}
func TestHandoffCanonicalJSONUnicode(t *testing.T) {
	raw := dhJSON(map[string]string{"v": "<é>\u2028\u2029\\u2028\n"})
	want := "{\"v\":\"<é>\u2028\u2029\\\\u2028\\n\"}"
	if raw != want {
		t.Fatalf("%q != %q", raw, want)
	}
}
