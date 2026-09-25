package handoff

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func policyWrite(t *testing.T, root, name string, b []byte) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(root, name), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestPolicyInstructionPreservesBOMNewlinesAndPlacement(t *testing.T) {
	root := t.TempDir()
	original := []byte("\xef\xbb\xbf---\r\nkind: root\r\n---\r\n\r\n# Project\r\n\r\nKeep local instructions.\r\n")
	policyWrite(t, root, "AGENTS.md", original)
	desired, e := prepareInstructions(root)
	if e != nil {
		t.Fatal(e)
	}
	raw := desired["AGENTS.md"]
	if !bytes.HasPrefix(raw, []byte("\xef\xbb\xbf---\r\n")) || !bytes.Contains(raw, []byte("# Project\r\n\r\n"+managedBegin)) || !bytes.Contains(raw, []byte("Keep local instructions.\r\n")) {
		t.Fatalf("did not preserve content/placement")
	}
	if strings.Contains(strings.ReplaceAll(string(raw), "\r\n", ""), "\n") {
		t.Fatal("mixed newlines")
	}
	policyWrite(t, root, "AGENTS.md", raw)
	again, e := prepareInstructions(root)
	if e != nil || !bytes.Equal(again["AGENTS.md"], raw) {
		t.Fatalf("not idempotent: %v", e)
	}
}
func TestPolicyInstructionsOnlyAgents(t *testing.T) {
	for _, existingAgents := range []bool{false, true} {
		root := t.TempDir()
		// Even malformed former managed content belongs to the repository now.
		original := []byte("# Local\n" + managedBegin)
		policyWrite(t, root, "CLAUDE.md", original)
		if existingAgents {
			policyWrite(t, root, "AGENTS.md", []byte("# Project\n"))
		}
		desired, err := prepareInstructions(root)
		if err != nil || len(desired) != 1 || desired["AGENTS.md"] == nil {
			t.Fatalf("AGENTS-only policy: %v", err)
		}
		for name, data := range desired {
			policyWrite(t, root, name, data)
		}
		actual, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
		if err != nil || !bytes.Equal(actual, original) {
			t.Fatal("repository CLAUDE.md changed")
		}
	}
}
func TestPolicyInstructionsPreserveClaudeSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Skip(err)
	}
	desired, err := prepareInstructions(root)
	if err != nil || len(desired) != 1 || desired["AGENTS.md"] == nil {
		t.Fatalf("unrelated CLAUDE symlink affected policy: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(root, "CLAUDE.md")); err != nil || target != "missing" {
		t.Fatal("CLAUDE symlink changed")
	}
}
func TestPolicyInstructionsRejectAgentsSymlinkAndOverride(t *testing.T) {
	root := t.TempDir()
	policyWrite(t, root, "CLAUDE.md", []byte("# Local\n"))
	if err := os.Symlink("CLAUDE.md", filepath.Join(root, "AGENTS.md")); err != nil {
		t.Skip(err)
	}
	if _, err := prepareInstructions(root); err == nil {
		t.Fatal("AGENTS symlink accepted")
	}
	if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	policyWrite(t, root, "AGENTS.override.md", []byte("override"))
	if _, err := prepareInstructions(root); err == nil {
		t.Fatal("override accepted")
	}
}
func TestPolicyRejectUnsafeLinksAndBrokenBlocks(t *testing.T) {
	for _, input := range []string{managedBegin, "<!-- BEGIN MANAGED: System A-System B DEVELOPMENT HANDOFF -->", managedEnd + managedBegin, managedBegin + managedEnd + managedBegin + managedEnd} {
		t.Run(input, func(t *testing.T) {
			root := t.TempDir()
			policyWrite(t, root, "AGENTS.md", []byte(input))
			if _, e := prepareInstructions(root); e == nil {
				t.Fatal("invalid block accepted")
			}
		})
	}
	root := t.TempDir()
	if e := os.Symlink("missing", filepath.Join(root, "AGENTS.md")); e != nil {
		t.Skip(e)
	}
	if _, e := prepareInstructions(root); e == nil {
		t.Fatal("dangling symlink accepted")
	}
}
func TestPolicyIgnore(t *testing.T) {
	root := t.TempDir()
	policyWrite(t, root, ".gitignore", []byte("\xef\xbb\xbfbuild/\r\n"))
	b, e := prepareIgnore(root)
	if e != nil || string(b) != "\xef\xbb\xbfbuild/\r\n/.knowledge-os-handoffs/\r\n" {
		t.Fatalf("ignore %q %v", b, e)
	}
	policyWrite(t, root, ".gitignore", b)
	again, e := prepareIgnore(root)
	if e != nil || !bytes.Equal(b, again) {
		t.Fatal("not idempotent")
	}
	policyWrite(t, root, ".gitignore", []byte("/.knowledge-os-handoffs/\n!.knowledge-os-handoffs/file\n"))
	if _, e = prepareIgnore(root); e == nil {
		t.Fatal("conflict accepted")
	}
}
func updateFixture() string {
	return "# Implementation updates\n\n## UPD-001 — Change\n- Recorded at: 2026-09-24T12:00:00Z\n- Source or trigger: request\n- Initial definition affected: scope\n- Update: changed scope\n- Status: agreed\n- Reason or agreement: explicit\n- Impact: bounded\n- Evidence: local source\n- Related entries: none\n"
}
func TestPolicyUpdates(t *testing.T) {
	root := t.TempDir()
	action, result, e := prepareUpdates(root)
	if e != nil || action != "create" || result["entries"] != 0 {
		t.Fatal(action, result, e)
	}
	path := filepath.Join(root, updatesName)
	policyWrite(t, root, updatesName, updatesAsset())
	summary, e := validateUpdates(path)
	if e != nil || summary["entries"] != 0 || summary["latest"] != nil {
		t.Fatal(summary, e)
	}
	policyWrite(t, root, updatesName, []byte(updateFixture()))
	summary, e = validateUpdates(path)
	if e != nil || summary["entries"] != 1 || summary["latest"] != "UPD-001" {
		t.Fatal(summary, e)
	}
	for name, text := range map[string]string{
		"gap":       strings.Replace(updateFixture(), "UPD-001", "UPD-002", 1),
		"future":    strings.Replace(updateFixture(), "entries: none", "entries: UPD-001", 1),
		"status":    strings.Replace(updateFixture(), "Status: agreed", "Status: done", 1),
		"timezone":  strings.Replace(updateFixture(), "12:00:00Z", "12:00:00", 1),
		"analysis":  updateFixture() + "- Analysis: n/a\n",
		"duplicate": updateFixture() + "- Impact: another\n",
		"missing":   strings.Replace(updateFixture(), "- Impact: bounded\n", "", 1),
		"secret":    updateFixture() + "password=live-secret\n",
		"heading":   updateFixture() + "## Other\n",
		"nul":       updateFixture() + "\x00",
	} {
		t.Run(name, func(t *testing.T) {
			policyWrite(t, root, updatesName, []byte(text))
			if _, e := validateUpdates(path); e == nil {
				t.Fatal("invalid updates accepted")
			}
		})
	}
}

func TestPolicyTimestampISOForms(t *testing.T) {
	for _, s := range []string{"2026-09-24T12:00:00Z", "20260924T120000+0000", "2026-09-24 12:00+00:00", "2026-W39-4T12:00:00Z", "2026W394T12:00:00Z", "2026-09-24_12:00:00,123+00:00", "2026-09-24☀12:00:00Z"} {
		if !updateTimestamp(s) {
			t.Errorf("rejected %s", s)
		}
	}
	for _, s := range []string{"2026-09-24T12:00:00", "2026-W99-4T12:00:00Z", "2026-09-99T12:00:00Z"} {
		if updateTimestamp(s) {
			t.Errorf("accepted %s", s)
		}
	}
}
