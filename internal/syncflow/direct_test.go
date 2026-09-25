package syncflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type directTestFixture struct {
	dir, repo, vault, candidate, evidence, state, ref, oldOID, newOID, note, baseText string
	manifest, review                                                                  map[string]any
}

func newDirectFixture(t *testing.T, candidateText string) *directTestFixture {
	t.Helper()
	dir := t.TempDir()
	repo := sourceRepo(t)
	if _, e := sourceGit(repo, nil, "remote", "add", "origin", "https://example.invalid/team/APP00001-example.git"); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(repo, "source.txt"), "old\n")
	oldOID := sourceCommit(t, repo)
	write(t, filepath.Join(repo, "source.txt"), "new\n")
	newOID := sourceCommit(t, repo)
	refBytes, e := sourceGit(repo, nil, "symbolic-ref", "HEAD")
	if e != nil {
		t.Fatal(e)
	}
	vault := filepath.Join(dir, "vault")
	candidate := filepath.Join(dir, "candidate")
	evidence := filepath.Join(dir, "evidence")
	state := filepath.Join(dir, "state")
	note := "20-Repos/example.md"
	baseText := "---\naliases: [APP00001-example]\ncommit-analizado: " + oldOID[:12] + "\nrama-analizada: master\n---\nold note\n"
	write(t, filepath.Join(vault, note), baseText)
	if candidateText == "old note\n" {
		candidateText = baseText
	}
	write(t, filepath.Join(candidate, note), candidateText)
	write(t, filepath.Join(evidence, "source.txt"), "source evidence\n")
	outcome := "write"
	if candidateText == baseText {
		outcome = "no-documentation-change"
	}
	source := directSourceArg{"APP00001-example", repo, strings.TrimSpace(string(refBytes)), oldOID, newOID, outcome}
	manifest, issues, e := freezeBound(vault, candidate, evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
	if e != nil || len(issues) != 0 {
		t.Fatal(e, issues)
	}
	d, _ := digest(manifest)
	review := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	return &directTestFixture{dir, repo, vault, candidate, evidence, state, source.productionRef, oldOID, newOID, note, baseText, manifest, review}
}

func (f *directTestFixture) checkouts() map[string]string {
	return map[string]string{"APP00001-example": f.repo}
}

func TestDirectPublishExactBytesAndReplay(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	result, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0)
	if e != nil || result["reused"] != false {
		t.Fatal(result, e)
	}
	body, e := os.ReadFile(filepath.Join(f.vault, f.note))
	if e != nil || string(body) != "new note\n" {
		t.Fatal("reviewed bytes were not published")
	}
	result, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0)
	if e != nil || result["reused"] != true {
		t.Fatal(result, e)
	}
}

func TestDirectLifecycleThroughPublicCLI(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	manifestPath := filepath.Join(f.dir, "direct-manifest.json")
	reviewPath := filepath.Join(f.dir, "direct-review.json")
	frozen := call(t, "review", "freeze", "--vault", f.vault, "--candidate", f.candidate, "--evidence-root", f.evidence, "--evidence", "source.txt", "--source", "APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write", "--analysis-date", "2026-09-24", "--output", manifestPath)
	writeReview := map[string]any{"version": json.Number("2"), "manifest_digest": frozen["manifest_digest"], "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	saveJSON(t, reviewPath, writeReview)
	checked := call(t, "review", "check", "--vault", f.vault, "--candidate", f.candidate, "--evidence-root", f.evidence, "--manifest", manifestPath, "--review", reviewPath, "--checkout", "APP00001-example", f.repo)
	if checked["code"] != "note-candidate-reviewed" {
		t.Fatal(checked)
	}
	published := call(t, "review", "publish", "--state-root", f.state, "--vault", f.vault, "--candidate", f.candidate, "--evidence-root", f.evidence, "--manifest", manifestPath, "--review", reviewPath, "--checkout", "APP00001-example", f.repo)
	if published["code"] != "direct-publication-complete" {
		t.Fatal(published)
	}
}

func TestDirectPublishRejectsSourceAndDestinationDrift(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		f := newDirectFixture(t, "new note\n")
		write(t, filepath.Join(f.repo, "source.txt"), "later\n")
		sourceCommit(t, f.repo)
		if _, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0); e == nil || !strings.Contains(e.Error(), "source-ref-stale") {
			t.Fatal(e)
		}
	})
	t.Run("destination", func(t *testing.T) {
		f := newDirectFixture(t, "new note\n")
		write(t, filepath.Join(f.vault, f.note), "user edit\n")
		if _, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0); e == nil || !strings.Contains(e.Error(), "stale") {
			t.Fatal(e)
		}
	})
}

func TestDirectOutputScanReportsLocationsWithoutEcho(t *testing.T) {
	candidate := t.TempDir()
	write(t, filepath.Join(candidate, "20-Repos/a.md"), "API_KEY is configured by the runtime.\napi_key: ${API_KEY}\nsrc/home/components/view.ts\napi_key: actual-secret-value\nBearer abcdefghijklmnop\n")
	issues, e := directScanIssues(candidate, nil, nil)
	if e != nil || len(issues) != 2 {
		t.Fatal(issues, e)
	}
	encoded, _ := json.Marshal(issues)
	text := string(encoded)
	if strings.Contains(text, "actual-secret-value") || strings.Contains(text, "abcdefghijklmnop") {
		t.Fatal("scanner echoed credential material")
	}
	for _, value := range issues {
		issue := obj(value)
		if issue["path"] != "20-Repos/a.md" || issue["line"] == nil {
			t.Fatal(issue)
		}
	}
}

func TestDirectFreezeRejectsMalformedRepositoryCommitBeforeReview(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	os.Remove(filepath.Join(f.vault, f.note))
	os.Remove(filepath.Join(f.candidate, f.note))
	f.note = "20-Repos/APP00001-example/example.md"
	write(t, filepath.Join(f.vault, f.note), f.baseText)
	write(t, filepath.Join(f.candidate, f.note), "---\naliases: [APP00001-example]\ncommit-analizado: b2f60de493b\nrama-analizada: master\n---\nnew note\n")
	source := directSourceArg{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"}
	manifest, issues, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
	if e != nil || manifest != nil || len(issues) == 0 {
		t.Fatal(manifest, issues, e)
	}
	found := false
	for _, value := range issues {
		issue := obj(value)
		if issue["code"] == "repo-commit-invalid" && issue["path"] == f.note && issue["line"] == 3 {
			found = true
		}
	}
	if !found {
		t.Fatal(issues)
	}
}

func TestDirectNoChangeWritesInventoryCompatibleAcknowledgement(t *testing.T) {
	f := newDirectFixture(t, "old note\n")
	result, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0)
	if e != nil || result["status"] != "pass" {
		t.Fatal(result, e)
	}
	body, e := os.ReadFile(filepath.Join(f.vault, ackPath))
	if e != nil {
		t.Fatal(e)
	}
	records, e := acknowledgementDocument(body, true)
	if e != nil {
		t.Fatal(e)
	}
	record := obj(records["APP00001-example"])
	if record["decision"] != "no-documentation-change" || record["analyzed_sha"] != f.newOID[:12] {
		t.Fatal(record)
	}
	note, _ := os.ReadFile(filepath.Join(f.vault, f.note))
	if string(note) != f.baseText {
		t.Fatal("no-change publication changed note baseline")
	}
}

func TestDirectAcknowledgementMergeInitialAndRetirement(t *testing.T) {
	t.Run("unrelated-merge", func(t *testing.T) {
		f := newDirectFixture(t, "old note\n")
		unrelated := map[string]any{"version": json.Number("1"), "repositories": []any{map[string]any{"repository": "APP00002-other", "branch": "main", "analyzed_sha": "123456789abc", "decision": "no-documentation-change", "analysis_date": "2026-09-24"}}}
		body, _ := canonical(unrelated)
		write(t, filepath.Join(f.vault, ackPath), string(append(body, '\n')))
		if _, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0); e != nil {
			t.Fatal(e)
		}
		body, _ = os.ReadFile(filepath.Join(f.vault, ackPath))
		records, e := acknowledgementDocument(body, true)
		if e != nil || len(records) != 2 || records["APP00002-other"] == nil || records["APP00001-example"] == nil {
			t.Fatal(records, e)
		}
	})

	t.Run("independent-replay", func(t *testing.T) {
		f := newDirectFixture(t, "old note\n")
		repoB := sourceRepo(t)
		if _, e := sourceGit(repoB, nil, "remote", "add", "origin", "https://example.invalid/team/APP00002-other.git"); e != nil {
			t.Fatal(e)
		}
		write(t, filepath.Join(repoB, "source.txt"), "old\n")
		oldB := sourceCommit(t, repoB)
		write(t, filepath.Join(repoB, "source.txt"), "new\n")
		newB := sourceCommit(t, repoB)
		refB, _ := sourceGit(repoB, nil, "symbolic-ref", "HEAD")
		write(t, filepath.Join(f.vault, "20-Repos/other.md"), "---\naliases: [APP00002-other]\ncommit-analizado: "+oldB[:12]+"\nrama-analizada: master\n---\n")
		sourceB := directSourceArg{"APP00002-other", repoB, strings.TrimSpace(string(refB)), oldB, newB, "no-documentation-change"}
		manifestB, _, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{sourceB})
		if e != nil {
			t.Fatal(e)
		}
		dB, _ := digest(manifestB)
		reviewB := map[string]any{"version": json.Number("2"), "manifest_digest": dB, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
		if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0); e != nil {
			t.Fatal(e)
		}
		if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifestB, reviewB, map[string]string{"APP00002-other": repoB}, 0, 0); e != nil {
			t.Fatal(e)
		}
		if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0); e != nil {
			t.Fatal(e)
		}
	})

	t.Run("initial-no-node", func(t *testing.T) {
		f := newDirectFixture(t, "old note\n")
		if e := os.Remove(filepath.Join(f.candidate, f.note)); e != nil {
			t.Fatal(e)
		}
		if e := os.Remove(filepath.Join(f.vault, f.note)); e != nil {
			t.Fatal(e)
		}
		source := directSourceArg{"APP00001-example", f.repo, f.ref, "", f.newOID, "no-durable-node"}
		manifest, _, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
		if e != nil {
			t.Fatal(e)
		}
		d, _ := digest(manifest)
		reviewValue := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
		if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, f.checkouts(), 0, 0); e != nil {
			t.Fatal(e)
		}
		body, _ := os.ReadFile(filepath.Join(f.vault, ackPath))
		records, e := acknowledgementDocument(body, true)
		if e != nil || obj(records["APP00001-example"])["decision"] != "no-durable-node" {
			t.Fatal(records, e)
		}
	})

	t.Run("matching-retirement", func(t *testing.T) {
		f := newDirectFixture(t, "new note\n")
		ack := map[string]any{"version": json.Number("1"), "repositories": []any{map[string]any{"repository": "APP00001-example", "branch": strings.TrimPrefix(f.ref, "refs/heads/"), "analyzed_sha": f.newOID[:12], "decision": "no-documentation-change", "analysis_date": "2026-09-24"}}}
		body, _ := canonical(ack)
		write(t, filepath.Join(f.vault, ackPath), string(append(body, '\n')))
		source := directSourceArg{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"}
		manifest, _, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
		if e != nil {
			t.Fatal(e)
		}
		d, _ := digest(manifest)
		reviewValue := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
		if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, f.checkouts(), 0, 0); e != nil {
			t.Fatal(e)
		}
		if _, e = os.Stat(filepath.Join(f.vault, ackPath)); !os.IsNotExist(e) {
			t.Fatal("matching acknowledgement was not retired")
		}
	})
}

func TestDirectMixedSourceOutcomesAdvanceOnlyTheirOwnCursor(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	repoB := sourceRepo(t)
	if _, e := sourceGit(repoB, nil, "remote", "add", "origin", "https://example.invalid/team/APP00002-other.git"); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(repoB, "source.txt"), "old\n")
	oldB := sourceCommit(t, repoB)
	write(t, filepath.Join(repoB, "source.txt"), "new\n")
	newB := sourceCommit(t, repoB)
	refB, _ := sourceGit(repoB, nil, "symbolic-ref", "HEAD")
	write(t, filepath.Join(f.vault, "20-Repos/other.md"), "---\naliases: [APP00002-other]\ncommit-analizado: "+oldB[:12]+"\nrama-analizada: master\n---\n")
	repoC := sourceRepo(t)
	if _, e := sourceGit(repoC, nil, "remote", "add", "origin", "https://example.invalid/team/APP00003-empty.git"); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(repoC, "source.txt"), "new\n")
	newC := sourceCommit(t, repoC)
	refC, _ := sourceGit(repoC, nil, "symbolic-ref", "HEAD")
	sources := []directSourceArg{
		{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"},
		{"APP00002-other", repoB, strings.TrimSpace(string(refB)), oldB, newB, "no-documentation-change"},
		{"APP00003-empty", repoC, strings.TrimSpace(string(refC)), "", newC, "no-durable-node"},
	}
	manifest, issues, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", sources)
	if e != nil || len(issues) != 0 {
		t.Fatal(e, issues)
	}
	d, _ := digest(manifest)
	reviewValue := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	checkouts := map[string]string{"APP00001-example": f.repo, "APP00002-other": repoB, "APP00003-empty": repoC}
	if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, checkouts, 0, 0); e != nil {
		t.Fatal(e)
	}
	result, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, checkouts, 0, 0)
	if e != nil || result["reused"] != true {
		t.Fatal(result, e)
	}
	body, _ := os.ReadFile(filepath.Join(f.vault, ackPath))
	records, e := acknowledgementDocument(body, true)
	if e != nil || records["APP00001-example"] != nil || obj(records["APP00002-other"])["decision"] != "no-documentation-change" || obj(records["APP00003-empty"])["decision"] != "no-durable-node" {
		t.Fatal(records, e)
	}
}

func TestDirectMixedSourceOutcomesRejectPerSourceCandidateMismatch(t *testing.T) {
	tests := []struct {
		name, outcome            string
		base, candidate, deleted bool
	}{
		{"no-change-modifies-note", "no-documentation-change", true, true, false},
		{"no-change-deletes-note", "no-documentation-change", true, false, true},
		{"no-node-creates-note", "no-durable-node", false, true, false},
		{"no-node-deletes-note", "no-durable-node", true, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newDirectFixture(t, "new note\n")
			repoB := sourceRepo(t)
			if _, e := sourceGit(repoB, nil, "remote", "add", "origin", "https://example.invalid/team/APP00002-other.git"); e != nil {
				t.Fatal(e)
			}
			write(t, filepath.Join(repoB, "source.txt"), "old\n")
			oldB := sourceCommit(t, repoB)
			write(t, filepath.Join(repoB, "source.txt"), "new\n")
			newB := sourceCommit(t, repoB)
			refB, _ := sourceGit(repoB, nil, "symbolic-ref", "HEAD")
			if test.base {
				write(t, filepath.Join(f.vault, "20-Repos/other.md"), "---\naliases: [APP00002-other]\n---\nold\n")
			}
			if test.candidate {
				write(t, filepath.Join(f.candidate, "20-Repos/other.md"), "---\naliases: [APP00002-other]\n---\nnew\n")
			}
			sources := []directSourceArg{
				{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"},
				{"APP00002-other", repoB, strings.TrimSpace(string(refB)), oldB, newB, test.outcome},
			}
			deleted := []string(nil)
			if test.deleted {
				deleted = []string{"20-Repos/other.md"}
			}
			manifest, issues, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, deleted, "", "2026-09-24", sources)
			if e == nil || !strings.Contains(e.Error(), "source-outcome-note-mismatch") || manifest != nil || len(issues) != 0 {
				t.Fatal(manifest, issues, e)
			}
		})
	}
}

func TestDirectNoDurableNodeAllowsVaultWithoutRepositoryDirectory(t *testing.T) {
	manifest := map[string]any{
		"candidate_files": map[string]any{},
		"base_files":      map[string]any{},
		"base_present":    []any{},
		"deleted_files":   []any{},
		"source_bindings": []any{map[string]any{"repository": "APP00001-example", "outcome": "no-durable-node"}},
	}
	if e := validateSourceOutcomes(t.TempDir(), manifest); e != nil {
		t.Fatal(e)
	}
}

func TestDirectCrashRecoveryAndMixedImageSafety(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	if _, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 1); e == nil || !strings.Contains(e.Error(), "simulated-crash") {
		t.Fatal(e)
	}
	result, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", f.manifest, f.review, f.checkouts(), 0, 0)
	if e != nil || result["reused"] != true {
		t.Fatal(result, e)
	}

	f = newDirectFixture(t, "new note\n")
	second := "20-Repos/second.md"
	write(t, filepath.Join(f.vault, second), "second old\n")
	write(t, filepath.Join(f.candidate, second), "second new\n")
	source := directSourceArg{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"}
	manifest, issues, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
	if e != nil || len(issues) != 0 {
		t.Fatal(e, issues)
	}
	d, _ := digest(manifest)
	review := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, review, f.checkouts(), 0, 1); e == nil || !strings.Contains(e.Error(), "simulated-crash") {
		t.Fatal(e)
	}
	write(t, filepath.Join(f.vault, second), "unrelated edit\n")
	if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, review, f.checkouts(), 0, 0); e == nil || !strings.Contains(e.Error(), "destination-stale") {
		t.Fatal(e)
	}
	body, _ := os.ReadFile(filepath.Join(f.vault, second))
	if string(body) != "unrelated edit\n" {
		t.Fatal("recovery overwrote unrelated edit")
	}

	f = newDirectFixture(t, "new note\n")
	write(t, filepath.Join(f.vault, second), "second old\n")
	write(t, filepath.Join(f.candidate, second), "second new\n")
	source = directSourceArg{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"}
	manifest, _, _ = freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
	d, _ = digest(manifest)
	review = map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	_, _ = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, review, f.checkouts(), 0, 1)
	if _, e = directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, review, f.checkouts(), 0, 0); e != nil {
		t.Fatal(e)
	}
}

func TestDirectReviewOnlyExplainsRetiredConnections(t *testing.T) {
	f := newDirectFixture(t, "new note\n")
	base := "old note\n<!-- connection:connection.old -->\n"
	write(t, filepath.Join(f.vault, f.note), base)
	source := directSourceArg{"APP00001-example", f.repo, f.ref, f.oldOID, f.newOID, "write"}
	manifest, issues, e := freezeBound(f.vault, f.candidate, f.evidence, []string{"source.txt"}, nil, "", "2026-09-24", []directSourceArg{source})
	if e != nil || len(issues) != 0 {
		t.Fatal(e, issues)
	}
	d, _ := digest(manifest)
	reviewValue := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{f.note + "#connection.old": "source no longer exposes this connection"}}
	if e = review(reviewValue, manifest); e != nil {
		t.Fatal(e)
	}
	delete(reviewValue["retired_connections"].(map[string]any), f.note+"#connection.old")
	if e = review(reviewValue, manifest); e == nil {
		t.Fatal("retirement was not explicit")
	}
}

func TestDirectPublishCannotBypassSourceOutcomeChecks(t *testing.T) {
	f := newDirectFixture(t, "old note\n")
	manifest := clone(f.manifest)
	sources := append([]any{}, arr(f.manifest["source_bindings"])...)
	binding := clone(obj(sources[0]))
	binding["outcome"] = "no-durable-node"
	sources[0] = binding
	manifest["source_bindings"] = sources
	acks := map[string]any{}
	for repository, value := range obj(f.manifest["acknowledgements"]) {
		acks[repository] = clone(obj(value))
	}
	result := clone(obj(obj(acks["APP00001-example"])["result"]))
	result["decision"] = "no-durable-node"
	obj(acks["APP00001-example"])["result"] = result
	manifest["acknowledgements"] = acks
	d, _ := digest(manifest)
	reviewValue := map[string]any{"version": json.Number("2"), "manifest_digest": d, "verdict": "accept", "findings": []any{}, "retired_connections": map[string]any{}}
	if _, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, f.checkouts(), 0, 0); e == nil || !strings.Contains(e.Error(), "source-outcome-note-mismatch") {
		t.Fatal(e)
	}

	binding["outcome"] = "write"
	obj(acks["APP00001-example"])["result"] = nil
	if e := validateSourceOutcomes(f.vault, manifest); e == nil {
		t.Fatalf("outcome bypass setup invalid: changed=%v source=%v", directDocsChanged(manifest), obj(arr(manifest["source_bindings"])[0]))
	}
	d, _ = digest(manifest)
	reviewValue["manifest_digest"] = d
	published, e := directPublish(f.state, f.vault, f.candidate, f.evidence, "", manifest, reviewValue, f.checkouts(), 0, 0)
	if e == nil && obj(published)["status"] != "blocked" {
		t.Fatal(published, e)
	}
}
